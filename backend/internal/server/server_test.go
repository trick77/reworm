package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/version"
)

var static = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("SPA")) })

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "127.0.0.1:40000" // the probes answer loopback only
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVersion(t *testing.T) {
	rec := get(New(static), "/api/version")
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
	if rec := get(New(static), "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", rec.Code)
	}
	down := httpx.Check{Name: "database", Run: func(context.Context) error { return errors.New("gone") }}
	if rec := get(New(static, down), "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz with a failing check = %d, want 503", rec.Code)
	}
}

func TestStaticGetsTheRest(t *testing.T) {
	if rec := get(New(static), "/party/K7Q4"); rec.Code != http.StatusOK || rec.Body.String() != "SPA" {
		t.Errorf("/party/K7Q4 = %d %q, want the SPA", rec.Code, rec.Body.String())
	}
}

func TestCrossOriginMutationRefused(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/version", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	New(static).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site POST = %d, want 403", rec.Code)
	}
}
