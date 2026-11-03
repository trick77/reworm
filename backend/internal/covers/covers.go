// Package covers serves song covers from our own disk (plan D4): the stored cover_url (Apple artwork or the Cover
// Art Archive) is fetched on first use and kept under the data dir, so a game never loads an outside image.
package covers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// size is the edge Apple artwork is fetched at: crisp on a TV stage, ~50 KB.
const size = "600x600"

// maxBytes bounds a fetched image.
const maxBytes = 5 << 20

// cacheControl lets the browser keep a cover for good: a song's original cover does not change.
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

	// fetches lets one upstream fetch per song run; concurrent requests for it share the result.
	fetches singleflight.Group
	mu      sync.Mutex
	failed  map[string]time.Time // song id → no fetch before then
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

// SourceURL is the URL a stored cover URL is fetched from: Apple's {w}x{h} template at 600x600, others as they are.
func SourceURL(stored string) string {
	return strings.ReplaceAll(strings.ReplaceAll(stored, "{w}x{h}", size), "{f}", "jpg")
}

// path is where a song's cover lives: a hash of the id, never the id itself, in a path.
func (h *Handler) path(songID string) string {
	sum := sha256.Sum256([]byte(songID))
	return filepath.Join(h.dir, hex.EncodeToString(sum[:16]))
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("songId")
	p := h.path(id)
	b, err := os.ReadFile(p) //nolint:gosec // p is a hash under our own cache dir
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.WarnContext(r.Context(), "cover cache unreadable", "song", id, logging.Err(err))
		}
		b, err = h.get(r.Context(), id, p)
	}
	if err != nil {
		var pr *api.Problem
		if !errors.As(err, &pr) {
			slog.WarnContext(r.Context(), "cover unavailable", "song", id, logging.Err(err))
			err = api.ErrNotFound
		}
		api.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", cacheControl)
	_, _ = w.Write(b)
}

// get is the cover from upstream: one fetch per song at a time, none while a recent failure stands.
func (h *Handler) get(ctx context.Context, id, p string) ([]byte, error) {
	h.mu.Lock()
	until, failed := h.failed[id]
	h.mu.Unlock()
	if failed && h.now().Before(until) {
		return nil, api.ErrNotFound
	}
	// The shared fetch must not end with the one client that happened to start it.
	v, err, _ := h.fetches.Do(id, func() (any, error) {
		return h.fetch(context.WithoutCancel(ctx), id, p)
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// fail remembers a failed fetch for ttl.
func (h *Handler) fail(id string, ttl time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.failed) >= maxFailures {
		clear(h.failed)
	}
	h.failed[id] = h.now().Add(ttl)
}

// fetch downloads a song's cover and caches it at p.
func (h *Handler) fetch(ctx context.Context, id, p string) ([]byte, error) {
	stored, err := h.lookup(ctx, id)
	if errors.Is(err, deck.ErrNotFound) || (err == nil && stored == "") {
		return nil, api.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SourceURL(stored), nil)
	if err != nil {
		h.fail(id, badCoverTTL)
		return nil, err
	}
	resp, err := h.client.Do(req) //nolint:gosec // the URL comes from our own dataset
	if err != nil {
		h.fail(id, downCoverTTL)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		h.fail(id, badCoverTTL)
		return nil, fmt.Errorf("cover fetch: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		h.fail(id, downCoverTTL)
		return nil, err
	}
	if len(b) > maxBytes {
		h.fail(id, badCoverTTL)
		return nil, fmt.Errorf("cover over %d bytes", maxBytes)
	}
	if ct := http.DetectContentType(b); !strings.HasPrefix(ct, "image/") {
		h.fail(id, badCoverTTL)
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
