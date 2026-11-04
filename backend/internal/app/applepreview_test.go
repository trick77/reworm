package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/toolkit/applemusic"
)

// appleServer is the web player (its token) and the catalog: song 1 in ch has a preview, song 2 lost its, ISRC
// GB1 has a track with one in de only.
func appleServer(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	enc := base64.RawURLEncoding.EncodeToString
	tok := enc([]byte(`{"alg":"ES256"}`)) + "." +
		enc([]byte(fmt.Sprintf(`{"iss":"AMPWebPlay","exp":%d}`, time.Now().Add(30*24*time.Hour).Unix()))) + "." + enc([]byte("s"))
	track := func(id, preview string) string {
		return fmt.Sprintf(`{"id":%q,"type":"songs","attributes":{"name":"Song","previews":[{"url":%q}]}}`, id, preview)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /us/browse", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<script src="/assets/index~abc.js"></script>`)
	})
	mux.HandleFunc("GET /assets/index~abc.js", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintf(w, `x="%s"`, tok) })
	mux.HandleFunc("GET /v1/catalog/{sf}/songs/{id}", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+tok || r.Header.Get("Origin") != "https://music.apple.com" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.PathValue("sf") + "/" + r.PathValue("id") {
		case "ch/1":
			_, _ = fmt.Fprintf(w, `{"data":[%s]}`, track("1", "https://audio.example/1.m4a"))
		case "ch/2":
			_, _ = fmt.Fprintf(w, `{"data":[%s]}`, track("2", ""))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /v1/catalog/{sf}/songs", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("filter[isrc]") == "GB1" && r.PathValue("sf") == "de" {
			_, _ = fmt.Fprintf(w, `{"data":[%s,%s]}`, track("8", ""), track("9", "https://audio.example/9.m4a"))
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestApplePreviews(t *testing.T) {
	var calls atomic.Int32
	srv := appleServer(t, &calls)
	client := &applemusic.Client{BaseURL: srv.URL, HTTP: srv.Client(), Dev: &applemusic.Source{BaseURL: srv.URL, HTTP: srv.Client()}}
	src := preview.NewCache(applePreviews{catalog: client}, preview.TTL, nil)
	ctx := context.Background()
	for _, tc := range []struct {
		song deck.Song
		want string
		err  error
	}{
		{deck.Song{ID: "a", AppleID: "1", Storefront: "ch"}, "https://audio.example/1.m4a", nil},
		{deck.Song{ID: "b", AppleID: "1"}, "https://audio.example/1.m4a", nil}, // storefront defaults to ch
		{deck.Song{ID: "c", AppleID: "2", Storefront: "ch", ISRC: "GB1"}, "https://audio.example/9.m4a", nil},
		{deck.Song{ID: "d", AppleID: "gone", Storefront: "ch", ISRC: "GB1"}, "https://audio.example/9.m4a", nil},
		{deck.Song{ID: "e", AppleID: "gone", Storefront: "ch"}, "", preview.ErrNone},
		{deck.Song{ID: "f"}, "", preview.ErrNone},
	} {
		got, err := src.PreviewURL(ctx, tc.song)
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Errorf("%+v: %q, %v; want %q, %v", tc.song, got, err, tc.want, tc.err)
		}
	}
	// Cached: asking again goes to nobody.
	before := calls.Load()
	if got, _ := src.PreviewURL(ctx, deck.Song{ID: "a", AppleID: "1", Storefront: "ch"}); got != "https://audio.example/1.m4a" || calls.Load() != before {
		t.Errorf("cached lookup: %q, %d new calls", got, calls.Load()-before)
	}
}

func TestWarnOffHost(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	warnOffHost("https://" + preview.AudioHost + "/itunes-assets/x.m4a")
	if buf.Len() != 0 {
		t.Errorf("on host warned: %s", buf.String())
	}
	warnOffHost("https://audio.example/1.m4a")
	if !strings.Contains(buf.String(), "audio.example") {
		t.Errorf("off host not warned: %q", buf.String())
	}
}

func TestApplePreviews_errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	defer srv.Close()
	tok := staticToken("t")
	a := applePreviews{catalog: &applemusic.Client{BaseURL: srv.URL, HTTP: srv.Client(), Dev: tok}}
	if _, err := a.PreviewURL(context.Background(), deck.Song{ID: "x", AppleID: "1"}); !errors.Is(err, applemusic.ErrRateLimited) {
		t.Errorf("by id: %v", err)
	}
	if _, err := a.PreviewURL(context.Background(), deck.Song{ID: "x", ISRC: "GB1"}); !errors.Is(err, applemusic.ErrRateLimited) {
		t.Errorf("by isrc: %v", err)
	}
}

