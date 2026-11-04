package musicbrainz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/dancing-queen.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSearch_queryHeadersAndDecode(t *testing.T) {
	body := fixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws/2/recording" || r.Header.Get("User-Agent") != "Reworm/test ( https://github.com/trick77/reworm )" {
			t.Errorf("request %s %v", r.URL.Path, r.Header)
		}
		q := r.URL.Query()
		if q.Get("query") != `recording:"Dancing \"Queen\"" AND artist:"AB\\BA" AND firstreleasedate:[0000 TO 1977-12-31]` ||
			q.Get("fmt") != "json" || q.Get("limit") != "100" {
			t.Errorf("query %v", q)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), UserAgent: "Reworm/test ( https://github.com/trick77/reworm )"}
	recs, err := c.Search(context.Background(), `AB\BA`, `Dancing "Queen"`, 1977)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 5 || recs[1].ID != "a502f559-f70c-4e0d-a5e5-e0cc319d0bd7" || recs[1].Credit != "ABBA" ||
		len(recs[1].Releases) != 6 || recs[1].Releases[1].Group.Title != "Arrival" || recs[1].ISRCs[0] != "SEA017600030" {
		t.Errorf("decoded %+v", recs[1])
	}
}

func TestSearch_pagesPastTheFirstHundred(t *testing.T) {
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		off := r.URL.Query().Get("offset")
		offsets = append(offsets, off)
		_, _ = w.Write([]byte(`{"count":450,"recordings":[{"id":"r` + off + `","title":"T"}]}`))
	}))
	defer srv.Close()
	recs, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).Search(context.Background(), "A", "T", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(offsets, []string{"", "100", "200"}) || len(recs) != 3 || recs[2].ID != "r200" {
		t.Errorf("offsets %q, recs %+v", offsets, recs)
	}
}

func TestSearch_retriesTooFastThenFails(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()
	var waits []time.Duration
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(),
		Sleep: func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }}
	if recs, err := c.Search(context.Background(), "A", "T", 0); err != nil || len(recs) != 0 {
		t.Fatalf("%v, %v", recs, err)
	}
	if !slices.Equal(waits, []time.Duration{2 * time.Second, 4 * time.Second}) {
		t.Errorf("waits %v", waits)
	}

	calls.Store(-100)
	waits = nil
	if _, err := c.Search(context.Background(), "A", "T", 0); err == nil || !strings.Contains(err.Error(), "503 after 6 tries") {
		t.Errorf("err = %v", err)
	}
	c.Sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := c.Search(context.Background(), "A", "T", 0); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestSearch_errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == `recording:"bad" AND artist:"A"` {
			_, _ = w.Write([]byte(`{`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}
	if _, err := c.Search(context.Background(), "A", "T", 0); err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("400: %v", err)
	}
	if _, err := c.Search(context.Background(), "A", "bad", 0); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("bad json: %v", err)
	}
	// A network error is retried like a 503, then reported.
	var tries int
	down := &Client{BaseURL: "http://127.0.0.1:1", Sleep: func(context.Context, time.Duration) error { tries++; return nil }}
	if _, err := down.Search(context.Background(), "A", "T", 0); err == nil || !strings.Contains(err.Error(), "after 6 tries") || tries != 5 {
		t.Errorf("unreachable: %v after %d waits", err, tries)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := down.Search(ctx, "A", "T", 0); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled: %v", err)
	}
	if err := (&Client{}).sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleep %v", err)
	}
	if err := (&Client{}).sleep(context.Background(), time.Millisecond); err != nil {
		t.Errorf("sleep %v", err)
	}
}

func decodeFixture(t *testing.T) []Recording {
	t.Helper()
	body := fixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer srv.Close()
	recs, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).Search(context.Background(), "ABBA", "Dancing Queen", 0)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestPick_earliestExactRecordingAndItsAlbum(t *testing.T) {
	m, ok := Pick(decodeFixture(t), "Abba", "Dancing Queen")
	if !ok {
		t.Fatal("no match")
	}
	if m.Recording.ID != "a502f559-f70c-4e0d-a5e5-e0cc319d0bd7" || !m.Exact || m.Year != 1976 {
		t.Errorf("picked %s exact=%v year=%d", m.Recording.ID, m.Exact, m.Year)
	}
	// Neither the live recording's ISRC nor the DJ mix's comes along.
	if !slices.Equal(m.ISRCs, []string{"SEA017600030", "SEAYD7601020"}) {
		t.Errorf("isrcs %v", m.ISRCs)
	}
	// Arrival: the earliest official studio album, not the earlier single, bootleg or the compilation.
	if m.Original.ID != "rg-arrival" || m.Original.Date != "1976-10-11" || m.Original.Title != "Arrival" {
		t.Errorf("original %+v", m.Original)
	}
	if m.First.ID != "rg-single" || m.First.Date != "1976-08-15" {
		t.Errorf("first %+v", m.First)
	}
	if m.CoverGroup().ID != "rg-arrival" {
		t.Errorf("cover group %+v", m.CoverGroup())
	}
}

