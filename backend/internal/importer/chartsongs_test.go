package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trick77/toolkit/applemusic"

	"github.com/trick77/reworm/internal/cards"
	"github.com/trick77/reworm/internal/charts"
	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/lastfm"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

// wikis serves the chart pages of 2024; a page of 2023 lists nothing.
func wikis(t *testing.T) *httptest.Server {
	t.Helper()
	row := func(title, artist string) string {
		return "{{Nummer-eins-Hits Zeile\n| Titel = " + title + "\n| Interpret = " + artist + "\n}}\n"
	}
	pages := map[string]string{
		"Liste der Nummer-eins-Hits in der Schweiz (2024)": "== Singles ==\n" + row("[[Fresh Song]]", "[[Fresh Act]]") +
			row("New Song", "Nobody") + "== Alben ==\n",
		"Liste der Nummer-eins-Hits in Deutschland (2024)": "== Singles ==\n" + row("Dancing Queen", "[[ABBA]]"),
		"Liste der Nummer-eins-Hits in Österreich (2024)":  "== Singles ==\n" + row("Fresh Song", "Fresh Act"),
		"Billboard Year-End Hot 100 singles of 2024": "{| class=\"wikitable\"\n|-\n| scope=\"row\" | 1\n" +
			"| \"[[Bohemian Rhapsody]]\" || [[Queen (band)|Queen]] featuring Nobody\n|}\n",
		"List of UK top-ten singles in 2024": "==Top-ten singles==\n{| class=\"wikitable\"\n|-\n| 3\n" +
			"| align=\"left\"|\"[[Fresh Song]]\"\n| align=\"left\"|Fresh Act\n| 1\n|}\n",
		"Liste der Nummer-eins-Hits in der Schweiz (2023)": "== Singles ==\n",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Reworm/") {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		text, ok := pages[r.URL.Query().Get("page")]
		if !ok {
			_, _ = fmt.Fprint(w, `{"error":{"code":"missingtitle"}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"parse": map[string]any{"wikitext": text}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestImport_chartsThenSongs(t *testing.T) {
	var mbHits atomic.Int32
	ep := endpoints(services(t, &mbHits, nil))
	wiki := wikis(t)
	ep.DeWiki, ep.EnWiki, ep.WikiEvery = wiki.URL+"/de", wiki.URL+"/en", time.Nanosecond
	data := t.TempDir()
	if code, out := run(t, ep, nil, "-data", data, "-years", "2024", "charts"); code != 0 {
		t.Fatalf("charts: exit %d\n%s", code, out)
	}
	if code, out := run(t, ep, nil, "-data", data, "-years", "2023", "charts"); code != 1 || !strings.Contains(out, "no hits found") {
		t.Errorf("an empty page must fail the step: exit %d\n%s", code, out)
	}
	if code, out := run(t, ep, nil, "-data", data, "-years", "20x", "charts"); code != 1 || !strings.Contains(out, "years") {
		t.Errorf("bad years: exit %d\n%s", code, out)
	}
	if err := cards.WriteSnapshot(filepath.Join(data, "cards"), "mega-mix", []cards.Card{
		{Artist: "Abba", Title: "Dancing Queen!", Year: 1977}, {Artist: "Queen", Title: "Bohemian Rhapsody", Year: 1975}}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"BACKEND_CARDS_FILES": "second.csv=Mega Mix", "BACKEND_LASTFM_API_KEY": "unused"}
	if code, out := run(t, ep, env, "-data", data, "songs"); code != 0 {
		t.Fatalf("songs: exit %d\n%s", code, out)
	}
	b, err := os.ReadFile(filepath.Join(data, "songs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	songs, err := song.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]song.Song{}
	for _, s := range songs {
		byID[s.ID] = s
	}
	// Fresh Song: dated by MusicBrainz, in Mega Mix only (Rock Anthems and Christmas Hits are not configured).
	fresh := byID["fresh"]
	if len(songs) != 3 || fresh.Year != 2024 || fresh.YearSource != song.YearMusicBrainz || fresh.AppleID != "fresh-1" ||
		!slices.Equal(fresh.Sets, []string{"mega-mix"}) || !slices.Equal(fresh.Categories, []string{"gen-alpha"}) {
		t.Fatalf("songs:\n%s", b)
	}
	// The cards win over the charts naming them.
	if br := byID["br-studio"]; br.Year != 1975 || br.YearSource != song.YearCard {
		t.Errorf("Bohemian Rhapsody = %+v", br)
	}
	rep, err := os.ReadFile(filepath.Join(data, "import-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| 2024 | 6 | 1 |", "## Chart songs added (1)", "| Fresh Act | Fresh Song | 2024 | at, ch, uk | mega-mix |",
		"## Chart songs left out", "| Nobody | New Song | ch | MusicBrainz knows no dated recording |"} {
		if !strings.Contains(string(rep), want) {
			t.Errorf("report lacks %q:\n%s", want, rep)
		}
	}
	// Offline, the chart song comes back as stored.
	if code, out := run(t, ep, env, "-data", data, "-offline", "songs"); code != 0 {
		t.Fatalf("offline: exit %d\n%s", code, out)
	}
	b2, err := os.ReadFile(filepath.Join(data, "songs.jsonl"))
	if err != nil || string(b2) != string(b) {
		t.Errorf("offline dataset differs (%v):\n%s", err, b2)
	}
}

// countingCatalog is a fakeCatalog that counts the ISRC lookups.
type countingCatalog struct {
	fakeCatalog
	calls *int
}

func (c countingCatalog) SongsByISRC(ctx context.Context, sf string, isrcs []string) ([]applemusic.Song, error) {
	*c.calls++
	return c.fakeCatalog.SongsByISRC(ctx, sf, isrcs)
}

func TestCollect_chartRowsBesideCards(t *testing.T) {
	sets := []cards.Set{{Slug: "mega-mix", Cards: []cards.Card{{Artist: "Teddy Swims", Title: "Lose Control", Year: 2023}}}}
	rows := []charts.Row{
		{Entry: charts.Entry{Artist: "Teddy Swims", Title: "Lose Control", Source: charts.CH}, Year: 2024},
		{Entry: charts.Entry{Artist: "Benson Boone", Title: "Beautiful Things", Source: charts.Billboard}, Year: 2025},
		{Entry: charts.Entry{Artist: "Benson Boone", Title: "Beautiful Things", Source: charts.CH}, Year: 2024},
		{Entry: charts.Entry{Artist: "Benson Boone", Title: "Beautiful Things", Source: charts.DE}, Year: 2024},
	}
	es, _, st := collect(sets, rows)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	bb, card := es[0], es[1]
	if !bb.Chart || bb.Year != 2024 || !slices.Equal(bb.Years, []int{2024, 2025}) || len(bb.Sets) != 0 ||
		!slices.Equal(bb.Sources, []string{charts.Billboard, charts.CH, charts.DE}) {
		t.Errorf("chart entry %+v", bb)
	}
	if card.Chart || len(card.Sources) != 0 || card.Year != 2023 {
		t.Errorf("card entry %+v", card)
	}
	if *st[2024] != (chartStats{Rows: 3, OnCard: 1}) || *st[2025] != (chartStats{Rows: 1}) {
		t.Errorf("stats %+v %+v", *st[2024], *st[2025])
	}
}

func TestResolveChart_datesByMusicBrainzCheckedByApple(t *testing.T) {
	rec := func(id, date string) musicbrainz.Recording {
		return musicbrainz.Recording{ID: id, Title: "Messy", Credit: "Lola Young", Artists: []string{"Lola Young"},
			FirstReleaseDate: date, ISRCs: []string{"GB" + id}}
	}
	track := func(date string) applemusic.Song {
		return applemusic.Song{ID: "a1", Name: "Messy", Artist: "Lola Young", Album: "This Wasn't Meant For You Anyway",
			ReleaseDate: date, Preview: true}
	}
	e := entry{Artist: "Lola Young", Title: "Messy", Year: 2025, Chart: true, Sources: []string{charts.CH}}
	for _, tc := range []struct {
		name   string
		recs   []musicbrainz.Recording
		apple  []applemusic.Song
		year   int
		drop   string
		lookup bool
	}{
		{"kept", []musicbrainz.Recording{rec("m1", "2024-05-24")}, []applemusic.Song{track("2024-06-14")}, 2024, "", true},
		// The pick prefers no recording by the chart year: the earliest is the song, a recurrent here.
		{"recurrent", []musicbrainz.Recording{rec("m2", "2024-05-24"), rec("m1", "2001")}, nil, 2001,
			"released 2001, before 2023", false},
		{"Apple disagrees", []musicbrainz.Recording{rec("m1", "2024")}, []applemusic.Song{track("2019-01-01")}, 2024,
			"Apple dates it 2019, MusicBrainz 2024", true},
		{"no Apple track", []musicbrainz.Recording{rec("m1", "2024")}, nil, 2024, "no Apple track with a preview", true},
		{"no MusicBrainz", nil, nil, 0, "MusicBrainz knows no dated recording", false},
	} {
		calls := 0
		p := &Pipeline{MB: fakeMB{bounded: tc.recs}, Apple: countingCatalog{fakeCatalog{byISRC: tc.apple}, &calls},
			ChartFrom: 2023}
		r, err := p.resolveChart(context.Background(), e)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if r.Drop != tc.drop || r.Song.Year != tc.year || (calls > 0) != tc.lookup {
			t.Errorf("%s: drop %q year %d Apple asked %d times", tc.name, r.Drop, r.Song.Year, calls)
		}
		if tc.drop == "" && (r.Song.ID != "m1" || r.Song.YearSource != song.YearMusicBrainz || r.Song.AppleID != "a1" ||
			len(r.Song.Sets) != 0 || !r.Chart) {
			t.Errorf("%s: song %+v", tc.name, r.Song)
		}
	}
}

// errMB fails every search.
type errMB struct{ err error }

func (f errMB) Search(context.Context, string, string, int) ([]musicbrainz.Recording, error) {
	return nil, f.err
}

// A MusicBrainz failure leaves a chart song out; offline it is the caller's to answer (the previous dataset).
func TestResolveChart_musicBrainzFailure(t *testing.T) {
	e := entry{Artist: "Taylor Swift", Title: "Is It Over Now?", Year: 2023, Chart: true}
	p := &Pipeline{MB: errMB{errors.New("musicbrainz: status 503 after 6 tries")}}
	r, err := p.resolveChart(context.Background(), e)
	if err != nil || !strings.HasPrefix(r.Drop, "MusicBrainz failed: ") {
		t.Errorf("drop %q, err %v", r.Drop, err)
	}
	p = &Pipeline{MB: errMB{httpcache.ErrOffline}}
	if _, err := p.resolveChart(context.Background(), e); !errors.Is(err, httpcache.ErrOffline) {
		t.Errorf("offline: %v", err)
	}
}

func TestMerge_cardBeatsChartAndChartSpellingsJoin(t *testing.T) {
	card := resolved{entry: entry{Artist: "Lady Gaga & Bruno Mars", Title: "Die with a Smile", Year: 2024, Sets: []string{"mega-mix"}},
		Song: song.Song{ID: "dws", Year: 2024, YearSource: song.YearCard, Sets: []string{"mega-mix"}}}
	bb := resolved{entry: entry{Artist: "Lady Gaga and Bruno Mars", Title: "Die with a Smile", Year: 2025, Chart: true,
		Sources: []string{charts.Billboard}}, Song: song.Song{ID: "dws", Year: 2023, YearSource: song.YearMusicBrainz}}
	ch := bb
	ch.Artist, ch.Sources = "Lady Gaga & Bruno Mars feat. x", []string{charts.CH}
	for _, order := range [][]resolved{{card, bb, ch}, {bb, card, ch}, {bb, ch, card}} {
		out, merged := merge(order)
		if len(out) != 1 || len(merged) != 0 {
			t.Fatalf("out %+v merged %q", out, merged)
		}
		o := out[0]
		if o.Chart || o.Song.Year != 2024 || o.Song.YearSource != song.YearCard || !slices.Equal(o.Song.Sets, []string{"mega-mix"}) ||
			!slices.Equal(o.Sources, []string{charts.Billboard, charts.CH}) {
			t.Errorf("merged %+v", o)
		}
	}
	dropped := bb
	dropped.Drop = "no Apple track with a preview"
	out, _ := merge([]resolved{dropped, ch})
	if len(out) != 1 || out[0].Drop != "" || !slices.Equal(out[0].Sources, []string{charts.Billboard, charts.CH}) {
		t.Errorf("kept beats dropped: %+v", out)
	}
	out, _ = merge([]resolved{ch, dropped})
	if out[0].Drop != "" {
		t.Errorf("a dropped one replaced the kept: %+v", out)
	}
}

func TestWithoutTwins_foldsChartSpellingsOfOneSong(t *testing.T) {
	huntrx := musicbrainz.Match{Recording: musicbrainz.Recording{Artists: []string{"HUNTR/X", "EJAE"}}}
	golden := func(id, artist string, chart bool, src string) resolved {
		r := resolved{entry: entry{Artist: artist, Title: "Golden", Chart: chart}, Song: song.Song{ID: id, Title: "Golden"},
			match: huntrx}
		if src != "" {
			r.Sources = []string{src}
		}
		return r
	}
	out := withoutTwins([]resolved{
		golden("g1", "Huntr/x feat. Ejae", true, charts.CH),
		golden("g2", "Huntr/x (Ejae)", true, charts.DE),
		{entry: entry{Artist: "Other", Title: "Golden", Chart: true}, Song: song.Song{ID: "o", Title: "Golden"}},
	})
	if len(out) != 2 || out[0].Song.ID != "g1" || !slices.Equal(out[0].Sources, []string{charts.CH, charts.DE}) {
		t.Fatalf("chart twins: %+v", out)
	}
	// A card song of the same artist and title is kept, whatever the order.
	out = withoutTwins([]resolved{golden("g1", "Huntr/x feat. Ejae", true, charts.CH), golden("card", "HUNTR/X", false, "")})
	if len(out) != 1 || out[0].Song.ID != "card" || !slices.Equal(out[0].Sources, []string{charts.CH}) {
		t.Fatalf("card twin: %+v", out)
	}
}

func TestPlaceCharts_billboardListenersAndSets(t *testing.T) {
	chart := func(year, listeners int, genres []string, sources ...string) resolved {
		return resolved{entry: entry{Artist: "A", Title: "T", Chart: true, Sources: sources},
			Song: song.Song{Year: year, Listeners: listeners, Genres: genres}}
	}
	all := func(string) bool { return true }
	rs := []resolved{
		{Song: song.Song{ID: "card", Sets: []string{"essentials"}}},
		chart(2024, 10, []string{"pop"}, charts.CH),
		chart(2024, minListeners(2024)-1, []string{"country"}, charts.Billboard),
		chart(2025, minListeners(2025), []string{"metal"}, charts.Billboard),
		chart(2024, 10, []string{"schlager"}, charts.Billboard, charts.AT),
	}
	kept, dropped := placeCharts(rs, all)
	if len(kept) != 4 || len(dropped) != 1 || !strings.HasPrefix(dropped[0].Drop, "only on Billboard, ") {
		t.Fatalf("kept %d dropped %+v", len(kept), dropped)
	}
	for i, want := range [][]string{{"essentials"}, {"mega-mix"}, {"mega-mix", "rock-anthems"}, {"mega-mix", "schlager-fest"}} {
		if !slices.Equal(kept[i].Song.Sets, want) {
			t.Errorf("%d: sets %q, want %q", i, kept[i].Song.Sets, want)
		}
	}
	if !slices.Equal(kept[2].Song.Genres, []string{"metal", "rock"}) {
		t.Errorf("rock-anthems adds its genre: %q", kept[2].Song.Genres)
	}
	// Offline, a song kept from the previous dataset keeps its stored sets.
	prev := chart(2024, 10, []string{"pop"}, charts.CH)
	prev.kept, prev.Song.Sets = true, []string{"christmas-hits", "mega-mix"}
	if kept, _ := placeCharts([]resolved{prev}, all); !slices.Equal(kept[0].Song.Sets, prev.Song.Sets) {
		t.Errorf("offline sets %q", kept[0].Song.Sets)
	}
}

func TestChartSets(t *testing.T) {
	only := func(sets ...string) func(string) bool { return func(s string) bool { return slices.Contains(sets, s) } }
	all := only("mega-mix", "rock-anthems", "schlager-fest", "christmas-hits")
	for _, tc := range []struct {
		genres    []string
		christmas bool
		in        func(string) bool
		want      []string
	}{
		{[]string{"indie", "rock"}, false, all, []string{"mega-mix", "rock-anthems"}},
		{[]string{"indie", "pop", "rock"}, false, all, []string{"mega-mix"}},
		{[]string{"pop"}, true, all, []string{"christmas-hits", "mega-mix"}},
		{[]string{"schlager"}, false, only("schlager-fest"), []string{"schlager-fest"}},
		{nil, false, only(), []string{}},
	} {
		if got := chartSets(tc.genres, tc.christmas, tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("%q christmas=%v: %q, want %q", tc.genres, tc.christmas, got, tc.want)
		}
	}
}

func TestMinListenersAndChristmasTag(t *testing.T) {
	ys := []int{2023, 2024, 2025}
	if minListeners(1990) != billboardMinListeners[ys[0]] || minListeners(2030) != billboardMinListeners[ys[2]] ||
		minListeners(2024) != billboardMinListeners[2024] {
		t.Errorf("minListeners")
	}
	tags := func(names ...string) []lastfm.Tag {
		var out []lastfm.Tag
		for _, n := range names {
			out = append(out, lastfm.Tag{Name: n})
		}
		return out
	}
	if !christmasTag(tags("pop", "Christmas Songs")) || !christmasTag(tags("xmas")) ||
		christmasTag(tags("a", "b", "c", "d", "e", "christmas")) || christmasTag(nil) {
		t.Error("christmasTag")
	}
	for title, want := range map[string]bool{"XMAS": true, "It Can't Be Christmas": true, "Weihnachtsmann": true,
		"Christmastime": true, "Golden": false, "Xmasters": false} {
		if christmasTitle(title) != want {
			t.Errorf("christmasTitle(%q) != %v", title, want)
		}
	}
}

// byReadingLF knows tracks by artist|title.
type byReadingLF map[string]lastfm.Track

func (f byReadingLF) Track(_ context.Context, artist, title string) (lastfm.Track, bool, error) {
	t, ok := f[artist+"|"+title]
	return t, ok, nil
}

func (byReadingLF) ArtistTags(context.Context, string) ([]lastfm.Tag, error) { return nil, nil }

// A chart song takes its most listened reading, a card song its first.
func TestTag_chartSongTakesTheMostListenedReading(t *testing.T) {
	lf := byReadingLF{
		"Kendrick Lamar and SZA|Luther": {Name: "Luther", Artist: "Kendrick Lamar and SZA", Listeners: 1106},
		"Kendrick Lamar|Luther":         {Name: "Luther", Artist: "Kendrick Lamar", Listeners: 900_000, Tags: []lastfm.Tag{{Name: "rap"}}},
	}
	p := &Pipeline{LastFM: lf}
	for _, chart := range []bool{true, false} {
		r := resolved{entry: entry{Artist: "Kendrick Lamar and SZA", Title: "Luther", Chart: chart}, Song: song.Song{Title: "Luther"}}
		if err := p.tag(context.Background(), &r); err != nil {
			t.Fatal(err)
		}
		want := 1106
		if chart {
			want = 900_000
		}
		if r.Song.Listeners != want {
			t.Errorf("chart=%v: listeners %d, want %d", chart, r.Song.Listeners, want)
		}
	}
	// Apple's naming is a reading of a chart song too.
	lf["Zach Bryan|I Remember Everything (feat. Kacey Musgraves)"] = lastfm.Track{Listeners: 1_500_000}
	r := resolved{entry: entry{Artist: "Zach Bryan featuring Kacey Musgraves", Title: "I Remember Everything", Chart: true},
		Song:  song.Song{Title: "I Remember Everything"},
		track: applemusic.Song{ID: "a", Name: "I Remember Everything (feat. Kacey Musgraves)", Artist: "Zach Bryan & Kacey Musgraves"}}
	if err := p.tag(context.Background(), &r); err != nil || r.Song.Listeners != 1_500_000 {
		t.Errorf("Apple reading: %d, %v", r.Song.Listeners, err)
	}
}

func TestPrevious_chartSongKeepsItsYearAndSets(t *testing.T) {
	stored := song.Song{ID: "m1", Artist: "Lola Young", Title: "Messy", Year: 2024, YearSource: song.YearMusicBrainz,
		Sets: []string{"mega-mix"}}
	p := &Pipeline{Previous: map[string]song.Song{song.Key("Lola Young", "Messy"): stored}}
	r, err := p.previous(entry{Artist: "Lola Young", Title: "Messy", Year: 2025, Chart: true})
	if err != nil || r.Song.Year != 2024 || !slices.Equal(r.Song.Sets, []string{"mega-mix"}) || !r.kept {
		t.Errorf("previous %+v, %v", r.Song, err)
	}
}
