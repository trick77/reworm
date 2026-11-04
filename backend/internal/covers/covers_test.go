package covers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/deck"
)

// png is the smallest thing DetectContentType calls image/png.
var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")

func get(h *Handler, id string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Mount(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/covers/"+id, nil))
	return rec
}

func TestCover_fetchedOnceThenFromDisk(t *testing.T) {
	var hits atomic.Int32
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/art/600x600bb.jpg", http.StatusFound)
			return
		}
		gotPath = r.URL.Path
		_, _ = w.Write(png)
	}))
	defer upstream.Close()
	stored := map[string]string{"dq": upstream.URL + "/art/{w}x{h}bb.jpg", "caa": upstream.URL + "/redirect"}
	lookup := func(_ context.Context, id string) (string, error) {
		u, ok := stored[id]
		if !ok {
			return "", deck.ErrNotFound
		}
		return u, nil
	}
	dir := filepath.Join(t.TempDir(), "covers")
	h := New(lookup, dir, nil)
	for range 2 {
		rec := get(h, "dq")
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" ||
			rec.Header().Get("Cache-Control") != cacheControl || rec.Body.Len() != len(png) {
			t.Fatalf("cover = %d %v", rec.Code, rec.Header())
		}
	}
	if hits.Load() != 1 || gotPath != "/art/600x600bb.jpg" {
		t.Errorf("upstream hits = %d, path %q; want one fetch at 600x600", hits.Load(), gotPath)
	}
	if rec := get(h, "caa"); rec.Code != http.StatusOK {
		t.Errorf("redirected cover = %d", rec.Code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Errorf("cache dir = %v, %v", entries, err)
	}
}

func TestCover_notFound(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			_, _ = w.Write([]byte("<html>nope</html>"))
		case "/big":
			_, _ = w.Write(append(png, make([]byte, maxBytes)...))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	stored := map[string]string{"none": "", "gone": upstream.URL + "/x.jpg", "html": upstream.URL + "/html",
		"big": upstream.URL + "/big", "bad": "://bad", "down": "http://127.0.0.1:1/x.jpg"}
	lookup := func(_ context.Context, id string) (string, error) {
		if id == "dberr" {
			return "", errors.New("db down")
		}
		u, ok := stored[id]
		if !ok {
			return "", deck.ErrNotFound
		}
		return u, nil
	}
	h := New(lookup, filepath.Join(t.TempDir(), "covers"), nil)
	for _, id := range []string{"unknown", "none", "gone", "html", "big", "bad", "down"} {
		if rec := get(h, id); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", id, rec.Code)
		}
	}
	if rec := get(h, "dberr"); rec.Code != http.StatusNotFound {
		t.Errorf("db error = %d, want 404 (the UI draws the record)", rec.Code)
	}
}

func TestCover_oneFetchAtATimeAndFailuresRemembered(t *testing.T) {
	var hits atomic.Int32
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/gone.jpg" {
			http.NotFound(w, r)
			return
		}
		<-release
		_, _ = w.Write(png)
	}))
	defer upstream.Close()
	lookup := func(_ context.Context, id string) (string, error) { return upstream.URL + "/" + id + ".jpg", nil }
	h := New(lookup, filepath.Join(t.TempDir(), "covers"), nil)
	now := time.Unix(1_790_000_000, 0)
	var mu sync.Mutex
	h.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }

	// Eight reveals at once: one upstream fetch.
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Go(func() { codes[i] = get(h, "dq").Code })
	}
	for hits.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // let the others join the flight
	close(release)
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusOK {
			t.Errorf("codes = %v", codes)
			break
		}
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("upstream hits = %d, want 1", n)
	}

	// A 404 is remembered for an hour.
	hits.Store(0)
	for range 3 {
		if rec := get(h, "gone"); rec.Code != http.StatusNotFound {
			t.Fatalf("gone = %d", rec.Code)
		}
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("hits for a missing cover = %d, want 1", n)
	}
	mu.Lock()
	now = now.Add(badCoverTTL)
	mu.Unlock()
	get(h, "gone")
	if n := hits.Load(); n != 2 {
		t.Errorf("hits after the hour = %d, want 2", n)
	}
}

func TestCover_failuresBounded(t *testing.T) {
	h := New(nil, t.TempDir(), nil)
	for i := range maxFailures + 1 {
		h.fail(string(rune(i)), time.Hour)
	}
	if len(h.failed) > maxFailures {
		t.Errorf("failures = %d", len(h.failed))
	}
}

func TestCover_unwritableDir(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(png) }))
	defer upstream.Close()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h := New(func(context.Context, string) (string, error) { return upstream.URL + "/a.jpg", nil }, filepath.Join(file, "covers"), nil)
	if rec := get(h, "dq"); rec.Code != http.StatusOK {
		t.Errorf("unwritable cache = %d, want the cover anyway", rec.Code)
	}
}

func TestSourceURL(t *testing.T) {
	if got := SourceURL("https://is1.mzstatic.com/a/{w}x{h}bb.{f}"); got != "https://is1.mzstatic.com/a/600x600bb.jpg" {
		t.Errorf("apple = %q", got)
	}
	if got := SourceURL("https://coverartarchive.org/release-group/x/front-500"); got != "https://coverartarchive.org/release-group/x/front-500" {
		t.Errorf("caa = %q", got)
	}
}
