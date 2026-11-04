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
	m, ok := Pick(decodeFixture(t), "Abba", "Dancing Queen", 1976)
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
	m, ok := Pick(recs, "Beatles", "Help!", 0)
	if !ok || m.Recording.ID != "studio" || !m.Exact || m.Year != 1965 {
		t.Fatalf("picked %+v", m.Recording)
	}
	if !slices.Equal(m.ISRCs, []string{"GBAYE0601477"}) {
		t.Errorf("isrcs %v, want the studio one only", m.ISRCs)
	}
	// A card that names the version asks for it.
	if m, _ := Pick(recs, "The Beatles", "Help! (Demo)", 0); m.Recording.ID != "demo" {
		t.Errorf("demo card picked %s", m.Recording.ID)
	}
	// Only variants left: the song still resolves, to the earliest.
	if m, ok := Pick(recs[2:3], "The Beatles", "Help!", 0); !ok || m.Recording.ID != "demo" {
		t.Errorf("variants only: %v %s", ok, m.Recording.ID)
	}
}

// A bootleg's earlier date or a misdated entry does not beat the official recording of the card's time; a
// soundtrack counts as the original album.
func TestPick_officialAndCardTimePreferred(t *testing.T) {
	official := func(rg Group, date string) []Release {
		return []Release{{Status: "Official", Date: date, Group: rg}}
	}
	recs := []Recording{
		{ID: "bootleg", Title: "Let It Be", Credit: "The Beatles", FirstReleaseDate: "1969",
			Releases: []Release{{Status: "Bootleg", Date: "1969", Group: Group{ID: "kum-back", PrimaryType: "Album", SecondaryTypes: []string{"Compilation"}}}}},
		{ID: "misdated", Title: "Let It Be", Credit: "The Beatles", FirstReleaseDate: "1958",
			Releases: official(Group{ID: "odd", PrimaryType: "Album"}, "1958")},
		{ID: "studio", Title: "Let It Be", Credit: "The Beatles", FirstReleaseDate: "1970-03-06",
			Releases: official(Group{ID: "help", Title: "Let It Be", PrimaryType: "Album", SecondaryTypes: []string{"Soundtrack"}}, "1970-05-08")},
	}
	m, ok := Pick(recs, "The Beatles", "Let It Be", 1970)
	if !ok || m.Recording.ID != "studio" || m.Original.ID != "help" {
		t.Errorf("picked %s, original %+v", m.Recording.ID, m.Original)
	}
	// Without a card year the earliest official one wins; with only a bootleg left, the bootleg.
	if m, _ := Pick(recs, "The Beatles", "Let It Be", 0); m.Recording.ID != "misdated" {
		t.Errorf("no card year: %s", m.Recording.ID)
	}
	if m, _ := Pick(recs[:1], "The Beatles", "Let It Be", 1970); m.Recording.ID != "bootleg" {
		t.Errorf("bootleg only: %s", m.Recording.ID)
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
			FirstReleaseDate: "2006-02-28", Score: 100, ISRCs: []string{"USSM10603618"},
			Releases: []Release{{Date: "2006", Group: Group{ID: "single", PrimaryType: "Single"}}}},
		{ID: "a", Title: "Hips Don't Lie (Bamboo)", Credit: "Shakira & Wyclef Jean", Artists: []string{"Shakira", "Wyclef Jean"},
			FirstReleaseDate: "2006-02-28", Score: 100},
		// One credited artist: the card's "Shakira & Wyclef" is not split on its own, so Shakira alone is no match.
		{ID: "solo", Title: "Hips Don't Lie", Credit: "Shakira", Artists: []string{"Shakira"}, FirstReleaseDate: "2005", Score: 100},
	}
	m, ok := Pick(recs, "Shakira & Wyclef", "Hips Don't Lie", 0)
	if !ok || m.Exact {
		t.Fatalf("ok=%v exact=%v", ok, m.Exact)
	}
	// Same date and score: more releases wins, then the lower MBID.
	if m.Recording.ID != "b" || m.CoverGroup().ID != "single" || m.Original.ID != "" {
		t.Errorf("picked %+v", m)
	}
	// The chosen recording's ISRCs always count, loosely credited or not.
	if !slices.Equal(m.ISRCs, []string{"USSM10603618"}) {
		t.Errorf("isrcs %v", m.ISRCs)
	}
	recs[0].Releases = nil
	if m, _ := Pick(recs, "Shakira & Wyclef", "Hips Don't Lie", 0); m.Recording.ID != "a" {
		t.Errorf("tie on MBID: %s", m.Recording.ID)
	}
	if _, ok := Pick(recs, "Someone Else", "Hips Don't Lie", 0); ok {
		t.Error("other artist matched")
	}
	undated := []Recording{{ID: "x", Title: "T", Credit: "A"}, {ID: "y", Title: "T", Credit: "A", FirstReleaseDate: "1999"}}
	if m, _ := Pick(undated, "A", "T", 0); m.Recording.ID != "y" || m.Year != 1999 {
		t.Errorf("undated first: %+v", m.Recording)
	}
}

