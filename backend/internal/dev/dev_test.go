package dev

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/store"
)

// writeWAV writes secs seconds of a sine tone as 8 kHz 8-bit mono PCM: what the e2e setup generates too.
func writeWAV(t *testing.T, file string, hz float64, secs int) {
	t.Helper()
	const rate = 8000
	n := rate * secs
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	le(uint32(36 + n))
	b.WriteString("WAVEfmt ")
	le(uint32(16))
	le(uint16(1)) // PCM
	le(uint16(1)) // mono
	le(uint32(rate))
	le(uint32(rate)) // bytes per second
	le(uint16(1))    // block align
	le(uint16(8))    // bits per sample
	b.WriteString("data")
	le(uint32(n))
	for i := range n {
		b.WriteByte(byte(128 + 60*math.Sin(2*math.Pi*hz*float64(i)/rate)))
	}
	if err := os.WriteFile(file, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPreviews(t *testing.T) {
	dir := t.TempDir()
	writeWAV(t, filepath.Join(dir, "dq.wav"), 440, 1)
	if err := os.Mkdir(filepath.Join(dir, "sub.wav"), 0o750); err != nil {
		t.Fatal(err)
	}
	p, err := NewPreviews(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if u, err := p.PreviewURL(ctx, deck.Song{ID: "dq"}); err != nil || u != "/api/dev/previews/dq.wav" {
		t.Errorf("PreviewURL(dq) = %q, %v", u, err)
	}
	// no file, a directory, a path out of the directory: no preview, so the round skips the song
	for _, id := range []string{"gone", "sub", "../dq", "x/../dq"} {
		if _, err := p.PreviewURL(ctx, deck.Song{ID: id}); !errors.Is(err, preview.ErrNone) {
			t.Errorf("PreviewURL(%q) = %v, want ErrNone", id, err)
		}
	}

	mux := http.NewServeMux()
	p.Mount(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dev/previews/dq.wav", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "RIFF") ||
		!strings.Contains(rec.Header().Get("Content-Type"), "wav") {
		t.Errorf("GET dq.wav = %d %q %.4q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	// media elements seek with ranges (WebKit needs them)
	req := httptest.NewRequest(http.MethodGet, "/api/dev/previews/dq.wav", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "RIFF" {
		t.Errorf("range = %d %q", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/api/dev/previews/gone.wav", "/api/dev/previews/sub.wav",
		"/api/dev/previews/..%2Fdq.wav"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

func TestNewPreviews_needsADirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{filepath.Join(dir, "missing"), file} {
		if _, err := NewPreviews(d); err == nil {
			t.Errorf("NewPreviews(%s): no error", d)
		}
	}
}

func TestLoadSongs(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	file := filepath.Join(t.TempDir(), "songs.jsonl")
	write := func(s string) {
		if err := os.WriteFile(file, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"id":"fixture-dq","artist":"ABBA","title":"Dancing Queen","year":1976,"sets":["essentials"],"genres":["pop"]}

{"id":"fixture-br","artist":"Queen","title":"Bohemian Rhapsody","year":1975,"sets":["essentials"],"categories":["number-ones"]}
`)
	if n, err := LoadSongs(ctx, db, file); err != nil || n != 2 {
		t.Fatalf("LoadSongs = %d, %v", n, err)
	}
	songs := deck.New(db)
	got, err := songs.Select(ctx, deck.Selector{Sets: []string{"essentials"}, From: 1900, To: 2100})
	if err != nil || len(got) != 2 || got[0].ID != "fixture-br" || got[1].ID != "fixture-dq" || got[1].Year != 1976 {
		t.Fatalf("active songs = %+v, %v", got, err)
	}
	d, err := songs.Decks(ctx)
	if err != nil || len(d.Sets) != 1 || d.Sets[0].Count != 2 || len(d.Categories) != 1 || len(d.Genres) != 1 {
		t.Errorf("decks = %+v, %v", d, err)
	}
	if err := CheckData(ctx, db); err != nil {
		t.Errorf("fixture songs only: %v", err)
	}

	// loading another fixture updates in place and deactivates what it left out (the row stays)
	write(`{"id":"fixture-dq","artist":"ABBA","title":"Dancing Queen","year":1977,"sets":["essentials"]}`)
	if n, err := LoadSongs(ctx, db, file); err != nil || n != 1 {
		t.Fatalf("reload = %d, %v", n, err)
	}
	got, err = songs.Select(ctx, deck.Selector{From: 1900, To: 2100})
	if err != nil || len(got) != 1 || got[0].Year != 1977 {
		t.Errorf("after reload = %+v, %v", got, err)
	}
	if s, err := songs.Get(ctx, "fixture-br"); err != nil || s.Title != "Bohemian Rhapsody" {
		t.Errorf("the left-out song's row is kept: %+v, %v", s, err)
	}
}

func TestCheckData_refusesRealSongs(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := CheckData(ctx, db); err != nil {
		t.Errorf("empty data dir: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year, year_source)
VALUES ('mbid-1', 'A', 'B', 1999, 'card')`); err != nil {
		t.Fatal(err)
	}
	if err := CheckData(ctx, db); err == nil || !strings.Contains(err.Error(), "1 real songs") {
		t.Errorf("a real song: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := CheckData(ctx, db); err == nil {
		t.Error("closed database: no error")
	}
}

func TestLoadSongs_refusesABadFixture(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"garbled":   `{"id":`,
		"no year":   `{"id":"fixture-a","artist":"A","title":"T"}`,
		"no prefix": `{"id":"a","artist":"A","title":"T","year":1990}`,
		"twice": "{\"id\":\"fixture-a\",\"artist\":\"A\",\"title\":\"T\",\"year\":1990}\n" +
			"{\"id\":\"fixture-a\",\"artist\":\"A\",\"title\":\"T\",\"year\":1990}",
		"empty": "\n\n",
	} {
		file := filepath.Join(dir, strings.ReplaceAll(name, " ", "-"))
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSongs(ctx, db, file); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := LoadSongs(ctx, db, filepath.Join(dir, "missing")); err == nil {
		t.Error("missing file: no error")
	}
}
