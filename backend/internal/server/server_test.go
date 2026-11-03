package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/sse"
	"github.com/trick77/ship/kit/version"
)

var static = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("SPA")) })

func newHandler(checks ...httpx.Check) http.Handler {
	return New(Options{Static: static, Checks: checks})
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "127.0.0.1:40000" // the probes answer loopback only
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVersion(t *testing.T) {
	rec := get(newHandler(), "/api/version")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body versionResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != version.String() {
		t.Errorf("version = %q, want %q", body.Version, version.String())
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("security headers missing: %v", rec.Header())
	}
}

func TestProbes(t *testing.T) {
	if rec := get(newHandler(), "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", rec.Code)
	}
	down := httpx.Check{Name: "database", Run: func(context.Context) error { return errors.New("gone") }}
	if rec := get(newHandler(down), "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz with a failing check = %d, want 503", rec.Code)
	}
}

func TestStaticGetsTheRest(t *testing.T) {
	if rec := get(newHandler(), "/party/K7Q4"); rec.Code != http.StatusOK || rec.Body.String() != "SPA" {
		t.Errorf("/party/K7Q4 = %d %q, want the SPA", rec.Code, rec.Body.String())
	}
}

func TestWrongMethodIs405(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/version", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/version = %d, want 405", rec.Code)
	}
}

type pingRoutes struct{}

func (pingRoutes) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("pong")) })
}

func TestAPIRoutesMounted(t *testing.T) {
	h := New(Options{Static: static, API: []Routes{pingRoutes{}}})
	if rec := get(h, "/api/ping"); rec.Code != http.StatusOK || rec.Body.String() != "pong" {
		t.Errorf("/api/ping = %d %q", rec.Code, rec.Body.String())
	}
}

type streamRoutes struct{}

func (streamRoutes) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stream", func(w http.ResponseWriter, r *http.Request) {
		sw, err := sse.Open(w)
		if err != nil {
			return
		}
		defer sw.Close()
		_ = sw.Send("state", "{}")
		<-r.Context().Done()
	})
}

// The room streams pass the whole middleware chain: every layer must flush through.
func TestStreamsFlushThroughTheChain(t *testing.T) {
	srv := httptest.NewServer(New(Options{Static: static, API: []Routes{streamRoutes{}}}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || line != "event: state\n" {
		t.Errorf("first line = %q, %v", line, err)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.18.0.2:40000" // Traefik
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.7")
	// Reached directly: the header is the caller's to forge, only the TCP peer counts.
	if got := ClientIP(0)(req); got != "172.18.0.2" {
		t.Errorf("ClientIP(0) = %q, want the TCP peer", got)
	}
	// Behind one Traefik: the entry it appended, never the forgeable leftmost.
	if got := ClientIP(1)(req); got != "203.0.113.7" {
		t.Errorf("ClientIP(1) = %q, want the entry Traefik appended", got)
	}
}

func TestCrossOriginMutationRefused(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/version", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site POST = %d, want 403", rec.Code)
	}
}