// The tribute act shares the title and the name inside its own; it is neither the song nor a source of ISRCs.
func TestPick_revivalBandAndVersionsStayOut(t *testing.T) {
	recs := []Recording{
		{ID: "revival", Title: "Help!", Credit: "Beatles Revival Band", Artists: []string{"Beatles Revival Band"},
			FirstReleaseDate: "1964", ISRCs: []string{"REVIVAL1"}},
		{ID: "studio", Title: "Help!", Credit: "The Beatles", Artists: []string{"The Beatles"},
			FirstReleaseDate: "1965-07-19", ISRCs: []string{"GBAYE0601477"}, Releases: []Release{{}, {}}},
		{ID: "demo", Title: "Help! (demo)", Credit: "The Beatles", Artists: []string{"The Beatles"},
			FirstReleaseDate: "1965-01", ISRCs: []string{"DEMO1"}},
		{ID: "remaster", Title: "Help! - Remastered 2009", Credit: "The Beatles", Artists: []string{"The Beatles"},
			Disambiguation: "2009 stereo version", FirstReleaseDate: "2009", ISRCs: []string{"GBAYE0900001"}},
		{ID: "anthology", Title: "Help!", Credit: "The Beatles", Artists: []string{"The Beatles"},
			Disambiguation: "take 12, live in studio", FirstReleaseDate: "1996", ISRCs: []string{"LIVE1"}},
		{ID: "karaoke", Title: "Help!", Credit: "The Beatles Karaoke Band", Artists: []string{"The Beatles Karaoke Band"},
			FirstReleaseDate: "2001", ISRCs: []string{"KARA1"}},
	}
	m, ok := Pick(recs, "Beatles", "Help!")
	if !ok || m.Recording.ID != "studio" || !m.Exact || m.Year != 1965 {
		t.Fatalf("picked %+v", m.Recording)
	}
	if !slices.Equal(m.ISRCs, []string{"GBAYE0601477"}) {
		t.Errorf("isrcs %v, want the studio one only", m.ISRCs)
	}
	// A card that names the version asks for it.
	if m, _ := Pick(recs, "The Beatles", "Help! (Demo)"); m.Recording.ID != "demo" {
		t.Errorf("demo card picked %s", m.Recording.ID)
	}
	// Only variants left: the song still resolves, to the earliest.
	if m, ok := Pick(recs[2:3], "The Beatles", "Help!"); !ok || m.Recording.ID != "demo" {
		t.Errorf("variants only: %v %s", ok, m.Recording.ID)
	}
}

func TestVersionWords(t *testing.T) {
	for _, tc := range []struct {
		text, card string
		want       bool
	}{
		{"live, 1979-11-05: Wembley Arena", "Dancing Queen", true},
		{"single/LP/12″ version", "Dancing Queen", false},
		{"part of a DJ-mix", "Dancing Queen", true},
		{"Re-Recorded", "Kids in America", true},
		{"Live and Let Die", "Live and Let Die", false},
		{"Layla (Live)", "Layla (Live)", false},
		{"Unplugged", "Layla (Live)", true},
		{"Delivery", "Delivery", false},
	} {
		if got := Version(tc.text, tc.card); got != tc.want {
			t.Errorf("Version(%q, %q) = %v", tc.text, tc.card, got)
		}
	}
	if !variant("Radio Edit", "Song") || variant("Radio Edit", "Song (Radio Edit)") {
		t.Error("variant")
	}
}

