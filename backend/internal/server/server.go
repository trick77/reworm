// Package server is the HTTP surface: kit's probes, the API and the embedded SPA behind kit's middleware.
package server

import (
	"log/slog"
	"net/http"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/version"
)

// New returns the handler. static serves everything outside /api and the probes (the SPA); checks gate /readyz.
func New(static http.Handler, checks ...httpx.Check) http.Handler {
	mux := http.NewServeMux()
	httpx.Health(mux, slog.Default(), checks...)
	mux.HandleFunc("GET /api/version", handleVersion)
	// GET (HEAD included) only: a wrong method on an API route is a 405, not the SPA's 404.
	mux.Handle("GET /", static)
	// Access log outermost, so a panic's line and its 500 carry the request id; the security headers outside
	// Recover, so the 500 keeps them.
	return httpx.Chain(mux,
		httpx.AccessLog(nil, httpx.AccessLogOptions{ClientIP: ClientIP, RoutineLevel: slog.LevelInfo}),
		httpx.SecurityHeaders(""),
		httpx.Recover(nil),
		httpx.CrossOrigin,
	)
}

// trustedHops is the proxies in front of the app: one Traefik (compose.yaml).
const trustedHops = 1

// ClientIP is the caller's address behind Traefik, what the access log and rate limits key on.
func ClientIP(r *http.Request) string { return httpx.ClientIP(r, trustedHops) }

type versionResponse struct {
	Version string `json:"version"`
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, versionResponse{Version: version.String()})
}
