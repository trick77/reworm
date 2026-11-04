// Package server is the HTTP surface: kit's probes, the API and the embedded SPA behind kit's middleware.
package server

import (
	"log/slog"
	"net/http"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/version"
)

// Options configures New.
type Options struct {
	// Static serves everything outside /api and the probes: the SPA.
	Static http.Handler
	// TrustedProxies is how many X-Forwarded-For entries from the right are the proxies' (BACKEND_TRUSTED_PROXIES).
	TrustedProxies int
	// Checks gate /readyz.
	Checks []httpx.Check
	// API are the route groups under /api (accounts, decks, covers, rooms, training).
	API []Routes
}

// Routes is a group of API routes.
type Routes interface {
	Mount(mux *http.ServeMux)
}

// New returns the handler.
func New(o Options) http.Handler {
	mux := http.NewServeMux()
	httpx.Health(mux, slog.Default(), o.Checks...)
	mux.HandleFunc("GET /api/version", handleVersion)
	for _, r := range o.API {
		r.Mount(mux)
	}
	// GET (HEAD included) only: a wrong method on an API route is a 405, not the SPA's 404.
	mux.Handle("GET /", o.Static)
	// Access log outermost, so a panic's line and its 500 carry the request id; the security headers outside
	// Recover, so the 500 keeps them.
	return httpx.Chain(mux,
		httpx.AccessLog(nil, httpx.AccessLogOptions{ClientIP: ClientIP(o.TrustedProxies), RoutineLevel: slog.LevelInfo}),
		httpx.SecurityHeaders(csp),
		httpx.Recover(nil),
		httpx.CrossOrigin,
	)
}

// csp is kit's default plus the clips: the player streams Apple's previews straight from Apple's audio host, and its
// unlock plays a data: WAV. Without media-src, default-src 'self' blocks both and no clip makes a sound.
const csp = httpx.DefaultCSP + "; media-src 'self' data: https://audio-ssl.itunes.apple.com"

// ClientIP resolves the caller's address behind trustedProxies proxies, what the access log and rate limits key on.
func ClientIP(trustedProxies int) func(*http.Request) string {
	return func(r *http.Request) string { return httpx.ClientIP(r, trustedProxies) }
}

type versionResponse struct {
	Version string `json:"version"`
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, versionResponse{Version: version.String()})
}
