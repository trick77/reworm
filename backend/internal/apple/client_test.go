package apple

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTokens hands out "tok1", then "tok2" after an Invalidate.
type fakeTokens struct {
	invalidated atomic.Int32
	err         error
}

func (f *fakeTokens) Token(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return fmt.Sprintf("tok%d", f.invalidated.Load()+1), nil
}

func (f *fakeTokens) Invalidate(string) { f.invalidated.Add(1) }

func song(id, album, date string, preview bool) string {
	prev := `[]`
	if preview {
		prev = `[{"url":"https://audio.example/` + id + `.m4a"}]`
	}
	return fmt.Sprintf(`{"id":%q,"type":"songs","attributes":{"name":"Dancing Queen","artistName":"ABBA","albumName":%q,`+
		`"releaseDate":%q,"isrc":"SEA017600030","artwork":{"url":"https://img.example/%s/{w}x{h}bb.jpg"},"previews":%s}}`,
		id, album, date, id, prev)
}

func newClient(t *testing.T, h http.Handler) (*Client, *fakeTokens) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	tok := &fakeTokens{}
	return &Client{BaseURL: srv.URL, HTTP: srv.Client(), Dev: tok,
		Sleep: func(context.Context, time.Duration) error { return nil }}, tok
}

func TestSongsByISRC_followsNextAndSendsThePlayersHeaders(t *testing.T) {
	c, _ := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Origin") != "https://music.apple.com" ||
			r.Header.Get("Authorization") != "Bearer tok1" {
			t.Errorf("request %s %v", r.Method, r.Header)
		}
		if r.URL.Path != "/v1/catalog/ch/songs" || r.URL.Query().Get("filter[isrc]") != "SEA017600030,SEAYD7601020" {
			t.Errorf("url %s", r.URL)
		}
		if r.URL.Query().Get("offset") == "" {
			_, _ = fmt.Fprintf(w, `{"data":[%s],"next":"/v1/catalog/ch/songs?filter%%5Bisrc%%5D=SEA017600030,SEAYD7601020&offset=25"}`,
				song("1", "Arrival", "1976-10-11", true))
			return
		}
		_, _ = fmt.Fprintf(w, `{"data":[%s]}`, song("2", "Gold", "1992-09-21", false))
	}))
	got, err := c.SongsByISRC(context.Background(), "ch", []string{"SEA017600030", "SEAYD7601020"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Album != "Arrival" || !got[0].Preview || got[1].Preview {
		t.Fatalf("got %+v", got)
	}
	if got[0].Artwork != "https://img.example/1/{w}x{h}bb.jpg" || got[0].Year() != 1976 || got[0].ISRC != "SEA017600030" {
		t.Errorf("song %+v", got[0])
	}
	if none, err := c.SongsByISRC(context.Background(), "ch", nil); none != nil || err != nil {
		t.Errorf("no isrcs: %v, %v", none, err)
	}
}

func TestSongsByISRC_notFoundIsEmptyAndForeignNextFails(t *testing.T) {
	c, _ := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/catalog/us/songs" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[],"next":"https://evil.example/v1/x"}`)
	}))
	if got, err := c.SongsByISRC(context.Background(), "us", []string{"X"}); err != nil || len(got) != 0 {
		t.Errorf("404: %v, %v", got, err)
	}
	if _, err := c.SongsByISRC(context.Background(), "de", []string{"X"}); err == nil || !strings.Contains(err.Error(), "next link") {
		t.Errorf("foreign next: %v", err)
	}
}

func TestSearchSongs(t *testing.T) {
	c, _ := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/v1/catalog/de/search" || q.Get("term") != "ABBA Dancing Queen" || q.Get("types") != "songs" || q.Get("limit") != "10" {
			t.Errorf("url %s", r.URL)
		}
		_, _ = fmt.Fprintf(w, `{"results":{"songs":{"data":[%s]}}}`, strings.Replace(song("3", "Arrival", "1976", true), "https://img", "http://img", 1))
	}))
	got, err := c.SearchSongs(context.Background(), "de", "ABBA Dancing Queen", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "3" || got[0].Artwork != "" || got[0].Year() != 1976 {
		t.Errorf("got %+v (plain-http artwork must be dropped)", got)
	}
	if (Song{}).Year() != 0 {
		t.Error("year of an empty date")
	}
}

