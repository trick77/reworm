package lastfm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// dancingQueen is track.getInfo as Last.fm answers it (trimmed, 2026-11-04): listeners a string, tags without counts.
const dancingQueen = `{"track":{"name":"Dancing Queen","mbid":"006cfafd","listeners":"2664004","playcount":"22452650",
"artist":{"name":"ABBA","mbid":"d87e52c5"},"toptags":{"tag":[{"name":"Disco","url":"u"},{"name":"pop","url":"u"},
{"name":"70s","url":"u"}]}}}`

// udo is artist.getTopTags: counts as numbers.
const udo = `{"toptags":{"tag":[{"count":100,"name":"Deutschrock","url":"u"},{"count":81,"name":"german","url":"u"}],
"@attr":{"artist":"Udo Lindenberg"}}}`

func noSleep(context.Context, time.Duration) error { return nil }

func TestClient_trackAndArtistTags(t *testing.T) {
	var flaky atomic.Int32
	var evicted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/2.0/" || q.Get("api_key") != "k3y" || q.Get("format") != "json" || q.Get("autocorrect") != "1" ||
			r.Header.Get("User-Agent") != "Reworm/test" {
			t.Errorf("request %s %v %q", r.URL.Path, q, r.Header.Get("User-Agent"))
		}
		switch q.Get("method") + "|" + q.Get("artist") + "|" + q.Get("track") {
		case "track.getInfo|ABBA|Dancing Queen":
			_, _ = fmt.Fprint(w, dancingQueen)
		case "track.getInfo|Flaky|Song":
			// Rate limited inside a 200, then a 503, then the answer.
			switch flaky.Add(1) {
			case 1:
				_, _ = fmt.Fprint(w, `{"error":29,"message":"Rate Limit Exceeded"}`)
			case 2:
				w.WriteHeader(http.StatusServiceUnavailable)
			default:
				_, _ = fmt.Fprint(w, `{"track":{"name":"Song","listeners":"7","artist":{"name":"Flaky"},"toptags":""}}`)
			}
		case "track.getInfo|Solo|Song":
			// A list of one is the object itself.
			_, _ = fmt.Fprint(w, `{"track":{"name":"Song","listeners":"3","artist":{"name":"Solo"},"toptags":{"tag":{"name":"pop"}}}}`)
		case "artist.getTopTags|Udo Lindenberg|":
			_, _ = fmt.Fprint(w, udo)
		default:
			_, _ = fmt.Fprint(w, `{"error":6,"message":"Track not found","links":[]}`)
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Key: "k3y", HTTP: srv.Client(), UserAgent: "Reworm/test", Sleep: noSleep,
		Evict: func(_, u string) error { evicted = append(evicted, u); return nil }}
	ctx := context.Background()

	tr, ok, err := c.Track(ctx, "ABBA", "Dancing Queen")
	if err != nil || !ok || tr.Name != "Dancing Queen" || tr.Artist != "ABBA" || tr.Listeners != 2664004 ||
		!slices.Equal(tr.Tags, []Tag{{Name: "Disco"}, {Name: "pop"}, {Name: "70s"}}) {
		t.Errorf("Dancing Queen = %+v, %v, %v", tr, ok, err)
	}
	if _, ok, err := c.Track(ctx, "Nobody", "Nothing"); ok || err != nil {
		t.Errorf("unknown track: found %v, %v", ok, err)
	}
	// An empty tag list comes as "" from Last.fm at times: no tags, no error.
	if tr, ok, err := c.Track(ctx, "Flaky", "Song"); !ok || err != nil || tr.Listeners != 7 || len(tr.Tags) != 0 {
		t.Errorf("flaky = %+v, %v, %v", tr, ok, err)
	}
	// Both failures evicted (the 503 is never cached, the eviction a no-op), the answer kept.
	if len(evicted) != 2 || !strings.Contains(evicted[0], "Flaky") || !strings.Contains(evicted[1], "Flaky") {
		t.Errorf("evicted %v, want the two failed Flaky answers", evicted)
	}
	if tr, ok, err := c.Track(ctx, "Solo", "Song"); !ok || err != nil || !slices.Equal(tr.Tags, []Tag{{Name: "pop"}}) {
		t.Errorf("one tag = %+v, %v, %v", tr, ok, err)
	}
	tags, err := c.ArtistTags(ctx, "Udo Lindenberg")
	if err != nil || !slices.Equal(tags, []Tag{{"Deutschrock", 100}, {"german", 81}}) {
		t.Errorf("Udo = %+v, %v", tags, err)
	}
	if tags, err := c.ArtistTags(ctx, "Nobody"); err != nil || len(tags) != 0 {
		t.Errorf("unknown artist = %+v, %v", tags, err)
	}
}

// A bad key fails at once and its answer is evicted (a 200 carries it as often: the cache must not replay it to the
// next key), and no error names the URL: it carries the key. So does a 4xx without a Last.fm error.
func TestClient_errorsNeverShowTheKey(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Query().Get("artist") == "Page" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, "<html>gone</html>")
			return
		}
		_, _ = fmt.Fprint(w, `{"error":10,"message":"Invalid API key - You must be granted a valid key by last.fm"}`)
	}))
	evicted := 0
	c := &Client{BaseURL: srv.URL, Key: "s3cret", HTTP: srv.Client(), Sleep: noSleep,
		Evict: func(string, string) error { evicted++; return nil }}
	_, _, err := c.Track(context.Background(), "A", "B")
	if err == nil || !strings.Contains(err.Error(), "error 10") || hits.Load() != 1 || strings.Contains(err.Error(), "s3cret") || evicted != 1 {
		t.Errorf("bad key: %v after %d requests, %d evicted", err, hits.Load(), evicted)
	}
	if _, err := c.ArtistTags(context.Background(), "Page"); err == nil || !strings.Contains(err.Error(), "status 404") || evicted != 2 {
		t.Errorf("404 page: %v, %d evicted", err, evicted)
	}
	srv.Close()
	_, _, err = c.Track(context.Background(), "A", "B")
	if err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("network error: %v", err)
	}
	if !strings.Contains(err.Error(), "after 6 tries") {
		t.Errorf("network error not retried: %v", err)
	}
}

func TestClient_givesUpOnACancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}}
	if _, err := c.ArtistTags(ctx, "A"); err == nil {
		t.Error("no error after cancel")
	}
	// An undecodable 200 is retried, then reported.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "<html>") }))
	defer bad.Close()
	c = &Client{BaseURL: bad.URL, HTTP: bad.Client(), Sleep: noSleep}
	if _, err := c.ArtistTags(context.Background(), "A"); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("undecodable: %v", err)
	}
}
