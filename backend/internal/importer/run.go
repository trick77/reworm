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
	"github.com/trick77/ship/kit/version"

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/cards"
	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

const usage = `usage: import [-env file] [-data dir] [-workers n] cards|songs

  cards  download the card lists (BACKEND_CARDS_BASE_URL, BACKEND_CARDS_FILES) into <data>/cards/<set>.csv
  songs  resolve the snapshots of the sets BACKEND_CARDS_FILES names against MusicBrainz, Apple Music and the
         Cover Art Archive into <data>/songs.jsonl and <data>/import-report.md; every answer is cached in <data>/cache`

// Endpoints are the services' base URLs; tests point them at fixtures.
type Endpoints struct {
	MusicBrainz, AppleAPI, ApplePlayer, CoverArt string
	// MBEvery spaces MusicBrainz requests (1 s; tests 0); AppleEvery Apple's (cache misses only, as MusicBrainz).
	MBEvery, AppleEvery time.Duration
}

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
		err = runSongs(ctx, lookup, *dataDir, *workers, ep, log)
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

func runSongs(ctx context.Context, lookup func(string) string, dataDir string, workers int, ep Endpoints, log *slog.Logger) error {
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
	p := newPipeline(cacheDir, workers, ep, log)

	entries, stats := collect(sets)
	log.Info("resolving songs", "sets", len(sets), "songs", len(entries))
	rs, err := p.resolveAll(ctx, entries)
	if err != nil {
		return err
	}
	rs, merged := merge(rs)
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

// newPipeline wires the clients: every answer through the disk cache; MusicBrainz throttled below it, so only
// misses wait; the Apple token kept in the cache dir (its page and bundle are not cached: a cached bundle would
// hand out an expired token forever).
func newPipeline(cacheDir string, workers int, ep Endpoints, log *slog.Logger) *Pipeline {
	// One cache dir for all; clients evict what they cannot decode (any Transport on the dir does).
	cache := &httpcache.Transport{Dir: filepath.Join(cacheDir, "http")}
	cached := func(next http.RoundTripper) *http.Client {
		return &http.Client{Timeout: time.Minute, Transport: &httpcache.Transport{Dir: cache.Dir, Next: next}}
	}
	tokenPath := filepath.Join(cacheDir, "apple-token.json")
	tokens := &apple.Source{BaseURL: ep.ApplePlayer, HTTP: &http.Client{Timeout: time.Minute},
		OnNew: func(t apple.Token) {
			if err := apple.SaveToken(tokenPath, t); err != nil {
				log.Warn("apple token not kept", logging.Err(err))
			}
		}}
	if t, ok := apple.LoadToken(tokenPath, time.Now()); ok {
		tokens.Seed(t)
	}
	caa := cached(nil)
	caa.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Pipeline{
		MB: &musicbrainz.Client{BaseURL: ep.MusicBrainz,
			HTTP:      cached(&httpcache.Throttle{Every: ep.MBEvery}),
			UserAgent: "Reworm/" + version.String() + " ( https://github.com/trick77/reworm )",
			Evict:     cache.Evict},
		Apple: &apple.Client{BaseURL: ep.AppleAPI, HTTP: cached(&httpcache.Throttle{Every: ep.AppleEvery}), Dev: tokens,
			Evict: cache.Evict},
		Covers:  &CoverArt{BaseURL: ep.CoverArt, HTTP: caa},
		Workers: workers,
		Log:     log,
	}
}
