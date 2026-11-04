package httpcache

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trick77/ship/kit/ratelimit"
)

func get(t *testing.T, c *http.Client, method, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(b)
}

func TestTransport_cachesStableAnswersOnly(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Other", "dropped")
		_, _ = io.WriteString(w, "hello")
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); http.NotFound(w, nil) })
	mux.HandleFunc("/busy", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/denied", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "https://elsewhere.example/img.jpg", http.StatusTemporaryRedirect)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	dir := t.TempDir()
	c := &http.Client{
		Transport:     &Transport{Dir: dir},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	for _, tc := range []struct {
		method, path string
		status       int
		calls        int32 // server hits after two requests
	}{
		{http.MethodGet, "/ok", 200, 1},
		{http.MethodGet, "/gone", 404, 1},
		{http.MethodHead, "/moved", 307, 1},
		{http.MethodGet, "/busy", 503, 2},
		{http.MethodGet, "/denied", 401, 2},
	} {
		hits.Store(0)
		for i := range 2 {
			resp, body := get(t, c, tc.method, srv.URL+tc.path)
			if resp.StatusCode != tc.status {
				t.Fatalf("%s %s: status %d, want %d", tc.method, tc.path, resp.StatusCode, tc.status)
			}
			if tc.path == "/ok" && body != "hello" {
				t.Errorf("body %q", body)
			}
			if i == 1 && tc.calls == 1 {
				if resp.Header.Get(HitHeader) != "1" {
					t.Errorf("%s: second answer not from the cache", tc.path)
				}
				if tc.path == "/ok" && (resp.Header.Get("Content-Type") != "text/plain" || resp.Header.Get("X-Other") != "") {
					t.Errorf("headers %v", resp.Header)
				}
				if tc.path == "/moved" && resp.Header.Get("Location") != "https://elsewhere.example/img.jpg" {
					t.Errorf("location %q", resp.Header.Get("Location"))
				}
			}
		}
		if hits.Load() != tc.calls {
			t.Errorf("%s %s: server hit %d times, want %d", tc.method, tc.path, hits.Load(), tc.calls)
		}
	}

	// GET and HEAD of one URL are separate entries; a corrupt file is a miss, rewritten.
	hits.Store(0)
	get(t, c, http.MethodHead, srv.URL+"/ok")
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.gz"))
	for _, f := range files {
		if err := os.WriteFile(f, []byte("junk"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, body := get(t, c, http.MethodGet, srv.URL+"/ok"); body != "hello" {
		t.Errorf("after corruption body %q", body)
	}
	if hits.Load() != 2 {
		t.Errorf("hits = %d, want 2 (HEAD miss + corrupt GET)", hits.Load())
	}
}

func TestTransport_evictAndCutBodies(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if r.URL.Path == "/cut" && n == 1 {
			// Announces 100 bytes, sends 5, drops the connection.
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "hello")
			panic(http.ErrAbortHandler)
		}
		_, _ = io.WriteString(w, "body")
	}))
	defer srv.Close()
	tr := &Transport{Dir: t.TempDir()}
	c := &http.Client{Transport: tr}

	// A cut-off body is an error and never stored: the next request goes out.
	if _, err := c.Get(srv.URL + "/cut"); err == nil { //nolint:noctx // test
		t.Error("cut body: no error")
	}
	if _, body := get(t, c, http.MethodGet, srv.URL+"/cut"); body != "body" || hits.Load() != 2 {
		t.Errorf("after a cut body: %q, %d hits", body, hits.Load())
	}

	// Evict drops a stored answer; evicting one that is not stored is no error.
	hits.Store(0)
	get(t, c, http.MethodGet, srv.URL+"/x?q=a+b&filter%5Bisrc%5D=X")
	if err := tr.Evict(http.MethodGet, srv.URL+"/x?q=a+b&filter%5Bisrc%5D=X"); err != nil {
		t.Fatal(err)
	}
	get(t, c, http.MethodGet, srv.URL+"/x?q=a+b&filter%5Bisrc%5D=X")
	if hits.Load() != 2 {
		t.Errorf("hits = %d, want 2 (evicted answer fetched again)", hits.Load())
	}
	if err := tr.Evict(http.MethodGet, srv.URL+"/never"); err != nil {
		t.Errorf("evict missing: %v", err)
	}
	if err := tr.Evict(http.MethodGet, "://bad"); err == nil {
		t.Error("evict bad url: no error")
	}
}