// scripted answers each call with the next result and counts the calls.
type scripted struct {
	calls   atomic.Int32
	results []error
	block   chan struct{}
}

func (s *scripted) PreviewURL(context.Context, deck.Song) (string, error) {
	n := int(s.calls.Add(1)) - 1
	if s.block != nil {
		<-s.block
	}
	if n < len(s.results) && s.results[n] != nil {
		return "", s.results[n]
	}
	return "https://audio.example/x.m4a", nil
}

func TestGuardedPreviews_circuitOpensOnRateLimit(t *testing.T) {
	now := time.Unix(1000, 0)
	src := &scripted{results: []error{&applemusic.RateLimitedError{Path: "/v1/x", RetryAfter: time.Minute}, &applemusic.RateLimitedError{}}}
	g := newGuardedPreviews(src, func() time.Time { return now })
	ctx := context.Background()
	if _, err := g.PreviewURL(ctx, deck.Song{ID: "a"}); !errors.Is(err, errPreviewsUnavailable) || !errors.Is(err, applemusic.ErrRateLimited) {
		t.Fatalf("429: %v", err)
	}
	// Open for Retry-After: no call to Apple.
	now = now.Add(59 * time.Second)
	if _, err := g.PreviewURL(ctx, deck.Song{ID: "b"}); !errors.Is(err, errPreviewsUnavailable) || src.calls.Load() != 1 {
		t.Fatalf("open: %v after %d calls", err, src.calls.Load())
	}
	// Over: the next 429 without Retry-After opens it for openFor.
	now = now.Add(2 * time.Second)
	_, _ = g.PreviewURL(ctx, deck.Song{ID: "b"})
	now = now.Add(openFor - time.Second)
	if _, err := g.PreviewURL(ctx, deck.Song{ID: "c"}); !errors.Is(err, errPreviewsUnavailable) || src.calls.Load() != 2 {
		t.Errorf("openFor: %v after %d calls", err, src.calls.Load())
	}
	now = now.Add(2 * time.Second)
	if u, err := g.PreviewURL(ctx, deck.Song{ID: "c"}); err != nil || u == "" || src.calls.Load() != 3 {
		t.Errorf("closed again: %q, %v after %d calls", u, err, src.calls.Load())
	}
}

func TestGuardedPreviews_remembersSongsWithoutPreview(t *testing.T) {
	now := time.Unix(1000, 0)
	src := &scripted{results: []error{preview.ErrNone}}
	g := newGuardedPreviews(src, func() time.Time { return now })
	ctx := context.Background()
	for range 3 {
		if _, err := g.PreviewURL(ctx, deck.Song{ID: "a"}); !errors.Is(err, preview.ErrNone) {
			t.Fatalf("err = %v", err)
		}
	}
	if src.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (ErrNone remembered)", src.calls.Load())
	}
	now = now.Add(noneTTL + time.Second)
	if _, err := g.PreviewURL(ctx, deck.Song{ID: "a"}); err != nil || src.calls.Load() != 2 {
		t.Errorf("after noneTTL: %v, %d calls", err, src.calls.Load())
	}
}

func TestGuardedPreviews_queueIsBounded(t *testing.T) {
	src := &scripted{}
	g := newGuardedPreviews(src, nil)
	// Far more lookups than previewMaxWait/previewGap turns: the late ones are refused, not queued for long.
	var refused atomic.Int32
	done := make(chan struct{})
	for i := range 20 {
		go func() {
			if _, err := g.PreviewURL(context.Background(), deck.Song{ID: fmt.Sprint(i)}); errors.Is(err, errPreviewsUnavailable) {
				refused.Add(1)
			}
			done <- struct{}{}
		}()
	}
	for range 20 {
		<-done
	}
	if refused.Load() == 0 || src.calls.Load() > 10 {
		t.Errorf("refused %d, calls %d", refused.Load(), src.calls.Load())
	}
}

type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }
func (staticToken) Invalidate(string)                       {}

func TestPreviewSource_appleOutsideDevMode(t *testing.T) {
	src, routes, err := previewSource(Config{DataDir: t.TempDir()})
	if err != nil || src == nil || routes != nil {
		t.Errorf("non-dev: %T, %v, %v", src, routes, err)
	}
	if src, _, err := previewSource(Config{Dev: true}); err != nil || src != nil {
		t.Errorf("dev without clips: %T, %v", src, err)
	}
}
