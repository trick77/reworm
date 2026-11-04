// Package covers serves song covers from our own disk (plan D4): the stored cover_url (Apple artwork or the Cover
// Art Archive) is fetched on first use and kept under the data dir, so a game never loads an outside image. A cover
// is named by deck.CoverVersion, the hash of the URL it is fetched from: its file, its link's ?v= and its ETag.
package covers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/trick77/ship/kit/fsx"
	"github.com/trick77/ship/kit/logging"
	"golang.org/x/sync/singleflight"

	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
)

// maxBytes bounds a fetched image.
const maxBytes = 5 << 20

// cacheControl lets the browser keep a cover for good under the link naming it: another cover is another link.
const cacheControl = "public, max-age=31536000, immutable"

// How long a failed fetch is not tried again: an answer from the image host (404, not an image, too big) for an
// hour; a transport failure (host unreachable) a few minutes. Every reveal of the song would refetch otherwise.
const (
	badCoverTTL  = time.Hour
	downCoverTTL = 5 * time.Minute
	maxFailures  = 10_000
)

// Lookup is the stored cover URL of a song, "" when it has none, deck.ErrNotFound for an unknown id.
type Lookup func(ctx context.Context, songID string) (string, error)

// Handler serves GET /api/covers/{songId}.
type Handler struct {
	lookup Lookup
	dir    string
	client *http.Client
	now    func() time.Time

	// fetches lets one upstream fetch per cover run (keyed by its version: songs of one album share it);
	// concurrent requests for it share the result.
	fetches singleflight.Group
	mu      sync.Mutex
	failed  map[string]time.Time // cover version → no fetch before then
}

// New serves covers cached in dir, fetching with client (nil: a client with a 15 s timeout, following redirects).
func New(lookup Lookup, dir string, client *http.Client) *Handler {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Handler{lookup: lookup, dir: dir, client: client, now: time.Now, failed: map[string]time.Time{}}
}

// Mount adds the route.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/covers/{songId}", h.serve)
}

// Sweep removes every file in the cache that is no cover of stored (the songs' cover URLs): a cover a re-import
// replaced, and the files from before covers were named by their version. Run at boot, before any request. It
// returns how many files it removed.
func (h *Handler) Sweep(stored []string) (int, error) {
	keep := make(map[string]bool, len(stored))
	for _, s := range stored {
		keep[deck.CoverVersion(s)] = true
	}
	entries, err := os.ReadDir(h.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || keep[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(h.dir, e.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("songId")
	stored, err := h.lookup(r.Context(), id)
	if err == nil && stored == "" {
		err = deck.ErrNotFound
	}
	if err != nil {
		h.fault(w, r, id, err)
		return
	}
	version := deck.CoverVersion(stored)
	etag := `"` + version + `"`
	w.Header().Set("ETag", etag)
	// Kept for good only under the link that names this cover: an old or bare link is asked again (a 304 when the
	// browser holds this cover), so it never pins a cover a re-import replaced.
	if r.URL.Query().Get("v") == version {
		w.Header().Set("Cache-Control", cacheControl)
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	b, err := h.cover(r.Context(), id, stored)
	if err != nil {
		w.Header().Del("ETag")
		w.Header().Del("Cache-Control")
		h.fault(w, r, id, err)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	_, _ = w.Write(b) //nolint:gosec // an image (fetch checked image/*), sent as its sniffed type with nosniff
}

// fault answers a cover that cannot be served: 404 (the UI draws the record), logged unless it is plainly none.
func (h *Handler) fault(w http.ResponseWriter, r *http.Request, id string, err error) {
	if errors.Is(err, deck.ErrNotFound) {
		err = api.ErrNotFound
	}
	var pr *api.Problem
	if !errors.As(err, &pr) {
		slog.WarnContext(r.Context(), "cover unavailable", "song", id, logging.Err(err))
		err = api.ErrNotFound
	}
	api.WriteError(w, r, err)
}

// cover is the cover stored names: from disk, else from upstream.
func (h *Handler) cover(ctx context.Context, id, stored string) ([]byte, error) {
	version := deck.CoverVersion(stored)
	p := filepath.Join(h.dir, version)
	b, err := os.ReadFile(p) //nolint:gosec // p is a hash under our own cache dir
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		slog.WarnContext(ctx, "cover cache unreadable", "song", id, logging.Err(err))
	}
	return h.get(ctx, id, deck.CoverSource(stored), version, p)
}

// get is the cover from upstream: one fetch per cover at a time, none while a recent failure stands.
func (h *Handler) get(ctx context.Context, id, source, version, p string) ([]byte, error) {
	h.mu.Lock()
	until, failed := h.failed[version]
	h.mu.Unlock()
	if failed && h.now().Before(until) {
		return nil, api.ErrNotFound
	}
	// The shared fetch must not end with the one client that happened to start it.
	v, err, _ := h.fetches.Do(version, func() (any, error) {
		return h.fetch(context.WithoutCancel(ctx), id, source, version, p)
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// fail remembers a failed fetch of a cover for ttl.
func (h *Handler) fail(version string, ttl time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.failed) >= maxFailures {
		clear(h.failed)
	}
	h.failed[version] = h.now().Add(ttl)
}

// fetch downloads a cover from source and caches it at p (song id only names it in the log).
func (h *Handler) fetch(ctx context.Context, id, source, version, p string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		h.fail(version, badCoverTTL)
		return nil, err
	}
	resp, err := h.client.Do(req) //nolint:gosec // the URL comes from our own dataset
	if err != nil {
		h.fail(version, downCoverTTL)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		h.fail(version, badCoverTTL)
		return nil, fmt.Errorf("cover fetch: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		h.fail(version, downCoverTTL)
		return nil, err
	}
	if len(b) > maxBytes {
		h.fail(version, badCoverTTL)
		return nil, fmt.Errorf("cover over %d bytes", maxBytes)
	}
	if ct := http.DetectContentType(b); !strings.HasPrefix(ct, "image/") {
		h.fail(version, badCoverTTL)
		return nil, fmt.Errorf("cover is %s, not an image", ct)
	}
	// A cache that cannot be written still serves this request; the next one fetches again.
	err = os.MkdirAll(h.dir, 0o750)
	if err == nil {
		err = fsx.WriteFileAtomic(p, b, 0o640)
	}
	if err != nil {
		slog.WarnContext(ctx, "cover not cached", "song", id, logging.Err(err))
	}
	return b, nil
}
