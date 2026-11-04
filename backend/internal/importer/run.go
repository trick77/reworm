// Package importer builds the song dataset offline: card snapshots in, data/songs.jsonl, data/overrides.json and
// data/import-report.md out. Run by hand on a dev machine; the output is committed and embedded in the server.
package importer

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/trick77/ship/kit/env"
	"github.com/trick77/ship/kit/fsx"
	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/ratelimit"
	"github.com/trick77/ship/kit/version"

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/cards"
	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

const usage = `usage: import [-env file] [-data dir] [-workers n] [-offline] cards|songs

  cards  download the card lists (BACKEND_CARDS_BASE_URL, BACKEND_CARDS_FILES) into <data>/cards/<set>.csv
  songs  resolve the snapshots of the sets BACKEND_CARDS_FILES names against MusicBrainz, Apple Music and the
         Cover Art Archive into <data>/songs.jsonl and <data>/import-report.md; every answer is cached in <data>/cache.
         -offline answers from the cache only: a song that would need the network keeps its previous entry`

// Endpoints are the services' base URLs; tests point them at fixtures.
type Endpoints struct {
	MusicBrainz, AppleAPI, ApplePlayer, CoverArt string
	// MBEvery spaces MusicBrainz requests (1 s); AppleEvery Apple's (cache misses only, as MusicBrainz). Above 0:
	// kit's throttle refuses a zero gap (tests take 1 ns).
	MBEvery, AppleEvery time.Duration
}

// maxQueue is how long a request may wait for its paced turn: never refused, the workers just queue.
const maxQueue = 24 * time.Hour

// DefaultEndpoints are the real services.
var DefaultEndpoints = Endpoints{
	MusicBrainz: musicbrainz.BaseURL, AppleAPI: apple.APIURL, ApplePlayer: apple.PlayerURL, CoverArt: CAAURL,
	// Apple: the web player's API is unofficial with unpublished limits; 5 req/s caused a ~3.5 h block (2026-11-04),
	// so 20 req/min, the one rate Apple publishes (its iTunes Search API).
	MBEvery: time.Second, AppleEvery: 3 * time.Second,
}

// Main runs the command: args without the program name, getenv the process environment. It returns the exit code.
func Main(ctx context.Context, args []string, getenv func(string) string, stderr io.Writer, ep Endpoints) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { _, _ = fmt.Fprintln(stderr, usage) }
	envFile := fs.String("env", "", "dotenv file (KEY=VALUE lines); the process environment wins")
	dataDir := fs.String("data", "data", "the repo's data directory")
	workers := fs.Int("workers", 4, "songs resolved at once (MusicBrainz stays at 1 req/s)")
	offline := fs.Bool("offline", false, "songs: answer from the cache only, no network")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || (fs.Arg(0) != "cards" && fs.Arg(0) != "songs") {
		fs.Usage()
		return 2
	}
	lookup := getenv
	if *envFile != "" {
		file, err := readEnvFile(*envFile)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		lookup = lookupWith(file, getenv)
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	var err error
	if fs.Arg(0) == "cards" {
		err = runCards(ctx, lookup, *dataDir, log)
	} else {
		err = runSongs(ctx, lookup, *dataDir, *workers, *offline, ep, log)
	}
	if err != nil {
		log.Error("import failed", logging.Err(err))
		return 1
	}
	return 0
}

func runCards(ctx context.Context, lookup func(string) string, dataDir string, log *slog.Logger) error {
	r := env.New(lookup)
	base := r.Required("BACKEND_CARDS_BASE_URL")
	if err := r.Err(); err != nil {
		return err
	}
	srcs, err := configuredSources(lookup)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: time.Minute}
	for _, src := range srcs {
		cs, err := cards.Fetch(ctx, client, base, src)
		if err != nil {
			return err
		}
		if err := cards.WriteSnapshot(filepath.Join(dataDir, "cards"), src.Slug(), cs); err != nil {
			return err
		}
		log.Info("card set written", "set", src.Slug(), "cards", len(cs))
	}
	return nil
}

// configuredSources are the card sets BACKEND_CARDS_FILES names: the songs step reads exactly these snapshots.
func configuredSources(lookup func(string) string) ([]cards.Source, error) {
	r := env.New(lookup)
	spec := r.Required("BACKEND_CARDS_FILES")
	if err := r.Err(); err != nil {
		return nil, err
	}
	return cards.ParseSources(spec)
}

