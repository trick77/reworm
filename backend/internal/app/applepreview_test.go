package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
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
	client := &apple.Client{BaseURL: srv.URL, HTTP: srv.Client(), Dev: &apple.Source{BaseURL: srv.URL, HTTP: srv.Client()}}
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

func TestApplePreviews_errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	defer srv.Close()
	tok := staticToken("t")
	a := applePreviews{catalog: &apple.Client{BaseURL: srv.URL, HTTP: srv.Client(), Dev: tok}}
	if _, err := a.PreviewURL(context.Background(), deck.Song{ID: "x", AppleID: "1"}); !errors.Is(err, apple.ErrRateLimited) {
		t.Errorf("by id: %v", err)
	}
	if _, err := a.PreviewURL(context.Background(), deck.Song{ID: "x", ISRC: "GB1"}); !errors.Is(err, apple.ErrRateLimited) {
		t.Errorf("by isrc: %v", err)
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