// The card's artist with a backing band is the song, loosely: Ian Dury & the Blockheads for Ian Dury. A part of a
// band name is not: Simon is not Simon & Garfunkel, and Yusuf is the act of "Yusuf / Cat Stevens".
func TestPick_backingBandAliasesAndBandNames(t *testing.T) {
	recs := []Recording{{ID: "dury", Title: "Hit Me With Your Rhythm Stick", Credit: "Ian Dury & the Blockheads",
		Artists: []string{"Ian Dury & the Blockheads"}, SortNames: []string{"Dury, Ian & The Blockheads"}, FirstReleaseDate: "1978-11-23"}}
	if m, ok := Pick(recs, "Ian Dury", "Hit Me with Your Rhythm Stick", 1978); !ok || m.Exact || m.Recording.ID != "dury" {
		t.Errorf("ok=%v %+v", ok, m)
	}
	simon := []Recording{{ID: "paul", Title: "Mrs. Robinson", Credit: "Simon", Artists: []string{"Simon"}}}
	if _, ok := Pick(simon, "Simon & Garfunkel", "Mrs. Robinson", 0); ok {
		t.Error("Simon matched Simon & Garfunkel")
	}
	// A band MusicBrainz knows as one act, named for the card's artist.
	harley := []Recording{{ID: "cr", Title: "Make Me Smile (Come Up and See Me)", Credit: "Steve Harley & Cockney Rebel",
		Artists: []string{"Steve Harley & Cockney Rebel"}, SortNames: []string{"Harley, Steve & Cockney Rebel"}, FirstReleaseDate: "1975"}}
	if m, ok := Pick(harley, "Steve Harley", "Make Me Smile (Come up and See Me)", 1975); !ok || m.Exact || m.Recording.ID != "cr" {
		t.Errorf("Steve Harley: ok=%v %+v", ok, m)
	}
	// The band's article sorted last is no person: "Dekker, Desmond & Aces, The".
	aces := []Recording{{ID: "da", Title: "Israelites", Credit: "Desmond Dekker & the Aces", Artists: []string{"Desmond Dekker & the Aces"},
		SortNames: []string{"Dekker, Desmond & Aces, The"}, FirstReleaseDate: "1968"}}
	if m, ok := Pick(aces, "Desmond Dekker", "Israelites", 1968); !ok || m.Recording.ID != "da" {
		t.Errorf("Desmond Dekker: ok=%v %+v", ok, m)
	}
	// Not a band of the card's artist: a duo of two people, a duo of two names, an orchestra of its own, a group
	// MusicBrainz sent no sort name for.
	for _, tc := range []struct{ card, title, group, sort string }{
		{"Daryl Hall", "Rich Girl", "Daryl Hall & John Oates", "Hall, Daryl & Oates, John"},
		{"Simon", "Mrs. Robinson", "Simon & Garfunkel", "Simon & Garfunkel"},
		{"Elvis Presley", "Love Me Tender", "Elvis Presley & The Royal Philharmonic Orchestra", "Presley, Elvis & The Royal Philharmonic Orchestra"},
		{"Steve Harley", "Make Me Smile", "Steve Harley & Cockney Rebel", ""},
	} {
		recs := []Recording{{ID: "g", Title: tc.title, Credit: tc.group, Artists: []string{tc.group}, SortNames: []string{tc.sort}}}
		if _, ok := Pick(recs, tc.card, tc.title, 0); ok {
			t.Errorf("%s taken for %s", tc.group, tc.card)
		}
	}
	plus := []Recording{{ID: "ql", Title: "Bohemian Rhapsody", Credit: "Queen + Adam Lambert", Artists: []string{"Queen + Adam Lambert"}}}
	if _, ok := Pick(plus, "Queen", "Bohemian Rhapsody", 1975); ok {
		t.Error("Queen + Adam Lambert matched Queen")
	}
	yusuf := []Recording{{ID: "fs", Title: "Father and Son", Credit: "Yusuf", Artists: []string{"Yusuf"}, FirstReleaseDate: "1970"}}
	if m, ok := Pick(yusuf, "Yusuf / Cat Stevens", "Father And Son", 1970); !ok || m.Recording.ID != "fs" {
		t.Errorf("Yusuf: ok=%v %+v", ok, m)
	}
}

