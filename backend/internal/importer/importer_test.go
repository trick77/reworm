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
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
		"4,Someone,No Year,https://video.example/4,h4,x,\n"
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
			data = append(data, appleSong("dq-comp", "Dancing Queen", "ABBA", "Party Hits", "Various Artists", "1976-01-01", "SEAYD7601020", true))
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
		CoverArt: srv.URL + "/caa"}
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
		"Nobody,Unknown Song,1999\nSomeone,No Year,\n"; string(snap) != want {
		t.Errorf("snapshot:\n%s", snap)
	}
	for _, f := range []string{"essentials.csv", "mega-mix.csv"} {
		b, _ := os.ReadFile(filepath.Join(data, "cards", f))
		if bytes.Contains(b, []byte("video.example")) || bytes.Contains(b, []byte("list")) {
			t.Errorf("%s keeps source data:\n%s", f, b)
		}
	}

	// An override drops the unknown song; one names an id the dataset lacks.
	if err := os.WriteFile(filepath.Join(data, "overrides.json"), []byte(`{"_doc":"x","songs":{
		"nobody--unknown-song":{"exclude":true},
		"joseito-fernandez--guajira-guantanamera":{"apple_id":"999"},
		"missing-id":{"year":1990}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ep.MBEvery = 0
	if code, out := run(t, ep, nil, "-data", data, "-workers", "2", "songs"); code != 0 {
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
	if len(songs) != 3 {
		t.Fatalf("songs:\n%s", b)
	}

	dq := byID["dq-studio"]
	if dq.Year != 1976 || dq.YearSource != "card" || dq.AppleID != "dq-arrival" || dq.Storefront != "ch" ||
		dq.AppleAlbum != "Arrival (Bonus Track Version)" || dq.CoverSrc != "apple" ||
		dq.CoverURL != "https://img.example/dq-arrival/{w}x{h}bb.jpg" || strings.Join(dq.Sets, ",") != "essentials,mega-mix" ||
		dq.ISRC != "SEAYD7601020" || dq.MBID != "dq-studio" {
		t.Errorf("Dancing Queen = %+v", dq)
	}
	br := byID["br-studio"]
	if br.AppleID != "br-hits" || br.Storefront != "de" || br.CoverSrc != "caa" ||
		br.CoverURL != "https://coverartarchive.org/release-group/rg-opera/front-500" {
		t.Errorf("Bohemian Rhapsody = %+v", br)
	}
	gg := byID["joseito-fernandez--guajira-guantanamera"]
	if gg.Year != 1929 || gg.MBID != "" || gg.AppleID != "999" || gg.Storefront != "ch" || gg.CoverSrc != "" {
		t.Errorf("Guantanamera = %+v", gg)
	}
	if bytes.Contains(b, []byte("audio.example")) {
		t.Error("songs.jsonl stores a preview URL")
	}

	rep, err := os.ReadFile(filepath.Join(data, "import-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| cards | 6 |", "| cards without a year (dropped) | 1 |", "| songs | 3 |", "| overrides applied | 2 |",
		"| essentials | 4 | 1 | 2 | 1 | 2 |", "| mega-mix | 2 | 0 | 2 | 2 | 2 |",
		"| Queen | Bohemian Rhapsody | 1975 | 1975 | 1981 |  |",
		"| ABBA | Dancing Queen | 1976, 1977 |",
		"| Joseito Fernandez | Guajira Guantanamera | Apple track found by search (Joseíto Fernández – Guajira Guantanamera) |",
		"## Not found on MusicBrainz (2)", "## No Apple track with a preview (1)", "| Nobody | Unknown Song |",
		"- `missing-id`",
	} {
		if !strings.Contains(string(rep), want) {
			t.Errorf("report lacks %q:\n%s", want, rep)
		}
	}
	if strings.Contains(string(rep), "video.example") || strings.Contains(string(rep), srv.URL) {
		t.Error("report leaks a source URL")
	}

	// A re-run answers from the cache: MusicBrainz is not asked again, the output is the same.
	before := mbHits.Load()
	if code, out := run(t, ep, nil, "-data", data, "songs"); code != 0 {
		t.Fatalf("rerun: exit %d\n%s", code, out)
	}
	again, _ := os.ReadFile(filepath.Join(data, "songs.jsonl"))
	if mbHits.Load() != before || !bytes.Equal(again, b) {
		t.Errorf("rerun: %d new MusicBrainz requests, output changed: %v", mbHits.Load()-before, !bytes.Equal(again, b))
	}
	if _, err := os.Stat(filepath.Join(data, "cache", "apple-token.json")); err != nil {
		t.Errorf("apple token not kept: %v", err)
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
	if code, out := run(t, ep, nil, "-data", data, "songs"); code != 0 {
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
		{[]string{"-data", data, "songs"}, nil, 1, "no card snapshots"},
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
	if code, out := run(t, ep, nil, "-data", data, "songs"); code != 1 || !strings.Contains(out, "overrides") {
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

func TestImport_sourceErrorsFailTheRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	data := t.TempDir()
	_ = os.MkdirAll(filepath.Join(data, "cards"), 0o750)
	_ = os.WriteFile(filepath.Join(data, "cards", "x.csv"), []byte("artist,title,year\nA,T,1990\nB,U,1991\n"), 0o600)
	ep := Endpoints{MusicBrainz: srv.URL, AppleAPI: srv.URL, ApplePlayer: srv.URL, CoverArt: srv.URL}
	code, out := run(t, ep, nil, "-data", data, "songs")
	if code != 1 || !strings.Contains(out, "2 songs failed") {
		t.Errorf("exit %d, %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(data, "songs.jsonl")); err == nil {
		t.Error("a failed run wrote songs.jsonl")
	}
}
