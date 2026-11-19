// Package web embeds the built frontend (web/dist) and serves it as a SPA.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

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

// SPAHandler serves the embedded frontend (kit's spa) for an app under base ("" at the root, else PUBLIC_PATH;
// mounted with the prefix stripped): files as they are, /assets/ cached forever, the shell on every other path
// outside /api; the placeholder there when the UI was not built.
func SPAHandler(base string) http.Handler { return handlerAt(dist(), base) }

func handler(d fs.FS) http.Handler { return handlerAt(d, "") }

// handlerAt serves d with its shell rewritten for base. Vite builds with base "./" so one bundle runs under any
// prefix: the chunks load relative to each other, but the shell is served on deep routes (/room/K7Q4), where
// "./assets/…" would resolve to /room/assets/… and come back as the shell. So every "./" ref in the shell becomes
// base + "/", and the base-path meta tells the UI its prefix (router, API, links). A <base href> would do it in one
// tag, but kit's CSP sets base-uri 'none'.
func handlerAt(d fs.FS, base string) http.Handler {
	o := spa.Options{Placeholder: placeholder}
	if shell, err := fs.ReadFile(d, "index.html"); err == nil {
		s := strings.ReplaceAll(string(shell), `="./`, `="`+base+`/`)
		s = strings.Replace(s, `<meta name="base-path" content=""`, `<meta name="base-path" content="`+base+`"`, 1)
		rewritten := []byte(s)
		o.Index = func([]byte, *http.Request) []byte { return rewritten }
	}
	return spa.Handler(d, o)
}