// The original album is the earliest studio album of the song's recordings: here a sibling's, not the 1993
// soundtrack the chosen recording is on.
func TestPick_originalAlbumFromASiblingRecording(t *testing.T) {
	rel := func(group, typ, date string, secondary ...string) Release {
		return Release{Status: "Official", Date: date, Group: Group{ID: group, Title: group, PrimaryType: typ, SecondaryTypes: secondary}}
	}
	recs := []Recording{
		{ID: "single", Title: "Nights in White Satin", Credit: "The Moody Blues", Artists: []string{"The Moody Blues"}, FirstReleaseDate: "1967-11",
			Releases: []Release{rel("Single", "Single", "1967-11"), rel("A Bronx Tale", "Album", "1993", "Soundtrack")}},
		{ID: "album", Title: "Nights in White Satin", Credit: "The Moody Blues", Artists: []string{"The Moody Blues"}, FirstReleaseDate: "1967-11-10",
			Releases: []Release{rel("Days of Future Passed", "Album", "1967-11-10")}},
	}
	m, ok := Pick(recs, "The Moody Blues", "Nights In White Satin", 1967)
	if !ok || m.Recording.ID != "single" || m.Original.ID != "Days of Future Passed" || m.First.ID != "Single" {
		t.Errorf("ok=%v id=%s original=%s first=%s", ok, m.Recording.ID, m.Original.ID, m.First.ID)
	}
	// A sibling's album from outside the card's time is none: the chosen recording's own stays.
	recs[1].Releases = []Release{rel("Days of Future Passed (2017 Reissue)", "Album", "2017")}
	if m, _ := Pick(recs, "The Moody Blues", "Nights In White Satin", 1967); m.Original.ID != "A Bronx Tale" {
		t.Errorf("late sibling album: original=%s", m.Original.ID)
	}
	// Nor a sibling that is a re-recording.
	recs[1].Title, recs[1].Releases = "Nights in White Satin (re-recorded)", []Release{rel("Days of Future Passed", "Album", "1967-11-10")}
	if m, _ := Pick(recs, "The Moody Blues", "Nights In White Satin", 1967); m.Original.ID != "A Bronx Tale" {
		t.Errorf("re-recorded sibling: original=%s", m.Original.ID)
	}
}

