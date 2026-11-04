package httpcache

import (
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

func TestThrottle_spacesRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	now := time.Unix(1000, 0)
	var slept []time.Duration
	th := &Throttle{
		Every: time.Second,
		Now:   func() time.Time { return now },
		Sleep: func(_ context.Context, d time.Duration) error { slept = append(slept, d); now = now.Add(d); return nil },
	}
	c := &http.Client{Transport: th}
	get(t, c, http.MethodGet, srv.URL)
	now = now.Add(300 * time.Millisecond)
	get(t, c, http.MethodGet, srv.URL)
	now = now.Add(2 * time.Second)
	get(t, c, http.MethodGet, srv.URL)
	if len(slept) != 1 || slept[0] != 700*time.Millisecond {
		t.Errorf("slept %v, want [700ms]", slept)
	}

	th.Sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := c.Get(srv.URL); !errors.Is(err, context.Canceled) { //nolint:noctx // test
		t.Errorf("err = %v, want canceled", err)
	}
}

func TestThrottle_defaultSleepStopsOnCancel(t *testing.T) {
	th := &Throttle{}
	if err := th.sleep(context.Background(), time.Millisecond); err != nil {
		t.Error(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := th.sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}