func runSongs(ctx context.Context, lookup func(string) string, dataDir string, workers int, offline bool, ep Endpoints,
	log *slog.Logger) error {
	srcs, err := configuredSources(lookup)
	if err != nil {
		return err
	}
	sets, strays, err := cards.ReadSnapshots(filepath.Join(dataDir, "cards"), srcs)
	if err != nil {
		return err
	}
	for _, f := range strays {
		log.Warn("card snapshot of a set no longer configured, left out (delete it)", "file", f)
	}
	ovPath := filepath.Join(dataDir, "overrides.json")
	ov, err := loadOverrides(ovPath)
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(dataDir, "cache")
	p := newPipeline(cacheDir, workers, offline, ep, log)
	if offline {
		if p.Previous, err = previousSongs(filepath.Join(dataDir, "songs.jsonl")); err != nil {
			return err
		}
	}

	entries, stats := collect(sets)
	log.Info("resolving songs", "sets", len(sets), "songs", len(entries), "offline", offline)
	rs, err := p.resolveAll(ctx, entries)
	if err != nil {
		return err
	}
	rs, merged, err := p.merge(ctx, rs)
	if err != nil {
		return err
	}
	kept, applied, unknown, err := p.applyOverrides(ctx, rs, ov)
	if err != nil {
		return err
	}
	songs := make([]song.Song, len(kept))
	for i, r := range kept {
		songs[i] = r.Song
	}
	jsonl, err := song.Encode(songs)
	if err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(filepath.Join(dataDir, "songs.jsonl"), jsonl, 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(ovPath); errors.Is(err, os.ErrNotExist) {
		tmpl, err := templateOverrides()
		if err != nil {
			return err
		}
		if err := fsx.WriteFileAtomic(ovPath, tmpl, 0o644); err != nil {
			return err
		}
	}
	// The report shows the dataset as written: every section from the songs kept, overrides applied.
	rep := report{sets: stats, resolved: kept, merged: merged, applied: applied, unknown: unknown}
	if err := fsx.WriteFileAtomic(filepath.Join(dataDir, "import-report.md"), rep.markdown(), 0o644); err != nil {
		return err
	}
	log.Info("songs written", "songs", len(songs))
	return nil
}

// previousSongs reads the dataset an offline run falls back to, keyed by card key (song.Key of artist and title);
// none when there is no file yet.
func previousSongs(path string) (map[string]song.Song, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the repo's own dataset
	if errors.Is(err, os.ErrNotExist) {
		return map[string]song.Song{}, nil
	}
	if err != nil {
		return nil, err
	}
	songs, err := song.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("previous dataset: %w", err)
	}
	out := make(map[string]song.Song, len(songs))
	for _, s := range songs {
		out[song.Key(s.Artist, s.Title)] = s
	}
	return out, nil
}

// newPipeline wires the clients: every answer through the disk cache; MusicBrainz and Apple paced below it (kit's
// throttle, one turn at a time for all workers), so only misses wait; the Apple token kept in the cache dir (its
// page and bundle are not cached: a cached bundle would hand out an expired token forever). Offline, every miss is
// httpcache.ErrOffline instead of a request.
func newPipeline(cacheDir string, workers int, offline bool, ep Endpoints, log *slog.Logger) *Pipeline {
	// One cache dir for all; clients evict what they cannot decode (any Transport on the dir does).
	cache := &httpcache.Transport{Dir: filepath.Join(cacheDir, "http")}
	var net http.RoundTripper // nil: the default transport
	if offline {
		net = httpcache.Offline{}
	}
	cached := func(gap time.Duration) *http.Client {
		next := net
		if gap > 0 && !offline {
			next = &httpcache.Paced{Throttle: ratelimit.NewThrottle(gap, maxQueue)}
		}
		return &http.Client{Timeout: time.Minute, Transport: &httpcache.Transport{Dir: cache.Dir, Next: next}}
	}
	tokenPath := filepath.Join(cacheDir, "apple-token.json")
	tokens := &apple.Source{BaseURL: ep.ApplePlayer, HTTP: &http.Client{Timeout: time.Minute, Transport: net},
		OnNew: func(t apple.Token) {
			if err := apple.SaveToken(tokenPath, t); err != nil {
				log.Warn("apple token not kept", logging.Err(err))
			}
		}}
	if t, ok := apple.LoadToken(tokenPath, time.Now()); ok {
		tokens.Seed(t)
	}
	caa := cached(0)
	caa.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Pipeline{
		MB: &musicbrainz.Client{BaseURL: ep.MusicBrainz,
			HTTP:      cached(ep.MBEvery),
			UserAgent: "Reworm/" + version.String() + " ( https://github.com/trick77/reworm )",
			Evict:     cache.Evict},
		Apple: &apple.Client{BaseURL: ep.AppleAPI, HTTP: cached(ep.AppleEvery), Dev: tokens,
			Evict: cache.Evict},
		Covers:  &CoverArt{BaseURL: ep.CoverArt, HTTP: caa},
		Workers: workers,
		Log:     log,
	}
}
