package covers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

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
