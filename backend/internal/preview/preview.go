// Package preview resolves the preview URL a round's clip plays. The source is Apple Music (apple_id + storefront,
// else the ISRC); URLs are cached about a day, so a party does not ask Apple once per round.
package preview

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/trick77/reworm/internal/deck"
)

// Source resolves a song's preview URL.
type Source interface {
	PreviewURL(ctx context.Context, s deck.Song) (string, error)
}

// ErrNone is a song the source has no preview for: the round skips it.
var ErrNone = errors.New("no preview for this song")

// AudioHost is where Apple's previews stream from, straight to the browser: the server's CSP media-src allows it.
const AudioHost = "audio-ssl.itunes.apple.com"

// TTL is how long a resolved URL is reused.
const TTL = 24 * time.Hour

// maxEntries bounds the cache; past it expired entries go, then everything (a new day's lookups refill it).
const maxEntries = 20_000

// Cache is a Source that remembers what src resolved for TTL. Failures are not cached. Safe for concurrent use.
type Cache struct {
	src Source
	ttl time.Duration
	now func() time.Time

	mu sync.Mutex
	m  map[string]entry
}

type entry struct {
	url string
	exp time.Time
}

// NewCache wraps src; now nil is time.Now.
func NewCache(src Source, ttl time.Duration, now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{src: src, ttl: ttl, now: now, m: map[string]entry{}}
}

// PreviewURL is the cached URL, else src's, cached on success.
func (c *Cache) PreviewURL(ctx context.Context, s deck.Song) (string, error) {
	c.mu.Lock()
	e, ok := c.m[s.ID]
	c.mu.Unlock()
	if ok && c.now().Before(e.exp) {
		return e.url, nil
	}
	u, err := c.src.PreviewURL(ctx, s)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= maxEntries {
		now := c.now()
		for k, v := range c.m {
			if !now.Before(v.exp) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= maxEntries {
			clear(c.m)
		}
	}
	c.m[s.ID] = entry{url: u, exp: c.now().Add(c.ttl)}
	return u, nil
}

// Fake is a Source for tests and local runs: a URL per song id, ErrNone for the ids in Missing. It counts calls.
type Fake struct {
	Missing map[string]bool

	mu    sync.Mutex
	calls int
}

// PreviewURL is https://previews.invalid/<id>.m4a.
func (f *Fake) PreviewURL(_ context.Context, s deck.Song) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.Missing[s.ID] {
		return "", ErrNone
	}
	return "https://previews.invalid/" + s.ID + ".m4a", nil
}

// Calls is how often PreviewURL ran.
func (f *Fake) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}
