package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/trick77/ship/kit/spa"
)

func serve(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandler_unknownRouteGetsTheShell(t *testing.T) {
	h := handler(fstest.MapFS{"index.html": {Data: []byte("INDEX")}})
	if rec := serve(h, "/join/K7Q4"); rec.Code != http.StatusOK || rec.Body.String() != "INDEX" {
		t.Errorf("/join/K7Q4 = %d %q, want 200 INDEX", rec.Code, rec.Body.String())
	}
	// A mistyped endpoint stays a 404, not HTML posing as JSON.
	if rec := serve(h, "/api/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("/api/nope = %d, want 404", rec.Code)
	}
}

// A shell as Vite builds it with base "./": relative refs, the base-path meta empty.
const viteShell = `<!doctype html><html><head><meta name="base-path" content="" />` +
	`<link rel="icon" href="./icon.svg" /><link rel="manifest" href="./manifest.webmanifest" />` +
	`<script type="module" crossorigin src="./assets/index-abc.js"></script>` +
	`<link rel="stylesheet" crossorigin href="./assets/index-abc.css"></head><body></body></html>`

// The shell is served on deep routes (/room/K7Q4): every ref must be absolute, under the base path, or the browser
// resolves it against the route and gets the shell back as a script.
func TestHandler_shellRefsUnderTheBase(t *testing.T) {
	for base, want := range map[string]string{"": `src="/assets/index-abc.js"`, "/reworm": `src="/reworm/assets/index-abc.js"`} {
		h := handlerAt(fstest.MapFS{"index.html": {Data: []byte(viteShell)}}, base)
		body := serve(h, "/room/K7Q4").Body.String()
		if !strings.Contains(body, want) || !strings.Contains(body, `href="`+base+`/manifest.webmanifest"`) ||
			!strings.Contains(body, `<meta name="base-path" content="`+base+`" />`) {
			t.Errorf("base %q: shell = %s", base, body)
		}
		if strings.Contains(body, `="./`) {
			t.Errorf("base %q: a relative ref is left: %s", base, body)
		}
	}
}

// The real build: if Vite's output changes shape (refs no longer "./", the meta gone), this fails instead of a
// deep-route reload breaking in a browser.
func TestSPAHandler_builtShellRewritten(t *testing.T) {
	if !spa.Built(dist()) {
		t.Skip("UI not built")
	}
	for _, base := range []string{"", "/reworm"} {
		body := serve(handlerAt(dist(), base), "/room/K7Q4").Body.String()
		if strings.Contains(body, `="./`) || !strings.Contains(body, `src="`+base+`/assets/`) ||
			!strings.Contains(body, `<meta name="base-path" content="`+base+`"`) {
			t.Errorf("base %q: built shell not rewritten: %s", base, body)
		}
	}
}

func TestHandler_unbuiltServesPlaceholder(t *testing.T) {
	rec := serve(handler(fstest.MapFS{".gitkeep": {}}), "/")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "built without its UI") {
		t.Errorf("unbuilt / = %d %q, want 503 and the placeholder", rec.Code, rec.Body.String())
	}
}

func TestSPAHandler_tracksTheBuild(t *testing.T) {
	rec := serve(SPAHandler(""), "/")
	built := spa.Built(dist())
	if _, err := fs.Stat(dist(), "index.html"); (err == nil) != built {
		t.Fatalf("spa.Built = %v, disagrees with the embedded dist", built)
	}
	if want := map[bool]int{true: http.StatusOK, false: http.StatusServiceUnavailable}[built]; rec.Code != want {
		t.Errorf("/ = %d, want %d (built %v)", rec.Code, want, built)
	}
}
