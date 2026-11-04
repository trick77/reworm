package deck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/store"
)

// testRepo is a Repo on a fresh database holding a few songs.
func testRepo(t *testing.T) *Repo {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`INSERT INTO songs (id, artist, title, year, year_source, cover_url, sets, categories, genres, apple_id, storefront)
		 VALUES ('dq', 'ABBA', 'Dancing Queen', 1976, 'card', 'https://img/{w}x{h}bb.jpg', '["essentials","mega-mix"]', '["number-ones"]', '["pop","disco-funk"]', '1', 'ch')`,
		`INSERT INTO songs (id, artist, title, year, year_source, sets, genres) VALUES ('br', 'Queen', 'Bohemian Rhapsody', 1975, 'card', '["essentials"]', '["rock"]')`,
		`INSERT INTO songs (id, artist, title, year, year_source, sets, categories) VALUES ('gu', 'Joseíto Fernández', 'Guantanamera', 1929, 'card', '["radio-gold"]', '["evergreens"]')`,
		`INSERT INTO songs (id, artist, title, year, year_source, sets, genres) VALUES ('ww', 'Oasis', 'Wonderwall', 1995, 'card', '["new-set"]', '["rock"]')`,
		`INSERT INTO songs (id, artist, title, year, year_source, sets, active) VALUES ('old', 'Gone', 'Gone', 1980, 'card', '["essentials"]', 0)`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return New(db)
}

func ids(songs []Song) []string {
	out := make([]string, len(songs))
	for i, s := range songs {
		out[i] = s.ID
	}
	return out
}

func TestSelect(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	cases := []struct {
		sel  Selector
		want []string
	}{
		{Selector{}, []string{"br", "dq", "gu", "ww"}},
		{Selector{Sets: []string{"essentials"}}, []string{"br", "dq"}},
		{Selector{Sets: []string{"radio-gold"}, Genres: []string{"rock"}}, []string{"br", "gu", "ww"}},
		{Selector{Categories: []string{"number-ones"}}, []string{"dq"}},
		{Selector{Genres: []string{"rock"}, From: 1990}, []string{"ww"}},
		{Selector{Sets: []string{"nothing"}}, nil},
		// General is every song: whatever else is picked beside it, in the year range.
		{Selector{Sets: []string{"nothing"}, Genres: []string{"rock", General}, From: 1970}, []string{"br", "dq", "ww"}},
	}
	for _, c := range cases {
		if err := c.sel.Normalize(2026); err != nil {
			t.Fatal(err)
		}
		got, err := r.Select(ctx, c.sel)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), c.want) {
			t.Errorf("Select(%+v) = %v, want %v", c.sel, ids(got), c.want)
		}
	}
}

func TestSelector_Normalize(t *testing.T) {
	s := Selector{}
	if err := s.Normalize(2026); err != nil || s.From != 1900 || s.To != 2026 || s.Sets == nil {
		t.Errorf("defaults = %+v, %v", s, err)
	}
	if err := (&Selector{From: 2000, To: 1990}).Normalize(2026); err == nil {
		t.Error("empty range accepted")
	}
	if err := (&Selector{Sets: make([]string, 101)}).Normalize(2026); err == nil {
		t.Error("101 ids accepted")
	}
}

func TestGetAndReveal(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	s, err := r.Get(ctx, "dq")
	if err != nil {
		t.Fatal(err)
	}
	if s.Year != 1976 || s.AppleID != "1" || s.Storefront != "ch" || !s.HasCover {
		t.Errorf("song = %+v", s)
	}
	rv := s.Reveal()
	if rv.Cover == nil || *rv.Cover != "/api/covers/dq" {
		t.Errorf("cover = %v", rv.Cover)
	}
	if b, _ := r.Get(ctx, "br"); b.Reveal().Cover != nil {
		t.Error("song without a cover has a cover URL")
	}
	if _, err := r.Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown = %v", err)
	}
	if u, err := r.CoverURL(ctx, "dq"); err != nil || u != "https://img/{w}x{h}bb.jpg" {
		t.Errorf("CoverURL = %q, %v", u, err)
	}
	if u, err := r.CoverURL(ctx, "br"); err != nil || u != "" {
		t.Errorf("CoverURL without = %q, %v", u, err)
	}
	if _, err := r.CoverURL(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("CoverURL unknown = %v", err)
	}
}

func TestDecksHandler(t *testing.T) {
	r := testRepo(t)
	mux := http.NewServeMux()
	r.Mount(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/decks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var d Decks
	if err := json.NewDecoder(rec.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	wantSets := []Set{
		{ID: "essentials", Name: "Essentials", Count: 2, From: 1975, To: 1976},
		{ID: "mega-mix", Name: "Mega Mix", Count: 1, From: 1976, To: 1976},
		{ID: "radio-gold", Name: "Radio Gold", Count: 1, From: 1929, To: 1929},
		{ID: "new-set", Name: "New Set", Count: 1, From: 1995, To: 1995},
	}
	if !slices.Equal(d.Sets, wantSets) {
		t.Errorf("sets = %+v", d.Sets)
	}
	if !slices.Equal(d.Categories, []Group{{"number-ones", "Number ones", 1}, {"evergreens", "Evergreens (pre-1950)", 1}}) {
		t.Errorf("categories = %+v", d.Categories)
	}
	if !slices.Equal(d.Genres, []Group{{"general", "General", 4}, {"pop", "Pop", 1}, {"rock", "Rock", 2}, {"disco-funk", "Disco & Funk", 1}}) {
		t.Errorf("genres = %+v", d.Genres)
	}
}

func TestDecksHandler_dbError(t *testing.T) {
	r := testRepo(t)
	_ = r.db.Close()
	rec := httptest.NewRecorder()
	r.handleDecks(rec, httptest.NewRequest(http.MethodGet, "/api/decks", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestPlayedHistory(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	if _, err := r.db.ExecContext(ctx, `INSERT INTO profiles (id, name, avatar, created_at) VALUES ('p', 'Jan', 'vinyl', 1)`); err != nil {
		t.Fatal(err)
	}
	now := time.UnixMilli(1_700_000_000_000)
	for _, p := range []struct {
		id string
		at time.Time
	}{{"dq", now.Add(-time.Hour)}, {"dq", now}, {"br", now.Add(-60 * 24 * time.Hour)}} {
		if err := r.RecordPlayed(ctx, "p", p.id, p.at); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.Played(ctx, "p", now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["dq"].Equal(now) {
		t.Errorf("played = %v", got)
	}
}