func TestSearchAlbumsAndAlbumTracks(t *testing.T) {
	c, _ := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/catalog/ch/search":
			if r.URL.Query().Get("types") != "albums" || r.URL.Query().Get("term") != "ABBA Arrival" {
				t.Errorf("url %s", r.URL)
			}
			_, _ = fmt.Fprint(w, `{"results":{"albums":{"data":[{"id":"1440820126","attributes":{"name":"Arrival (Bonus Track Version)",`+
				`"artistName":"ABBA","releaseDate":"1976-10-11"}}]}}}`)
		case "/v1/catalog/ch/albums/1440820126/tracks":
			_, _ = fmt.Fprintf(w, `{"data":[%s,{"id":"9","type":"music-videos","attributes":{"name":"Video"}}]}`,
				strings.Replace(song("1440820267", "Arrival (Bonus Track Version)", "1976-10-11", true), `"artistName"`, `"albumArtistName":"ABBA","artistName"`, 1))
		default:
			http.NotFound(w, r)
		}
	}))
	albums, err := c.SearchAlbums(context.Background(), "ch", "ABBA Arrival", 10)
	if err != nil || len(albums) != 1 || albums[0].ID != "1440820126" || albums[0].Year() != 1976 || albums[0].Artist != "ABBA" {
		t.Fatalf("albums %+v, %v", albums, err)
	}
	tracks, err := c.AlbumTracks(context.Background(), "ch", "1440820126")
	if err != nil || len(tracks) != 1 || tracks[0].ID != "1440820267" || tracks[0].AlbumArtist != "ABBA" {
		t.Fatalf("tracks %+v, %v", tracks, err)
	}
	if _, err := c.SearchAlbums(context.Background(), "xx", "x", 1); err != nil {
		t.Errorf("404 search: %v", err)
	}
}

func TestGet_refusedTokenIsRefetchedOnce(t *testing.T) {
	var calls atomic.Int32
	c, tok := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") == "Bearer tok1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, `{"results":{}}`)
	}))
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); err != nil {
		t.Fatal(err)
	}
	if tok.invalidated.Load() != 1 || calls.Load() != 2 {
		t.Errorf("invalidated %d, calls %d", tok.invalidated.Load(), calls.Load())
	}

	c, _ = newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); !errors.Is(err, ErrDevTokenRefused) {
		t.Errorf("err = %v, want ErrDevTokenRefused", err)
	}
}

func TestGet_retriesRateLimitsAndCutBodies(t *testing.T) {
	var calls atomic.Int32
	var waits []time.Duration
	c, _ := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			_, _ = fmt.Fprint(w, `{"results":{"songs":{"da`)
		default:
			_, _ = fmt.Fprint(w, `{"results":{}}`)
		}
	}))
	c.Sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || len(waits) != 2 || waits[0] != 3*time.Second || waits[1] != time.Second {
		t.Errorf("calls %d, waits %v", calls.Load(), waits)
	}

	c, _ = newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); err == nil || !strings.Contains(err.Error(), "429") {
		t.Errorf("endless 429: %v", err)
	}
	c, _ = newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, `{`) }))
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("endless cut body: %v", err)
	}
	c, _ = newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	c.Sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled wait: %v", err)
	}
}

func TestGet_errors(t *testing.T) {
	c, tok := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"errors":[{"title":"Invalid Parameter","detail":"limit"}]}`)
	}))
	if _, err := c.SearchSongs(context.Background(), "ch", "secret words", 5); err == nil ||
		!strings.Contains(err.Error(), "Invalid Parameter: limit") || strings.Contains(err.Error(), "secret") {
		t.Errorf("err = %v", err)
	}
	tok.err = errors.New("no token")
	if _, err := c.SearchSongs(context.Background(), "ch", "x", 5); err == nil || err.Error() != "no token" {
		t.Errorf("token err = %v", err)
	}
	if apiError([]byte(`{"errors":[{"title":"T"}]}`)) != "T" || apiError([]byte("x")) != "no error detail" {
		t.Error("apiError")
	}
	if retryAfter("") != 2*time.Second || retryAfter("999") != maxRetryAfter {
		t.Error("retryAfter")
	}
	down := &Client{BaseURL: "http://127.0.0.1:1", Dev: &fakeTokens{}}
	if _, err := down.SearchSongs(context.Background(), "ch", "x", 5); err == nil {
		t.Error("unreachable: no error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (&Client{}).sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleep: %v", err)
	}
	if err := (&Client{}).sleep(context.Background(), time.Millisecond); err != nil {
		t.Errorf("sleep: %v", err)
	}
}

func TestTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "apple-token.json")
	if _, ok := LoadToken(path, now); ok {
		t.Error("missing file loaded")
	}
	tok := Token{Value: "v", Expires: now.Add(time.Hour)}
	if err := SaveToken(path, tok); err != nil {
		t.Fatal(err)
	}
	if got, ok := LoadToken(path, now); !ok || got.Value != "v" {
		t.Errorf("loaded %+v, %v", got, ok)
	}
	if _, ok := LoadToken(path, now.Add(2*time.Hour)); ok {
		t.Error("expired token loaded")
	}
}
