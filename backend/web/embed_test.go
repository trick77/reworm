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

func TestHandler_unbuiltServesPlaceholder(t *testing.T) {
	rec := serve(handler(fstest.MapFS{".gitkeep": {}}), "/")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "built without its UI") {
		t.Errorf("unbuilt / = %d %q, want 503 and the placeholder", rec.Code, rec.Body.String())
	}
}

func TestSPAHandler_tracksTheBuild(t *testing.T) {
	rec := serve(SPAHandler(), "/")
	built := spa.Built(dist())
	if _, err := fs.Stat(dist(), "index.html"); (err == nil) != built {
		t.Fatalf("spa.Built = %v, disagrees with the embedded dist", built)
	}
	if want := map[bool]int{true: http.StatusOK, false: http.StatusServiceUnavailable}[built]; rec.Code != want {
		t.Errorf("/ = %d, want %d (built %v)", rec.Code, want, built)
	}
}