func TestTransport_passesOtherMethodsAndErrors(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	c := &http.Client{Transport: &Transport{Dir: t.TempDir()}}
	for range 2 {
		get(t, c, http.MethodPost, srv.URL)
	}
	if hits.Load() != 2 {
		t.Errorf("POST hits = %d, want 2", hits.Load())
	}
	srv.Close()
	if _, err := c.Get(srv.URL + "/x"); err == nil { //nolint:noctx // test
		t.Error("closed server: no error")
	}
	// A cache dir that cannot be created fails the request instead of losing the answer silently.
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv2.Close()
	c2 := &http.Client{Transport: &Transport{Dir: blocker}}
	if _, err := c2.Get(srv2.URL); err == nil { //nolint:noctx // test
		t.Error("unwritable cache: no error")
	}
}

func TestPaced_spacesRequests(t *testing.T) {
	var at []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { at = append(at, time.Now()) }))
	defer srv.Close()
	c := &http.Client{Transport: &Paced{Throttle: ratelimit.NewThrottle(50*time.Millisecond, time.Second)}}
	for range 3 {
		get(t, c, http.MethodGet, srv.URL)
	}
	if len(at) != 3 || at[2].Sub(at[0]) < 90*time.Millisecond {
		t.Errorf("3 requests within %v, want >= 100ms apart in all", at[len(at)-1].Sub(at[0]))
	}

	// A caller that gives up while waiting for its turn gets its context's error.
	slow := &http.Client{Transport: &Paced{Throttle: ratelimit.NewThrottle(time.Hour, 2*time.Hour)}}
	get(t, slow, http.MethodGet, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if _, err := slow.Do(req); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the caller's deadline", err)
	}
}

func TestOffline_answersFromTheCacheOnly(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); _, _ = io.WriteString(w, "x") }))
	defer srv.Close()
	dir := t.TempDir()
	get(t, &http.Client{Transport: &Transport{Dir: dir}}, http.MethodGet, srv.URL+"/a")
	off := &http.Client{Transport: &Transport{Dir: dir, Next: Offline{}}}
	if _, body := get(t, off, http.MethodGet, srv.URL+"/a"); body != "x" {
		t.Errorf("cached body %q", body)
	}
	if _, err := off.Get(srv.URL + "/b"); !errors.Is(err, ErrOffline) { //nolint:noctx // test
		t.Errorf("miss: %v, want ErrOffline", err)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

// A secret in the query (an API key) is no part of the cache: not in the key, not on disk. A new key finds the
// answers the old one fetched; a URL without the parameter keys as it always did.
func TestTransport_redactsSecretParams(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); _, _ = io.WriteString(w, "x") }))
	defer srv.Close()
	dir := t.TempDir()
	tr := &Transport{Dir: dir, Redact: []string{"api_key"}}
	c := &http.Client{Transport: tr}
	get(t, c, http.MethodGet, srv.URL+"/q?track=a&api_key=old-secret")
	if _, body := get(t, c, http.MethodGet, srv.URL+"/q?track=a&api_key=new-secret"); body != "x" || hits.Load() != 1 {
		t.Errorf("rotated key: body %q, %d hits, want the cached answer", body, hits.Load())
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // test dir
		if err != nil {
			return err
		}
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return err
		}
		plain, err := io.ReadAll(zr)
		if err != nil {
			return err
		}
		if bytes.Contains(plain, []byte("secret")) {
			t.Errorf("%s stores the key: %s", p, plain)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Evict(http.MethodGet, srv.URL+"/q?track=a&api_key=other"); err != nil {
		t.Fatal(err)
	}
	get(t, c, http.MethodGet, srv.URL+"/q?track=a&api_key=old-secret")
	if hits.Load() != 2 {
		t.Errorf("evicted by another key: %d hits, want 2", hits.Load())
	}
	// Without the parameter the URL keys exactly as before (no re-encoding: existing caches stay valid).
	u := "https://x.example/s?b=2&a=1"
	if got, want := tr.key(http.MethodGet, tr.id(u)), (&Transport{Dir: dir}).key(http.MethodGet, u); got != want {
		t.Errorf("key moved for a URL without a secret: %s != %s", got, want)
	}
}
