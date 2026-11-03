// Package web embeds the built frontend (web/dist) and serves it as a SPA.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/trick77/ship/kit/spa"
)

// dist holds the Vite build, or only .gitkeep in a fresh clone (ship check empties it before the UI build; the
// image builds its own).
//
//go:embed all:dist
var distFS embed.FS

// placeholder is what an unbuilt binary answers on the app's routes, with a 503 (kit's spa): a Go-only build says
// so instead of serving a blank page.
//
//go:embed placeholder.html
var placeholder []byte

func dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // embedded at build time; a programmer error
	}
	return sub
}

// SPAHandler serves the embedded frontend (kit's spa): files as they are, /assets/ cached forever, the shell on
// every other path outside /api; the placeholder there when the UI was not built.
func SPAHandler() http.Handler { return handler(dist()) }

func handler(d fs.FS) http.Handler {
	return spa.Handler(d, spa.Options{Placeholder: placeholder})
}