func TestSearch_evictsAnUndecodableAnswer(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"recordings":[`))
			return
		}
		_, _ = w.Write([]byte(`{"count":0,"recordings":[]}`))
	}))
	defer srv.Close()
	var evicted []string
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(context.Context, time.Duration) error { return nil },
		Evict: func(method, u string) error { evicted = append(evicted, method+" "+u); return nil }}
	if _, err := c.Search(context.Background(), "A", "T", 0); err != nil {
		t.Fatal(err)
	}
	if len(evicted) != 1 || !strings.HasPrefix(evicted[0], "GET "+srv.URL+"/ws/2/recording?") {
		t.Errorf("evicted %v", evicted)
	}
	calls.Store(0)
	c.Evict = func(string, string) error { return errors.New("disk") }
	if _, err := c.Search(context.Background(), "A", "T", 0); err == nil || !strings.Contains(err.Error(), "evict") {
		t.Errorf("evict failure: %v", err)
	}
}

func TestPick_looseAndNone(t *testing.T) {
	recs := []Recording{
		{ID: "b", Title: "Hips Don't Lie", Credit: "Shakira feat. Wyclef Jean", Artists: []string{"Shakira", "Wyclef Jean"},
			FirstReleaseDate: "2006-02-28", Score: 100, Releases: []Release{{Date: "2006", Group: Group{ID: "single", PrimaryType: "Single"}}}},
		{ID: "a", Title: "Hips Don't Lie (Bamboo)", Credit: "Shakira", Artists: []string{"Shakira"}, FirstReleaseDate: "2006-02-28", Score: 100},
	}
	m, ok := Pick(recs, "Shakira & Wyclef", "Hips Don't Lie")
	if !ok || m.Exact {
		t.Fatalf("ok=%v exact=%v", ok, m.Exact)
	}
	// Same date and score: more releases wins, then the lower MBID.
	if m.Recording.ID != "b" || m.CoverGroup().ID != "single" || m.Original.ID != "" {
		t.Errorf("picked %+v", m)
	}
	recs[0].Releases = nil
	if m, _ := Pick(recs, "Shakira & Wyclef", "Hips Don't Lie"); m.Recording.ID != "a" {
		t.Errorf("tie on MBID: %s", m.Recording.ID)
	}
	if _, ok := Pick(recs, "Someone Else", "Hips Don't Lie"); ok {
		t.Error("other artist matched")
	}
	undated := []Recording{{ID: "x", Title: "T", Credit: "A"}, {ID: "y", Title: "T", Credit: "A", FirstReleaseDate: "1999"}}
	if m, _ := Pick(undated, "A", "T"); m.Recording.ID != "y" || m.Year != 1999 {
		t.Errorf("undated first: %+v", m.Recording)
	}
}

func TestTitleAndArtistHelpers(t *testing.T) {
	for in, want := range map[string]string{
		"Bohemian Rhapsody (Remastered 2011)":  "Bohemian Rhapsody",
		"Waterloo - Single Version [Mono]":     "Waterloo",
		"(I Can't Get No) Satisfaction":        "(I Can't Get No) Satisfaction",
		"(Mono)":                               "(Mono)",
		"Arrival (Deluxe Edition) - Remaster":  "Arrival",
		"Guajira Guantanamera":                 "Guajira Guantanamera",
		"Ein Bett im Kornfeld (Original Mix) ": "Ein Bett im Kornfeld",
	} {
		if got := Strip(in); got != want {
			t.Errorf("Strip(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string][]string{
		"Waterloo": {"Waterloo"},
		"Robin Hood - König der Diebe ((Everything I Do) I Do It For You)": {
			"Robin Hood - König der Diebe ((Everything I Do) I Do It For You)", "Robin Hood",
			"(Everything I Do) I Do It For You"},
		"James Bond 007 - Skyfall":                        {"James Bond 007 - Skyfall", "James Bond 007", "Skyfall"},
		"Bodyguard (I Will Always Love You)":              {"Bodyguard (I Will Always Love You)", "Bodyguard", "I Will Always Love You"},
		"Die Schöne und das Biest - Beauty and the Beast": {"Die Schöne und das Biest - Beauty and the Beast", "Die Schöne und das Biest", "Beauty and the Beast"},
		"(Mono)": {"(Mono)", "Mono"},
		"Odd)":   {"Odd)"},
	} {
		if got := SongTitles(in); !slices.Equal(got, want) {
			t.Errorf("SongTitles(%q) = %q, want %q", in, got, want)
		}
	}
	if !SameTitle("Bohemian Rhapsody - Remastered 2011", "bohemian rhapsody") || SameTitle("Waterloo", "Dancing Queen") {
		t.Error("SameTitle")
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"Shakira feat. Wyclef Jean", "Shakira", true},
		{"Joseíto Fernández", "Joseito Fernandez", true},
		{"The Beatles", "Beatles", true},
		{"ABBA", "Queen", false},
		{"", "Queen", false},
		{"Queen", "Queens of the Stone Age", false},
		{"Abba Revival Band", "ABBA", false},
		{"The Beatles Tribute", "Beatles", false},
	} {
		if got := SameArtist(tc.a, tc.b); got != tc.want {
			t.Errorf("SameArtist(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
	if MainArtist("& Friends") != "& Friends" || MainArtist("Calvin Harris x Dua Lipa") != "Calvin Harris" {
		t.Error("MainArtist")
	}
	if compareDates("1976", "1976-08-15") >= 0 || compareDates("", "1976") <= 0 || compareDates("1976", "") >= 0 {
		t.Error("compareDates")
	}
	if Year("19") != 0 || Year("1976-10") != 1976 {
		t.Error("Year")
	}
}