func TestCredit(t *testing.T) {
	rec := func(credit string, artists ...string) Recording { return Recording{Credit: credit, Artists: artists} }
	queenBowie := rec("Queen & David Bowie", "Queen", "David Bowie")
	emotions := rec("Earth, Wind & Fire with The Emotions", "Earth, Wind & Fire", "The Emotions")
	for _, tc := range []struct {
		name, card, title string
		rec               Recording
		by, led           bool
	}{
		// Several artists, MusicBrainz's split: all of them in any order and joining, or led by the main one.
		{"Queen & David Bowie", "Queen, David Bowie", "", queenBowie, true, true},
		{"David Bowie & Queen", "Queen, David Bowie", "", queenBowie, true, true},
		{"Queen", "Queen, David Bowie", "", queenBowie, true, true},
		{"David Bowie", "Queen, David Bowie", "", queenBowie, false, false},
		{"Mark Ronson", "Mark Ronson, Amy Winehouse", "", rec("Mark Ronson feat. Amy Winehouse", "Mark Ronson", "Amy Winehouse"), true, true},
		{"John Travolta & Olivia Newton-John", "John Travolta, Olivia Newton-John", "",
			rec("John Travolta, Olivia Newton‐John & Cast", "John Travolta", "Olivia Newton‐John", "Cast"), true, true},
		{"Ed Sheeran & Elton John", "Ed Sheeran", "", rec("Ed Sheeran & Elton John", "Ed Sheeran", "Elton John"), true, true},
		{"Martin Solveig & Dragonette", "Martin Solveig", "", rec("Martin Solveig feat. Dragonette", "Martin Solveig", "Dragonette"), true, true},
		// A guest the card's title names.
		{"Calvin Harris, Dua Lipa", "Calvin Harris", "One Kiss (with Dua Lipa)", rec("Calvin Harris", "Calvin Harris"), true, true},
		// The band whole, its guest or not; never a part of it.
		{"Earth, Wind & Fire", "Earth, Wind & Fire", "", emotions, true, true},
		{"Earth, Wind & Fire & The Emotions", "Earth, Wind & Fire", "", emotions, true, true},
		{"Earth", "Earth, Wind & Fire", "", emotions, false, false},
		{"The Emotions", "Earth, Wind & Fire", "", emotions, false, false},
		{"Earth", "Earth, Wind & Fire", "", rec("Earth, Wind & Fire", "Earth, Wind & Fire"), false, false},
		{"Simon", "Simon and Garfunkel", "", rec("Simon & Garfunkel", "Simon & Garfunkel"), false, false},
		{"Simon", "Simon & Garfunkel", "", Recording{}, false, false},
		{"AC", "AC/DC", "", rec("AC/DC", "AC/DC"), false, false},
		{"Florence", "Florence + the Machine", "", rec("Florence + the Machine", "Florence + the Machine"), false, false},
		{"Malcolm", "Malcolm X", "", Recording{}, false, false},
		{"Ricchi & Poveri", "Ricchi E Poveri", "", rec("Ricchi e Poveri", "Ricchi e Poveri"), true, true},
		// One act under its names.
		{"Cat Stevens", "Yusuf / Cat Stevens", "", rec("Yusuf", "Yusuf"), true, true},
		{"Yusuf / Cat Stevens", "Yusuf / Cat Stevens", "", rec("Yusuf", "Yusuf"), true, true},
		// An act the song does not name is not its artist; by ISRC, led by the main artist, it may be.
		{"Queen + Adam Lambert", "Queen", "", rec("Queen", "Queen"), false, false},
		{"Queen + Adam Lambert", "Queen", "", Recording{}, false, false},
		// A featuring mark ("feat.", "x") keeps its main artist the act (MainArtist), as for Calvin Harris x Dua Lipa.
		{"Queen feat. Adam Lambert", "Queen", "", rec("Queen", "Queen"), true, true},
		{"Nino Rota & Carlo Savina", "Nino Rota", "", rec("Nino Rota", "Nino Rota"), false, true},
		{"Nino Rota, Carlo Savina", "Nino Rota", "", rec("Nino Rota", "Nino Rota"), false, true},
		{"Elvis Presley & The Royal Philharmonic Orchestra", "Elvis Presley", "", rec("Elvis Presley", "Elvis Presley"), false, true},
		{"AC/DC", "AC/DC", "", Recording{}, true, true},
		{"DC/AC", "AC/DC", "", Recording{}, false, false},
		{"Adam Lambert & Queen", "Queen", "", rec("Queen", "Queen"), false, false},
		// A backing band.
		{"Glenn Miller and His Orchestra", "Glenn Miller", "", rec("Glenn Miller", "Glenn Miller"), true, true},
		{"Ian Dury & The Blockheads", "Ian Dury", "", rec("Ian Dury & the Blockheads", "Ian Dury & the Blockheads"), true, true},
		// A card MusicBrainz does not know: all its names, never a part.
		{"Queen & David Bowie", "Queen, David Bowie", "", Recording{}, true, true},
		{"Queen", "Queen, David Bowie", "", Recording{}, false, false},
		{"Earth", "Earth, Wind & Fire", "", Recording{}, false, false},
		{"Earth, Wind & Fire", "Earth Wind and Fire", "", Recording{}, true, true},
		// Other acts.
		{"Beatles Revival Band", "The Beatles", "", rec("The Beatles", "The Beatles"), false, false},
		{"Other Act", "Queen, David Bowie", "", queenBowie, false, false},
		{"", "Queen", "", Recording{}, false, false},
	} {
		c := NewCredit(tc.card, tc.title, tc.rec)
		if got := c.By(tc.name); got != tc.by {
			t.Errorf("By(%q) for %q (%q) = %v", tc.name, tc.card, tc.rec.Credit, got)
		}
		if got := c.Led(tc.name); got != tc.led {
			t.Errorf("Led(%q) for %q (%q) = %v", tc.name, tc.card, tc.rec.Credit, got)
		}
	}
	for _, tc := range []struct {
		card   string
		rec    Recording
		search string
	}{
		{"Queen, David Bowie", queenBowie, "Queen"},
		{"Queen, David Bowie", Recording{}, "Queen, David Bowie"},
		{"Earth, Wind & Fire", emotions, "Earth, Wind & Fire"},
		{"Earth, Wind & Fire", rec("Earth, Wind & Fire", "Earth, Wind & Fire"), "Earth, Wind & Fire"},
		{"Shakira feat. Wyclef Jean", rec("Shakira feat. Wyclef Jean", "Shakira", "Wyclef Jean"), "Shakira"},
		{"Calvin Harris (with Ellie Goulding)", Recording{}, "Calvin Harris"},
		{"Yusuf / Cat Stevens", rec("Yusuf", "Yusuf"), "Yusuf / Cat Stevens"},
		// The card names one of the artists: as written, not the orchestra credited first.
		{"Michael Nyman", rec("Münchner Philharmoniker, John Harle, Michael Nyman", "Münchner Philharmoniker", "John Harle", "Michael Nyman"), "Michael Nyman"},
		{"Ed Sheeran", rec("Ed Sheeran & Elton John", "Ed Sheeran", "Elton John"), "Ed Sheeran"},
	} {
		if got := NewCredit(tc.card, "", tc.rec).Search(); got != tc.search {
			t.Errorf("Search for %q (%q) = %q, want %q", tc.card, tc.rec.Credit, got, tc.search)
		}
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
	for in, want := range map[string]string{
		"Heroes - 2017 Remaster":                   "Heroes",
		"Surfin' U.S.A. (Mono)":                    "Surfin' U.S.A.",
		"You're The One That I Want - From Grease": "You're The One That I Want",
		"Mr. Saxobeat - Radio Edit":                "Mr. Saxobeat",
		"Imagine - Remastered 2010":                "Imagine",
		"Waterloo - Single Version [Mono]":         "Waterloo",
		"Bodyguard (I Will Always Love You)":       "Bodyguard (I Will Always Love You)",
		"Layla (Live)":                             "Layla (Live)",
		"(Remastered)":                             "(Remastered)",
		"Summer of '69":                            "Summer of '69",
		"1999":                                     "1999",
	} {
		if got := CleanTitle(in); got != want {
			t.Errorf("CleanTitle(%q) = %q, want %q", in, got, want)
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
		// A featuring note is no song inside the title.
		"Without You (feat. Sandro Cavazza)":   {"Without You (feat. Sandro Cavazza)", "Without You"},
		"Grease (Summer Nights)":               {"Grease (Summer Nights)", "Grease", "Summer Nights"},
		"Der Pate (The Godfather Waltz)":       {"Der Pate (The Godfather Waltz)", "Der Pate", "The Godfather Waltz"},
		"Marry Poppins (Supercalifragilistic)": {"Marry Poppins (Supercalifragilistic)", "Marry Poppins", "Supercalifragilistic"},
	} {
		if got := SongTitles(in); !slices.Equal(got, want) {
			t.Errorf("SongTitles(%q) = %q, want %q", in, got, want)
		}
	}
	// "(with X)" is a guest note when X is one of the artists, else a title.
	for _, tc := range []struct {
		title   string
		artists []string
		want    []string
	}{
		{"One Kiss (with Dua Lipa)", []string{"Calvin Harris", "Calvin Harris & Dua Lipa"}, []string{"One Kiss (with Dua Lipa)", "One Kiss"}},
		{"Miracle (with Ellie Goulding)", []string{"Calvin Harris (with Ellie Goulding)"}, []string{"Miracle (with Ellie Goulding)", "Miracle"}},
		{"One Kiss (with Dua Lipa)", []string{"Calvin Harris"}, []string{"One Kiss (with Dua Lipa)", "One Kiss", "with Dua Lipa"}},
		{"Wunderbare Jahre (With a Little Help from My Friends)", []string{"Joe Cocker"},
			[]string{"Wunderbare Jahre (With a Little Help from My Friends)", "Wunderbare Jahre", "With a Little Help from My Friends"}},
		{"U2 (With or Without You)", []string{"U2"}, []string{"U2 (With or Without You)", "U2", "With or Without You"}},
	} {
		if got := SongTitles(tc.title, tc.artists...); !slices.Equal(got, tc.want) {
			t.Errorf("SongTitles(%q, %q) = %q, want %q", tc.title, tc.artists, got, tc.want)
		}
	}
	if !SameTitle("Bohemian Rhapsody - Remastered 2011", "bohemian rhapsody") || SameTitle("Waterloo", "Dancing Queen") {
		t.Error("SameTitle")
	}
	// A superscript is a digit: the 2020 re-recording album is not the 1970 one; a remaster is.
	if SameTitle("Tea for the Tillerman²", "Tea for the Tillerman") || !SameTitle("Tea for the Tillerman (2020 Remaster)", "Tea for the Tillerman") {
		t.Error("SameTitle Tillerman²")
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"Shakira feat. Wyclef Jean", "Shakira", true},
		{"Calvin Harris (with Ellie Goulding)", "Calvin Harris", true},
		{"Calvin Harris x Dua Lipa", "Calvin Harris", true},
		{"Simon & Garfunkel", "Simon", false},
		{"Simon and Garfunkel", "Simon", false},
		{"Earth, Wind & Fire", "Earth", false},
		{"Florence + the Machine", "Florence", false},
		{"AC/DC", "AC", false},
		{"Malcolm X", "Malcolm", false},
		{"Joseíto Fernández", "Joseito Fernandez", true},
		{"The Beatles", "Beatles", true},
		{"ABBA", "Queen", false},
		{"", "Queen", false},
		{"Queen", "Queens of the Stone Age", false},
		{"Abba Revival Band", "ABBA", false},
		{"The Beatles Tribute", "Beatles", false},
		// The same artists, joined another way or in another order.
		{"Queen, David Bowie", "Queen & David Bowie", true},
		{"John Travolta, Olivia Newton-John", "John Travolta & Olivia Newton-John", true},
		{"Mark Forster, LEA", "Mark Forster & LEA", true},
		{"Ricchi E Poveri", "Ricchi & Poveri", true},
		{"Imany,Filatov & Karas", "Imany, Filatov and Karas", true},
		{"Helga Feddersen & Dieter Hallervorden", "Dieter Hallervorden, Helga Feddersen", true},
		{"Earth, Wind & Fire", "Earth Wind and Fire", true},
		{"Simon and Garfunkel", "Simon & Garfunkel", true},
		// One of them is not the set.
		{"Queen, David Bowie", "Queen", false},
		{"Queen", "Queen & David Bowie", false},
		{"Simon & Garfunkel", "Simon & Somebody", false},
		// An unspaced slash is in the name.
		{"AC/DC", "DC/AC", false},
		{"AC/DC", "AC/DC", true},
	} {
		if got := SameArtist(tc.a, tc.b); got != tc.want {
			t.Errorf("SameArtist(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
	for in, want := range map[string]string{
		"feat. Friends": "feat. Friends", "Calvin Harris x Dua Lipa": "Calvin Harris", "AC/DC": "AC/DC",
		"Earth, Wind & Fire": "Earth, Wind & Fire", "Simon and Garfunkel": "Simon and Garfunkel",
		"Florence + the Machine": "Florence + the Machine", "Malcolm X": "Malcolm X",
		"Shakira (feat. Wyclef Jean)": "Shakira", "Eminem ft. Rihanna": "Eminem", "Tiësto vs. Diplo": "Tiësto",
		"Feather Band feat. X": "Feather Band", "MEDUZA х Becky Hill": "MEDUZA",
	} {
		if got := MainArtist(in); got != want {
			t.Errorf("MainArtist(%q) = %q, want %q", in, got, want)
		}
	}
	if MainArtist("& Friends") != "& Friends" {
		t.Error("MainArtist")
	}
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"Queen & David Bowie", []string{"Queen", "David Bowie"}},
		{"Imany,Filatov & Karas", []string{"Imany", "Filatov", "Karas"}},
		{"Robin Schulz,Lilly Wood and The Prick", []string{"Robin Schulz", "Lilly Wood", "The Prick"}},
		{"Calvin Harris (with Ellie Goulding)", []string{"Calvin Harris", "Ellie Goulding"}},
		{"Pras (feat. Ol' Dirty Bastard & Mýa)", []string{"Pras", "Ol' Dirty Bastard", "Mýa"}},
		{"Eartha Kitt with Henri René", []string{"Eartha Kitt", "Henri René"}},
		{"Udo Lindenberg und das Panikorchester", []string{"Udo Lindenberg", "das Panikorchester"}},
		{"MEDUZA х Becky Hill х Goodboys", []string{"MEDUZA", "Becky Hill", "Goodboys"}},
		{"Tiësto vs. Diplo", []string{"Tiësto", "Diplo"}},
		{"A featuring B", []string{"A", "B"}},
		{"Feathers & Gold", []string{"Feathers", "Gold"}},
		{"Yusuf / Cat Stevens", []string{"Yusuf", "Cat Stevens"}},
		// Band names split too: a part alone says nothing (Credit).
		{"AC/DC", []string{"AC/DC"}},
		{"Florence + the Machine", []string{"Florence", "the Machine"}},
		{"EAV (Erste Allgemeine Verunsicherung)", []string{"EAV (Erste Allgemeine Verunsicherung)"}},
		{"Malcolm X", []string{"Malcolm X"}},
		{"E Nomine", []string{"E Nomine"}},
		{"&", []string{"&"}},
	} {
		if got := Artists(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("Artists(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if Lead("Nino Rota & Carlo Savina") != "Nino Rota" || Lead("Queen") != "Queen" {
		t.Error("Lead")
	}
	if compareDates("1976", "1976-08-15") >= 0 || compareDates("", "1976") <= 0 || compareDates("1976", "") >= 0 {
		t.Error("compareDates")
	}
	if Year("19") != 0 || Year("1976-10") != 1976 {
		t.Error("Year")
	}
}
