package importer

import (
	"bytes"
	"context"
	"encoding/base64"
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

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

// webToken is a JWT the player fixture ships: issuer AMPWebPlay, valid for a month.
func webToken() string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"ES256"}`)) + "." +
		enc([]byte(fmt.Sprintf(`{"iss":"AMPWebPlay","exp":%d}`, time.Now().Add(30*24*time.Hour).Unix()))) + "." + enc([]byte("sig"))
}

const (
	essentialsCSV = "Card#,Artist,Title,URL,Hashed Info,Youtube-Title,Year\n" +
		"1,ABBA,Dancing Queen,https://video.example/1,h1,ABBA - Dancing Queen,1976\n" +
		"2,Joseito Fernandez,Guajira Guantanamera,https://video.example/2,h2,x,1929\n" +
		"3,Nobody,Unknown Song,https://video.example/3,h3,x,1999\n" +
		"4,Someone,No Year,https://video.example/4,h4,x,\n" +
		"5,Lonely,Ballad - 2017 Remaster,https://video.example/7,h7,x,1999\n"
	megaMixCSV = "Card#,Artist,Title,URL,Hashed Info,Youtube-Title,Year\n" +
		"1,Abba,Dancing Queen!,https://video.example/5,h5,x,1977\n" +
		"2,Queen,Bohemian Rhapsody,https://video.example/6,h6,x,1975\n"

	dqRecordings = `{"count":2,"recordings":[
{"id":"dq-live","score":100,"title":"Dancing Queen","disambiguation":"live","first-release-date":"1986","isrcs":["LIVE1"],
 "artist-credit":[{"name":"ABBA"}],"releases":[]},
{"id":"dq-studio","score":100,"title":"Dancing Queen","first-release-date":"1976-08-15","isrcs":["SEAYD7601020"],
 "artist-credit":[{"name":"ABBA"}],"releases":[
  {"id":"r1","title":"Dancing Queen","status":"Official","date":"1976-08-15","release-group":{"id":"rg-dq-single","title":"Dancing Queen","primary-type":"Single"}},
  {"id":"r2","title":"Arrival","status":"Official","date":"1976-10-11","release-group":{"id":"rg-arrival","title":"Arrival","primary-type":"Album","secondary-types":[]}}]}]}`
	brRecordings = `{"count":1,"recordings":[
{"id":"br-studio","score":100,"title":"Bohemian Rhapsody","first-release-date":"1975-10-31","isrcs":["GBUM71029604"],
 "artist-credit":[{"name":"Queen"}],"releases":[
  {"id":"r3","title":"A Night at the Opera","status":"Official","date":"1975-11-21","release-group":{"id":"rg-opera","title":"A Night at the Opera","primary-type":"Album"}}]}]}`
)

func appleSong(id, name, artist, album, albumArtist, date, isrc string, preview bool) string {
	prev := `[]`
	if preview {
		prev = `[{"url":"https://audio.example/p.m4a"}]`
	}
	return fmt.Sprintf(`{"id":%q,"type":"songs","attributes":{"name":%q,"artistName":%q,"albumName":%q,"albumArtistName":%q,`+
		`"releaseDate":%q,"isrc":%q,"artwork":{"url":"https://img.example/%s/{w}x{h}bb.jpg"},"previews":%s}}`,
		id, name, artist, album, albumArtist, date, isrc, id, prev)
}

// services is one fixture server for every source: cards, MusicBrainz, the Apple web player and API, the Cover
// Art Archive.
func services(t *testing.T, mbHits *atomic.Int32) *httptest.Server {
	t.Helper()
	tok := webToken()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cards/{file}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("file") {
		case "first list.csv":
			_, _ = fmt.Fprint(w, essentialsCSV)
		case "second.csv":
			_, _ = fmt.Fprint(w, megaMixCSV)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /mb/ws/2/recording", func(w http.ResponseWriter, r *http.Request) {
		mbHits.Add(1)
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Reworm/") || !strings.HasSuffix(r.Header.Get("User-Agent"), "( https://github.com/trick77/reworm )") {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		q := r.URL.Query().Get("query")
		switch {
		case strings.Contains(q, `recording:"Dancing Queen"`):
			_, _ = fmt.Fprint(w, dqRecordings)
		case strings.Contains(q, `recording:"Bohemian Rhapsody"`):
			_, _ = fmt.Fprint(w, brRecordings)
		default:
			_, _ = fmt.Fprint(w, `{"count":0,"recordings":[]}`)
		}
	})
	mux.HandleFunc("GET /player/us/browse", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<script src="/assets/index~abc123.js"></script>`)
	})
	mux.HandleFunc("GET /player/assets/index~abc123.js", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `const t="%s";`, tok)
	})
	mux.HandleFunc("GET /apple/v1/catalog/{sf}/songs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+tok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var data []string
		switch r.URL.Query().Get("filter[isrc]") {
		case "SEAYD7601020":
			// An unrelated track Apple files under the same ISRC: never the song, even as the only own-album hit.
			data = append(data, appleSong("dq-comp", "Dancing Queen", "ABBA", "Party Hits", "Various Artists", "1976-01-01", "SEAYD7601020", true),
				appleSong("dq-alien", "What Out You", "Someone Else", "What Out You - Single", "Someone Else", "1975-01-01", "SEAYD7601020", true))
		case "GBUM71029604":
			if r.PathValue("sf") == "de" {
				data = append(data,
					appleSong("br-nopreview", "Bohemian Rhapsody", "Queen", "Greatest Hits", "Queen", "1981-10-26", "GBUM71029604", false),
					appleSong("br-va", "Bohemian Rhapsody", "Queen", "Rock Ballads", "Various Artists", "1975-01-01", "GBUM71029604", true),
					appleSong("br-hits", "Bohemian Rhapsody", "Queen", "Greatest Hits", "Queen", "1981-10-26", "GBUM71029604", true))
			}
		}
		_, _ = fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(data, ","))
	})
	mux.HandleFunc("GET /apple/v1/catalog/{sf}/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("types") == "albums" && q.Get("term") == "ABBA Arrival" && r.PathValue("sf") == "ch":
			_, _ = fmt.Fprint(w, `{"results":{"albums":{"data":[
{"id":"al-gold","attributes":{"name":"Gold","artistName":"ABBA","releaseDate":"1992-09-21"}},
{"id":"al-arrival-1999","attributes":{"name":"Arrival","artistName":"ABBA","releaseDate":"1999-01-01"}},
{"id":"al-arrival","attributes":{"name":"Arrival (Bonus Track Version)","artistName":"ABBA","releaseDate":"1976-10-11"}}]}}}`)
		case q.Get("types") == "songs" && q.Get("term") == "Joseito Fernandez Guajira Guantanamera" && r.PathValue("sf") == "us":
			_, _ = fmt.Fprintf(w, `{"results":{"songs":{"data":[%s,%s]}}}`,
				appleSong("gg-other", "Guantanamera", "Some Band", "Covers", "Some Band", "2001", "X", true),
				appleSong("gg-1", "Guajira Guantanamera", "Joseíto Fernández", "Mi biografia", "Joseíto Fernández", "1958", "CU1", true))
		default:
			_, _ = fmt.Fprint(w, `{"results":{}}`)
		}
	})
	mux.HandleFunc("GET /apple/v1/catalog/{sf}/songs/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("sf") + "/" + r.PathValue("id") {
		case "de/br-opera":
			_, _ = fmt.Fprintf(w, `{"data":[%s]}`, appleSong("br-opera", "Bohemian Rhapsody", "Queen", "A Night at the Opera (2011 Remaster)", "Queen", "1975-11-21", "GBUM71029605", true))
		case "ch/gg-alt":
			_, _ = fmt.Fprintf(w, `{"data":[%s]}`, appleSong("gg-alt", "Guajira Guantanamera", "Joseíto Fernández", "Mi biografia", "Joseíto Fernández", "1958", "CU2", true))
		case "ch/no-preview":
			_, _ = fmt.Fprintf(w, `{"data":[%s]}`, appleSong("no-preview", "X", "Y", "Z", "Y", "1990", "I", false))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /apple/v1/catalog/ch/albums/al-arrival/tracks", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":[%s,%s]}`,
			appleSong("dq-tiger", "Tiger", "ABBA", "Arrival (Bonus Track Version)", "ABBA", "1976-10-11", "SEAYD7601090", true),
			appleSong("dq-arrival", "Dancing Queen", "ABBA", "Arrival (Bonus Track Version)", "ABBA", "1976-10-11", "SEAYD7601020", true))
	})
	mux.HandleFunc("HEAD /caa/release-group/{id}/front-500", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "rg-opera" {
			http.Redirect(w, r, "https://archive.example/front.jpg", http.StatusTemporaryRedirect)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func endpoints(srv *httptest.Server) Endpoints {
	return Endpoints{MusicBrainz: srv.URL + "/mb", AppleAPI: srv.URL + "/apple", ApplePlayer: srv.URL + "/player",
		CoverArt: srv.URL + "/caa", MBEvery: time.Nanosecond, AppleEvery: time.Nanosecond}
}

func run(t *testing.T, ep Endpoints, env map[string]string, args ...string) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	code := Main(context.Background(), args, func(k string) string { return env[k] }, &stderr, ep)
	return code, stderr.String()
}

func TestImport_cardsThenSongs(t *testing.T) {
	var mbHits atomic.Int32
	srv := services(t, &mbHits)
	ep := endpoints(srv)
	data := t.TempDir()
	envFile := filepath.Join(t.TempDir(), ".env.local")
	if err := os.WriteFile(envFile, []byte("# local\nexport BACKEND_CARDS_BASE_URL=\""+srv.URL+"/cards/\"\n\n"+
		"BACKEND_CARDS_FILES='first list.csv=Essentials;second.csv=Mega Mix'\nBACKEND_LASTFM_API_KEY=unused\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if code, out := run(t, ep, nil, "-env", envFile, "-data", data, "cards"); code != 0 {
		t.Fatalf("cards: exit %d\n%s", code, out)
	}
	snap, err := os.ReadFile(filepath.Join(data, "cards", "essentials.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "artist,title,year\nABBA,Dancing Queen,1976\nJoseito Fernandez,Guajira Guantanamera,1929\n" +
		"Nobody,Unknown Song,1999\nSomeone,No Year,\nLonely,Ballad - 2017 Remaster,1999\n"; string(snap) != want {
		t.Errorf("snapshot:\n%s", snap)
	}
	for _, f := range []string{"essentials.csv", "mega-mix.csv"} {
		b, _ := os.ReadFile(filepath.Join(data, "cards", f))
		if bytes.Contains(b, []byte("video.example")) || bytes.Contains(b, []byte("list")) {
			t.Errorf("%s keeps source data:\n%s", f, b)
		}
	}

	// Overrides: one drops the unknown song, two pick other Apple tracks (album, ISRC and cover follow), one names
	// an id the dataset lacks. A snapshot of a set no longer configured is left out, with a warning.
	if err := os.WriteFile(filepath.Join(data, "overrides.json"), []byte(`{"_doc":"x","songs":{
		"nobody--unknown-song":{"exclude":true},
		"joseito-fernandez--guajira-guantanamera":{"apple_id":"gg-alt"},
		"br-studio":{"apple_id":"br-opera","storefront":"de"},
		"missing-id":{"year":1990}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "cards", "old-set.csv"), []byte("artist,title,year\nOld,Song,1990\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := run(t, ep, nil, "-env", envFile, "-data", data, "-workers", "2", "songs")
	if code != 0 {
		t.Fatalf("songs: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "no longer configured") || !strings.Contains(out, "old-set.csv") {
		t.Errorf("no warning about the stray snapshot:\n%s", out)
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
	if len(songs) != 4 {
		t.Fatalf("songs:\n%s", b)
	}
	// The title as shown drops the release note; the id keeps the card's spelling.
	if l := byID["lonely--ballad-2017-remaster"]; l.Title != "Ballad" {
		t.Errorf("Lonely = %+v", l)
	}

	dq := byID["dq-studio"]
	if dq.Year != 1976 || dq.YearSource != "card" || dq.AppleID != "dq-arrival" || dq.Storefront != "ch" ||
		dq.AppleAlbum != "Arrival (Bonus Track Version)" || dq.CoverSrc != "apple" ||
		dq.CoverURL != "https://img.example/dq-arrival/{w}x{h}bb.jpg" || strings.Join(dq.Sets, ",") != "essentials,mega-mix" ||
		dq.ISRC != "SEAYD7601020" || dq.MBID != "dq-studio" {
		t.Errorf("Dancing Queen = %+v", dq)
	}
	// The override's track is on the original album: its album, ISRC and the Apple cover replace the compilation's
	// and the archive's.
	br := byID["br-studio"]
	if br.AppleID != "br-opera" || br.Storefront != "de" || br.AppleAlbum != "A Night at the Opera (2011 Remaster)" ||
		br.ISRC != "GBUM71029605" || br.CoverSrc != "apple" || br.CoverURL != "https://img.example/br-opera/{w}x{h}bb.jpg" {
		t.Errorf("Bohemian Rhapsody = %+v", br)
	}
	gg := byID["joseito-fernandez--guajira-guantanamera"]
	if gg.Year != 1929 || gg.MBID != "" || gg.AppleID != "gg-alt" || gg.Storefront != "ch" || gg.ISRC != "CU2" ||
		gg.AppleAlbum != "Mi biografia" || gg.CoverSrc != "" {
		t.Errorf("Guantanamera = %+v", gg)
	}
	if bytes.Contains(b, []byte("audio.example")) {
		t.Error("songs.jsonl stores a preview URL")
	}

	rep, err := os.ReadFile(filepath.Join(data, "import-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Every section shows the dataset as written: the excluded song is gone, the overridden tracks count as set.
	for _, want := range []string{
		"| cards | 7 |", "| cards without a year (dropped) | 1 |", "| songs | 4 |", "| overrides applied | 3 |",
		"| cover: Apple (original album) | 2 |", "| cover: Cover Art Archive | 0 |",
		"| essentials | 5 | 1 | 3 | 1 | 2 |", "| mega-mix | 2 | 0 | 2 | 2 | 2 |",
		"| Joseito Fernandez | Guajira Guantanamera | 1929 | – | 1958 |  |",
		"| ABBA | Dancing Queen | 1976, 1977 |",
		"## Low-confidence matches (0)", "## Not found on MusicBrainz (2)", "## No Apple track with a preview (1)",
		"- `missing-id`",
	} {
		if !strings.Contains(string(rep), want) {
			t.Errorf("report lacks %q:\n%s", want, rep)
		}
	}
	for _, gone := range []string{"Nobody", "| Queen | Bohemian Rhapsody | 1975 | 1975 | 1981", "Old | Song", "found by search"} {
		if strings.Contains(string(rep), gone) {
			t.Errorf("report still shows %q:\n%s", gone, rep)
		}
	}
	if strings.Contains(string(rep), "video.example") || strings.Contains(string(rep), srv.URL) {
		t.Error("report leaks a source URL")
	}

	// A re-run answers from the cache: MusicBrainz is not asked again, the output is the same.
	before := mbHits.Load()
	if code, out := run(t, ep, nil, "-env", envFile, "-data", data, "songs"); code != 0 {
		t.Fatalf("rerun: exit %d\n%s", code, out)
	}
	again, _ := os.ReadFile(filepath.Join(data, "songs.jsonl"))
	if mbHits.Load() != before || !bytes.Equal(again, b) {
		t.Errorf("rerun: %d new MusicBrainz requests, output changed: %v", mbHits.Load()-before, !bytes.Equal(again, b))
	}
	if _, err := os.Stat(filepath.Join(data, "cache", "apple-token.json")); err != nil {
		t.Errorf("apple token not kept: %v", err)
	}

	// Offline: a card whose query is not cached keeps its previous entry (its new card year included), a new card
	// that would need the network is left out, MusicBrainz is not asked.
	if err := os.WriteFile(filepath.Join(data, "overrides.json"), []byte(`{"songs":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, ep, nil, "-env", envFile, "-data", data, "songs"); code != 0 {
		t.Fatalf("online rerun: exit %d\n%s", code, out)
	}
	online, _ := os.ReadFile(filepath.Join(data, "songs.jsonl"))
	if err := os.WriteFile(filepath.Join(data, "cards", "essentials.csv"),
		[]byte("artist,title,year\nABBA,Dancing Queen,1975\nFresh,Track,2001\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before = mbHits.Load()
	code, out = run(t, ep, nil, "-env", envFile, "-data", data, "-offline", "songs")
	if code != 0 || mbHits.Load() != before {
		t.Fatalf("offline: exit %d, %d MusicBrainz requests\n%s", code, mbHits.Load()-before, out)
	}
	if !strings.Contains(out, "left out") {
		t.Errorf("no word about the left-out card:\n%s", out)
	}
	off, _ := os.ReadFile(filepath.Join(data, "songs.jsonl"))
	offSongs, _ := song.Decode(off)
	var prevDQ, offDQ song.Song
	onSongs, _ := song.Decode(online)
	for _, s := range onSongs {
		if s.ID == "dq-studio" {
			prevDQ = s
		}
	}
	for _, s := range offSongs {
		if s.ID == "dq-studio" {
			offDQ = s
		}
		if s.Artist == "Fresh" {
			t.Errorf("offline run resolved a card it has no answers for: %+v", s)
		}
	}
	if offDQ.Year != 1975 || offDQ.AppleID != prevDQ.AppleID || offDQ.CoverURL != prevDQ.CoverURL || offDQ.AppleID == "" {
		t.Errorf("kept Dancing Queen = %+v, previous %+v", offDQ, prevDQ)
	}
	rep, _ = os.ReadFile(filepath.Join(data, "import-report.md"))
	if !strings.Contains(string(rep), previousNote) {
		t.Errorf("report does not say what was kept:\n%s", rep)
	}

	// An override naming no playable track fails the run instead of leaving the song without a clip.
	for _, id := range []string{"gone", "no-preview"} {
		if err := os.WriteFile(filepath.Join(data, "overrides.json"),
			[]byte(`{"songs":{"dq-studio":{"apple_id":"`+id+`"}}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, out := run(t, ep, nil, "-env", envFile, "-data", data, "songs"); code != 1 || !strings.Contains(out, "no track with a preview") {
			t.Errorf("override %s: exit %d\n%s", id, code, out)
		}
	}
}

// A year that moves after the resolve (a merge of two cards, a year override) redoes the original-album flag and
// the cover: the track was judged against the old year.
func TestRejudge_afterMergeAndYearOverride(t *testing.T) {
	caa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer caa.Close()
	p := &Pipeline{Covers: &CoverArt{BaseURL: caa.URL, HTTP: caa.Client()}}
	// The group is dated by a reissue (as Waterloo's album shows 2004): only the card year places the track.
	m := musicbrainz.Match{Original: musicbrainz.Group{ID: "rg", Title: "Album", Date: "2004"}}
	track := apple.Song{ID: "t", Name: "Song", Album: "Album", ReleaseDate: "1976-05-01", Artwork: "https://img.example/{w}x{h}.jpg", Preview: true}
	judged := func(year int) resolved {
		r := resolved{entry: entry{Artist: "A", Title: "Song", Year: year}, match: m,
			Song: song.Song{ID: "rec", Artist: "A", Title: "Song", Year: year, YearSource: song.YearCard}}
		if err := p.setTrack(context.Background(), &r, track, "ch"); err != nil {
			t.Fatal(err)
		}
		return r
	}
	late := judged(1990)
	if late.AppleOriginal || late.Song.CoverSrc != "" {
		t.Fatalf("1990: original=%v cover=%q", late.AppleOriginal, late.Song.CoverSrc)
	}
	out, _, err := p.merge(context.Background(), []resolved{late, judged(1976)})
	if err != nil || len(out) != 1 || out[0].Song.Year != 1976 || !out[0].AppleOriginal || out[0].Song.CoverSrc != song.CoverApple {
		t.Errorf("merged: %+v, %v", out, err)
	}
	kept, _, _, err := p.applyOverrides(context.Background(), []resolved{judged(1990)}, map[string]Override{"rec": {Year: 1976}})
	if err != nil || !kept[0].AppleOriginal || kept[0].Song.CoverSrc != song.CoverApple || kept[0].Song.YearSource != song.YearOverride {
		t.Errorf("year override: %+v, %v", kept, err)
	}
	// A song kept from the previous dataset has no track to judge: left as it was.
	prev := resolved{Song: song.Song{ID: "p", Year: 1976, AppleID: "x", CoverSrc: song.CoverApple}, trackYear: 1990, kept: true}
	if err := p.rejudge(context.Background(), &prev); err != nil || prev.Song.AppleID != "x" {
		t.Errorf("kept song rejudged: %+v, %v", prev.Song, err)
	}
}

// fakeMB answers each search from bounded or unbounded by its year bound.
type fakeMB struct{ bounded, unbounded []musicbrainz.Recording }

func (f fakeMB) Search(_ context.Context, _, _ string, until int) ([]musicbrainz.Recording, error) {
	if until > 0 {
		return f.bounded, nil
	}
	return f.unbounded, nil
}

func TestFindRecording_labelsTheUnboundedFallback(t *testing.T) {
	rec := func(date string) []musicbrainz.Recording {
		return []musicbrainz.Recording{{ID: "r", Title: "Song", Credit: "Act", Artists: []string{"Act"}, FirstReleaseDate: date}}
	}
	e := entry{Artist: "Act", Title: "Song", Year: 1929}
	for _, tc := range []struct {
		mb   fakeMB
		ok   bool
		want string
	}{
		{fakeMB{bounded: rec("1929")}, true, ""},
		{fakeMB{unbounded: rec("1940")}, true, "MusicBrainz has it only from 1940, after the card year"},
		{fakeMB{unbounded: rec("")}, true, "MusicBrainz knows no release date for it"},
		{fakeMB{unbounded: rec("1928")}, true, "MusicBrainz matched only by a search without the year bound"},
		{fakeMB{}, false, ""},
	} {
		p := &Pipeline{MB: tc.mb}
		_, ok, why, err := p.findRecording(context.Background(), e)
		if err != nil || ok != tc.ok || why != tc.want {
			t.Errorf("%+v: ok=%v why=%q err=%v", tc.mb, ok, why, err)
		}
	}
}

// byArtistMB answers only searches for its artist, recording each query.
type byArtistMB struct {
	recs    map[string][]musicbrainz.Recording
	queries *[]string
}

func (f byArtistMB) Search(_ context.Context, artist, title string, until int) ([]musicbrainz.Recording, error) {
	*f.queries = append(*f.queries, fmt.Sprintf("%s|%s|%d", artist, title, until))
	return f.recs[artist], nil
}

// boundMB answers bounded and unbounded searches apart, by artist.
type boundMB struct {
	bounded, unbounded map[string][]musicbrainz.Recording
}

func (f boundMB) Search(_ context.Context, artist, _ string, until int) ([]musicbrainz.Recording, error) {
	if until > 0 {
		return f.bounded[artist], nil
	}
	return f.unbounded[artist], nil
}

// MusicBrainz finds "Queen, David Bowie" only as Queen: searched by the card's first name last, after the
// unbounded search, and only a recording credited to the card's artists is taken; "Simon" alone is not Simon &
// Garfunkel.
func TestFindRecording_searchesTheLeadArtistLast(t *testing.T) {
	rec := musicbrainz.Recording{ID: "up", Title: "Under Pressure", Credit: "Queen & David Bowie",
		Artists: []string{"Queen", "David Bowie"}, FirstReleaseDate: "1981-10-26"}
	var queries []string
	p := &Pipeline{MB: byArtistMB{map[string][]musicbrainz.Recording{"Queen": {rec}}, &queries}}
	m, ok, why, err := p.findRecording(context.Background(), entry{Artist: "Queen, David Bowie", Title: "Under Pressure", Year: 1981})
	if err != nil || !ok || m.Recording.ID != "up" || m.Exact || why != "MusicBrainz matched as Queen – Under Pressure" {
		t.Errorf("ok=%v id=%s exact=%v why=%q err=%v", ok, m.Recording.ID, m.Exact, why, err)
	}
	if want := []string{"Queen, David Bowie|Under Pressure|1982", "Queen, David Bowie|Under Pressure|0", "Queen|Under Pressure|1982"}; !slices.Equal(queries, want) {
		t.Errorf("queries %q", queries)
	}
	simon := musicbrainz.Recording{ID: "s", Title: "Mrs. Robinson", Credit: "Simon", Artists: []string{"Simon"}, FirstReleaseDate: "1968"}
	queries = nil
	p = &Pipeline{MB: byArtistMB{map[string][]musicbrainz.Recording{"Simon": {simon}}, &queries}}
	if _, ok, _, _ := p.findRecording(context.Background(), entry{Artist: "Simon & Garfunkel", Title: "Mrs. Robinson", Year: 1968}); ok {
		t.Error("Simon taken for Simon & Garfunkel")
	}
	// The unbounded search finds it only undated or later: the first name's bounded pick is the better one.
	late := musicbrainz.Recording{ID: "late", Title: "Under Pressure", Credit: "Queen & David Bowie",
		Artists: []string{"Queen", "David Bowie"}, FirstReleaseDate: "1999"}
	for _, unbounded := range []musicbrainz.Recording{late, {ID: "undated", Title: "Under Pressure", Credit: "Queen & David Bowie",
		Artists: []string{"Queen", "David Bowie"}}} {
		p = &Pipeline{MB: boundMB{unbounded: map[string][]musicbrainz.Recording{"Queen, David Bowie": {unbounded}},
			bounded: map[string][]musicbrainz.Recording{"Queen": {rec}}}}
		m, ok, why, _ := p.findRecording(context.Background(), entry{Artist: "Queen, David Bowie", Title: "Under Pressure", Year: 1981})
		if !ok || m.Recording.ID != "up" || why != "MusicBrainz matched as Queen – Under Pressure" {
			t.Errorf("after %s: ok=%v id=%s why=%q", unbounded.ID, ok, m.Recording.ID, why)
		}
	}
	// … and when the first name finds nothing, the weak match stays.
	p = &Pipeline{MB: boundMB{unbounded: map[string][]musicbrainz.Recording{"Queen, David Bowie": {late}}}}
	if m, ok, why, _ := p.findRecording(context.Background(), entry{Artist: "Queen, David Bowie", Title: "Under Pressure", Year: 1981}); !ok || m.Recording.ID != "late" ||
		why != "MusicBrainz has it only from 1999, after the card year" {
		t.Errorf("weak kept: ok=%v id=%s why=%q", ok, m.Recording.ID, why)
	}
	// No other name: no further search.
	queries = nil
	p = &Pipeline{MB: byArtistMB{map[string][]musicbrainz.Recording{}, &queries}}
	if _, ok, _, _ := p.findRecording(context.Background(), entry{Artist: "ABBA", Title: "Waterloo", Year: 1974}); ok || len(queries) != 2 {
		t.Errorf("ok=%v queries %q", ok, queries)
	}
}

// fakeCatalog answers ISRC lookups and song searches; albums are never found.
type fakeCatalog struct {
	byISRC   []apple.Song
	searched map[string][]apple.Song
}

func (f fakeCatalog) SongsByISRC(context.Context, string, []string) ([]apple.Song, error) {
	return f.byISRC, nil
}

func (f fakeCatalog) SearchSongs(_ context.Context, _, term string, _ int) ([]apple.Song, error) {
	return f.searched[term], nil
}

func (fakeCatalog) SearchAlbums(context.Context, string, string, int) ([]apple.Album, error) {
	return nil, nil
}

func (fakeCatalog) AlbumTracks(context.Context, string, string) ([]apple.Song, error) {
	return nil, nil
}

func (fakeCatalog) SongByID(context.Context, string, string) (apple.Song, bool, error) {
	return apple.Song{}, false, nil
}

// Several artists and film titles: an ISRC hit or a search hit counts by every reading of the card title and by
// the credit as a set or its main artist; another act under the ISRC does not.
func TestFindTrack_severalArtistsAndFilmTitles(t *testing.T) {
	tr := func(id, name, artist string) apple.Song {
		return apple.Song{ID: id, Name: name, Artist: artist, Album: "A", Preview: true}
	}
	grease := musicbrainz.Match{ISRCs: []string{"NLF057890002"}, Recording: musicbrainz.Recording{ID: "sn", Title: "Summer Nights",
		Credit: "John Travolta, Olivia Newton‐John & Cast", Artists: []string{"John Travolta", "Olivia Newton‐John", "Cast"}}}
	queen := musicbrainz.Match{ISRCs: []string{"GBUM71029622"}, Recording: musicbrainz.Recording{ID: "up", Title: "Under Pressure",
		Credit: "Queen & David Bowie", Artists: []string{"Queen", "David Bowie"}}}
	godfather := musicbrainz.Match{ISRCs: []string{"USMC17251836"}, Recording: musicbrainz.Recording{ID: "gw", Title: "The Godfather Waltz",
		Credit: "Nino Rota", Artists: []string{"Nino Rota"}}}
	yusuf := musicbrainz.Match{Recording: musicbrainz.Recording{ID: "fs", Title: "Father and Son", Credit: "Yusuf", Artists: []string{"Yusuf"}}}
	oneKissRemix := musicbrainz.Match{Recording: musicbrainz.Recording{ID: "jz", Title: "One Kiss (with Dua Lipa) - Jauz Remix",
		Credit: "Calvin Harris", Artists: []string{"Calvin Harris"}}}
	unpluggedMatch := musicbrainz.Match{Recording: musicbrainz.Recording{ID: "u", Title: "Where Did You Sleep Last Night",
		Disambiguation: "live, MTV Unplugged", Credit: "Nirvana", Artists: []string{"Nirvana"}}}
	glory := musicbrainz.Match{Recording: musicbrainz.Recording{ID: "gl", Title: "Glory", Credit: "Common & John Legend",
		Artists: []string{"Common", "John Legend"}}}
	for _, tc := range []struct {
		e     entry
		m     musicbrainz.Match
		apple fakeCatalog
		want  string
	}{
		{entry{Artist: "John Travolta, Olivia Newton-John", Title: "Grease (Summer Nights)", Year: 1978}, grease,
			fakeCatalog{byISRC: []apple.Song{tr("other", "Summer Nights", "Other Act"), tr("sn", "Summer Nights", "John Travolta & Olivia Newton-John")}}, "sn"},
		{entry{Artist: "Queen, David Bowie", Title: "Under Pressure", Year: 1981}, queen,
			fakeCatalog{byISRC: []apple.Song{tr("up", "Under Pressure (feat. David Bowie)", "Queen")}}, "up"},
		{entry{Artist: "Nino Rota", Title: "Der Pate (The Godfather Waltz)", Year: 1972}, godfather,
			fakeCatalog{byISRC: []apple.Song{tr("gw", "The Godfather Waltz", "Nino Rota & Carlo Savina")}}, "gw"},
		// Another act under the ISRC is no hit.
		{entry{Artist: "Queen, David Bowie", Title: "Under Pressure", Year: 1981}, queen,
			fakeCatalog{byISRC: []apple.Song{tr("x", "Under Pressure", "Some Cover Band")}}, ""},
		// Searched as the card writes the title, found by the song inside it.
		{entry{Artist: "Luis Bacalov, Rocky Roberts", Title: "Django Unchained (Django)", Year: 2012}, musicbrainz.Match{},
			fakeCatalog{searched: map[string][]apple.Song{"Luis Bacalov, Rocky Roberts Django Unchained (Django)": {
				tr("dj", "Django", "Luis Bacalov & Rocky Roberts")}}}, "dj"},
		{entry{Artist: "Calvin Harris", Title: "One Kiss (with Dua Lipa)", Year: 2018}, musicbrainz.Match{},
			fakeCatalog{searched: map[string][]apple.Song{"Calvin Harris One Kiss (with Dua Lipa)": {
				tr("ok", "One Kiss", "Calvin Harris & Dua Lipa")}}}, "ok"},
		// The act under another of its names; its re-recording is no hit.
		{entry{Artist: "Yusuf / Cat Stevens", Title: "Father And Son", Year: 1970}, yusuf,
			fakeCatalog{searched: map[string][]apple.Song{"Yusuf / Cat Stevens Father and Son": {
				tr("re", "Father and Son (Re-Recorded Version)", "Yusuf / Cat Stevens"), tr("fs", "Father and Son", "Cat Stevens")}}}, "fs"},
		// The live take alone is no hit: no clip beats the wrong recording.
		{entry{Artist: "Common, John Legend", Title: "Selma (Glory)", Year: 2014}, glory,
			fakeCatalog{searched: map[string][]apple.Song{"Common Glory": {tr("live", "Glory [Live]", "Common & John Legend")}}}, ""},
		// A remix MusicBrainz took for the song is found first, the plain take by the card's title wins.
		{entry{Artist: "Calvin Harris", Title: "One Kiss (with Dua Lipa)", Year: 2018}, oneKissRemix,
			fakeCatalog{searched: map[string][]apple.Song{
				"Calvin Harris One Kiss (with Dua Lipa) - Jauz Remix": {tr("jz", "One Kiss (Jauz Remix)", "Calvin Harris & Dua Lipa")},
				"Calvin Harris One Kiss (with Dua Lipa)":              {tr("ok", "One Kiss", "Calvin Harris & Dua Lipa")}}}, "ok"},
		// A song MusicBrainz knows only live: its live take, nothing plain anywhere.
		{entry{Artist: "Nirvana", Title: "Where Did You Sleep Last Night", Year: 1994}, unpluggedMatch,
			fakeCatalog{searched: map[string][]apple.Song{"Nirvana Where Did You Sleep Last Night": {
				tr("l", "Where Did You Sleep Last Night (Live)", "Nirvana")}}}, "l"},
		// An act the card does not name, by search.
		{entry{Artist: "Queen", Title: "Bohemian Rhapsody", Year: 1975}, musicbrainz.Match{},
			fakeCatalog{searched: map[string][]apple.Song{"Queen Bohemian Rhapsody": {tr("ql", "Bohemian Rhapsody", "Queen + Adam Lambert")}}}, ""},
	} {
		p := &Pipeline{Apple: tc.apple}
		got, _, _, err := p.findTrack(context.Background(), tc.e, tc.m)
		if err != nil || got.ID != tc.want {
			t.Errorf("%s – %s: got %q, want %q (%v)", tc.e.Artist, tc.e.Title, got.ID, tc.want, err)
		}
	}
}

// best: the original-album track, then one on no album named live or remixed, then the artist's own album; never a
// track named a remix or live take the card does not ask for.
func TestBest_prefersOriginalThenPlainThenOwnAlbum(t *testing.T) {
	m := musicbrainz.Match{Original: musicbrainz.Group{ID: "rg", Title: "Help!", Date: "1965-08-06"}}
	e := entry{Artist: "The Beatles", Title: "Help!", Year: 1965}
	tr := func(id, name, album, albumArtist, date string) apple.Song {
		return apple.Song{ID: id, Name: name, Artist: "The Beatles", Album: album, AlbumArtist: albumArtist, ReleaseDate: date, Preview: true}
	}
	remix := tr("a-remix", "Help! (Remix)", "Help! - Single", "The Beatles", "2020")
	live := tr("b-live", "Help! (Live)", "Live at the BBC", "The Beatles", "1994")
	comp := tr("c-comp", "Help!", "Sixties Hits", "Various Artists", "1965")
	own := tr("d-own", "Help!", "1", "The Beatles", "2000")
	orig := tr("e-orig", "Help!", "Help! (Remastered)", "The Beatles", "1965-08-06")
	liveAlbum := tr("f-hits", "Help!", "Live Party Hits", "The Beatles", "1965")
	for _, tc := range []struct {
		songs []apple.Song
		want  string
	}{
		{[]apple.Song{remix, live, comp, own, orig}, "e-orig"},
		{[]apple.Song{remix, live, comp, own}, "d-own"},
		{[]apple.Song{remix, live, liveAlbum, comp}, "c-comp"},
		{[]apple.Song{remix, live, liveAlbum}, "f-hits"},
		{[]apple.Song{remix, live}, ""},
	} {
		if got, _ := best(tc.songs, m, e, nil); got.ID != tc.want {
			t.Errorf("best = %q, want %q", got.ID, tc.want)
		}
	}
	// Asked for by the card, the live take is the song.
	if got, _ := best([]apple.Song{remix, live}, m, entry{Artist: "The Beatles", Title: "Help! (Live)", Year: 1994}, nil); got.ID != "b-live" {
		t.Errorf("asked live: %q", got.ID)
	}
	// A song MusicBrainz knows only live: the live take is fine, a plain one still first.
	unplugged := musicbrainz.Match{Recording: musicbrainz.Recording{Title: "Where Did You Sleep Last Night", Disambiguation: "live, MTV Unplugged"}}
	nirvana := entry{Artist: "Nirvana", Title: "Where Did You Sleep Last Night", Year: 1994}
	liveTake := apple.Song{ID: "l", Name: "Where Did You Sleep Last Night (Live)", Artist: "Nirvana", Album: "MTV Unplugged in New York", Preview: true}
	studio := apple.Song{ID: "s", Name: "Where Did You Sleep Last Night", Artist: "Nirvana", Album: "Hits", Preview: true}
	if got, _ := best([]apple.Song{liveTake}, unplugged, nirvana, nil); got.ID != "l" {
		t.Errorf("live recording: %q", got.ID)
	}
	if got, _ := best([]apple.Song{liveTake, studio}, unplugged, nirvana, nil); got.ID != "s" {
		t.Errorf("plain first: %q", got.ID)
	}
	// A re-recording is not the 1970 song, the only candidate or not; nor is its "Tillerman²" the original album.
	tiller := musicbrainz.Match{Original: musicbrainz.Group{ID: "rg", Title: "Tea for the Tillerman", Date: "1970-11-23"}}
	cat := entry{Artist: "Yusuf / Cat Stevens", Title: "Father And Son", Year: 1970}
	rerec := apple.Song{ID: "re", Name: "Father and Son (Re-Recorded Version)", Artist: "Yusuf / Cat Stevens",
		Album: "Tea for the Tillerman²", ReleaseDate: "2020-09-18", Preview: true}
	if got, ok := best([]apple.Song{rerec}, tiller, cat, nil); ok {
		t.Errorf("re-recording picked: %q", got.ID)
	}
	rerec.Name, rerec.ReleaseDate = "Father and Son", "1970-11-23" // Apple dates the album's tracks so
	if onOriginal(rerec, tiller, 1970) {
		t.Error("Tillerman² is the original album")
	}
}

func TestImport_writesTheOverridesTemplate(t *testing.T) {
	var mbHits atomic.Int32
	ep := endpoints(services(t, &mbHits))
	data := t.TempDir()
	if err := os.MkdirAll(filepath.Join(data, "cards"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "cards", "rock.csv"), []byte("artist,title,year\nQueen,Bohemian Rhapsody,1975\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, ep, map[string]string{"BACKEND_CARDS_FILES": "src.csv=Rock"}, "-data", data, "songs"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	ov, err := loadOverrides(filepath.Join(data, "overrides.json"))
	if err != nil || len(ov) != 0 {
		t.Errorf("template: %v, %v", ov, err)
	}
	b, _ := os.ReadFile(filepath.Join(data, "overrides.json"))
	if !bytes.Contains(b, []byte(`"_doc"`)) || !bytes.Contains(b, []byte(`"songs": {}`)) {
		t.Errorf("template:\n%s", b)
	}
}

func TestMain_usageAndConfigErrors(t *testing.T) {
	ep := Endpoints{}
	data := t.TempDir()
	for _, tc := range []struct {
		args []string
		env  map[string]string
		code int
		want string
	}{
		{nil, nil, 2, "usage"},
		{[]string{"other"}, nil, 2, "usage"},
		{[]string{"-nope"}, nil, 2, "flag provided but not defined"},
		{[]string{"-env", filepath.Join(data, "missing"), "cards"}, nil, 1, "env file"},
		{[]string{"-data", data, "cards"}, nil, 1, "BACKEND_CARDS_BASE_URL"},
		{[]string{"-data", data, "cards"}, map[string]string{"BACKEND_CARDS_BASE_URL": "http://x", "BACKEND_CARDS_FILES": "bad"}, 1, "file=Set Name"},
		{[]string{"-data", data, "songs"}, nil, 1, "BACKEND_CARDS_FILES"},
		{[]string{"-data", data, "songs"}, map[string]string{"BACKEND_CARDS_FILES": "x.csv=Rock"}, 1, "snapshot of set rock"},
	} {
		code, out := run(t, ep, tc.env, tc.args...)
		if code != tc.code || !strings.Contains(out, tc.want) {
			t.Errorf("%v: exit %d, output %q; want %d and %q", tc.args, code, out, tc.code, tc.want)
		}
	}
	// A broken overrides file fails the run before any request.
	if err := os.MkdirAll(filepath.Join(data, "cards"), 0o750); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(data, "cards", "x.csv"), []byte("artist,title,year\nA,T,1990\n"), 0o600)
	_ = os.WriteFile(filepath.Join(data, "overrides.json"), []byte("{"), 0o600)
	if code, out := run(t, ep, map[string]string{"BACKEND_CARDS_FILES": "x.csv=X"}, "-data", data, "songs"); code != 1 || !strings.Contains(out, "overrides") {
		t.Errorf("bad overrides: %d %q", code, out)
	}
}

func TestImport_cardDownloadFails(t *testing.T) {
	var mbHits atomic.Int32
	srv := services(t, &mbHits)
	env := map[string]string{"BACKEND_CARDS_BASE_URL": srv.URL + "/cards", "BACKEND_CARDS_FILES": "gone.csv=Rock"}
	if code, out := run(t, endpoints(srv), env, "-data", t.TempDir(), "cards"); code != 1 || !strings.Contains(out, "cards Rock: status 404") {
		t.Errorf("exit %d, %q", code, out)
	}
}

// The first Apple 429 stops the run: no retry, no further Apple request, the missing songs counted.
func TestImport_stopsAtTheFirstAppleRateLimit(t *testing.T) {
	var appleCalls atomic.Int32
	tok := webToken()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /mb/ws/2/recording", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"count":0,"recordings":[]}`)
	})
	mux.HandleFunc("GET /player/us/browse", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<script src="/assets/index~abc123.js"></script>`)
	})
	mux.HandleFunc("GET /player/assets/index~abc123.js", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `const t="%s";`, tok)
	})
	mux.HandleFunc("GET /apple/", func(w http.ResponseWriter, _ *http.Request) {
		appleCalls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	data := t.TempDir()
	_ = os.MkdirAll(filepath.Join(data, "cards"), 0o750)
	_ = os.WriteFile(filepath.Join(data, "cards", "x.csv"), []byte("artist,title,year\nA,T,1990\nB,U,1991\nC,V,1992\n"), 0o600)
	code, out := run(t, endpoints(srv), map[string]string{"BACKEND_CARDS_FILES": "x.csv=X"}, "-data", data, "-workers", "1", "songs")
	if code != 1 || !strings.Contains(out, "stopped at the first Apple 429, 3 of 3 songs still missing") || appleCalls.Load() != 1 {
		t.Errorf("exit %d after %d Apple calls:\n%s", code, appleCalls.Load(), out)
	}
}

func TestImport_sourceErrorsFailTheRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	data := t.TempDir()
	_ = os.MkdirAll(filepath.Join(data, "cards"), 0o750)
	_ = os.WriteFile(filepath.Join(data, "cards", "x.csv"), []byte("artist,title,year\nA,T,1990\nB,U,1991\n"), 0o600)
	ep := Endpoints{MusicBrainz: srv.URL, AppleAPI: srv.URL, ApplePlayer: srv.URL, CoverArt: srv.URL}
	code, out := run(t, ep, map[string]string{"BACKEND_CARDS_FILES": "x.csv=X"}, "-data", data, "songs")
	if code != 1 || !strings.Contains(out, "2 songs failed") {
		t.Errorf("exit %d, %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(data, "songs.jsonl")); err == nil {
		t.Error("a failed run wrote songs.jsonl")
	}
}
