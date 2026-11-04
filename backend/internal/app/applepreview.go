package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/ratelimit"

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
)

// Catalog is what the preview source asks Apple; *apple.Client implements it.
type Catalog interface {
	SongByID(ctx context.Context, storefront, id string) (apple.Song, bool, error)
	SongsByISRC(ctx context.Context, storefront string, isrcs []string) ([]apple.Song, error)
}

// applePreviews resolves a song's preview URL from Apple Music: the stored track (apple_id in its storefront), else
// any track with the song's ISRC (ch, de, us), else preview.ErrNone. Preview URLs are not stored by the import:
// they change; preview.Cache keeps each one a day.
type applePreviews struct{ catalog Catalog }

var isrcStorefronts = []string{"ch", "de", "us"}

func (a applePreviews) PreviewURL(ctx context.Context, s deck.Song) (string, error) {
	if s.AppleID != "" {
		sf := s.Storefront
		if sf == "" {
			sf = isrcStorefronts[0]
		}
		t, found, err := a.catalog.SongByID(ctx, sf, s.AppleID)
		if err != nil {
			return "", err
		}
		if found && t.PreviewURL != "" {
			return t.PreviewURL, nil
		}
	}
	// The track left the catalog (or never had one): any other with the same recording.
	if s.ISRC != "" {
		for _, sf := range isrcStorefronts {
			songs, err := a.catalog.SongsByISRC(ctx, sf, []string{s.ISRC})
			if err != nil {
				return "", err
			}
			for _, t := range songs {
				if t.PreviewURL != "" {
					return t.PreviewURL, nil
				}
			}
		}
	}
	return "", preview.ErrNone
}

const (
	// previewGap spaces the server's Apple lookups: a training session resolves dozens of songs at once, and the
	// web player's API is unofficial with unpublished limits (the import met a 3.5 h block at 5 req/s).
	previewGap = 300 * time.Millisecond
	// previewMaxWait is the longest a lookup queues for its turn before it is refused: a player is waiting.
	previewMaxWait = 2 * time.Second
	// lookupDeadline bounds one song's lookup, by id and ISRC in all storefronts together.
	lookupDeadline = 8 * time.Second
	// requestTimeout bounds one request to Apple; the server tries each once.
	requestTimeout = 5 * time.Second
	// noneTTL is how long a song without a preview is not asked about again.
	noneTTL = 6 * time.Hour
	// openFor is how long a 429 stops all lookups when Apple sent no Retry-After.
	openFor = 15 * time.Minute
	// maxNone bounds the remembered songs without a preview.
	maxNone = 20_000
)

// errPreviewsUnavailable is a lookup the guard did not let through: Apple refused recently, or the queue is full.
// The rounds answer it with previews_unavailable.
var errPreviewsUnavailable = errors.New("apple music previews unavailable for now")

// guardedPreviews keeps the server's lookups polite: one turn per previewGap for all requests (kit's throttle), a
// song without a preview remembered for noneTTL (preview.Cache keeps only successes), and after a 429 no Apple call
// at all until its Retry-After (else openFor) is over: retrying into Apple's window only prolongs it.
type guardedPreviews struct {
	src      preview.Source
	throttle *ratelimit.Throttle
	now      func() time.Time

	mu        sync.Mutex
	none      map[string]time.Time
	openUntil time.Time
}

func newGuardedPreviews(src preview.Source, now func() time.Time) *guardedPreviews {
	if now == nil {
		now = time.Now
	}
	return &guardedPreviews{src: src, throttle: ratelimit.NewThrottle(previewGap, previewMaxWait), now: now,
		none: map[string]time.Time{}}
}

func (g *guardedPreviews) PreviewURL(ctx context.Context, s deck.Song) (string, error) {
	now := g.now()
	g.mu.Lock()
	until, none := g.none[s.ID]
	open := now.Before(g.openUntil)
	g.mu.Unlock()
	switch {
	case none && now.Before(until):
		return "", preview.ErrNone
	case open:
		return "", errPreviewsUnavailable
	}
	if err := g.throttle.Wait(ctx, "apple"); err != nil {
		return "", fmt.Errorf("%w: %w", errPreviewsUnavailable, err)
	}
	ctx, cancel := context.WithTimeout(ctx, lookupDeadline)
	defer cancel()
	u, err := g.src.PreviewURL(ctx, s)
	switch {
	case errors.Is(err, preview.ErrNone):
		g.mu.Lock()
		if len(g.none) >= maxNone {
			clear(g.none)
		}
		g.none[s.ID] = g.now().Add(noneTTL)
		g.mu.Unlock()
	case errors.Is(err, apple.ErrRateLimited):
		wait := openFor
		var rl *apple.RateLimitedError
		if errors.As(err, &rl) && rl.RetryAfter > 0 {
			wait = rl.RetryAfter
		}
		g.mu.Lock()
		g.openUntil = g.now().Add(wait)
		g.mu.Unlock()
		slog.Warn("apple music rate limited the previews; no lookups until it is over", "for", wait.String())
		return "", fmt.Errorf("%w: %w", errPreviewsUnavailable, err)
	}
	return u, err
}

// newApplePreviews is the Apple Music preview source: the web player's token kept in the data dir across restarts,
// each request tried once within requestTimeout, lookups guarded, every URL cached a day.
func newApplePreviews(dataDir string) preview.Source {
	tokenPath := filepath.Join(dataDir, "apple-token.json")
	tokens := &apple.Source{HTTP: &http.Client{Timeout: requestTimeout},
		OnNew: func(t apple.Token) {
			if err := apple.SaveToken(tokenPath, t); err != nil {
				slog.Warn("apple token not kept", logging.Err(err))
			}
		}}
	if t, ok := apple.LoadToken(tokenPath, time.Now()); ok {
		tokens.Seed(t)
	}
	client := &apple.Client{HTTP: &http.Client{Timeout: requestTimeout}, Dev: tokens, Tries: 1}
	return preview.NewCache(newGuardedPreviews(applePreviews{catalog: client}, nil), preview.TTL, nil)
}
