package cards

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const source = "\xef\xbb\xbfCard#,Artist,Title,URL,Hashed Info,Youtube-Title,Year\n" +
	"1,ABBA,Dancing Queen,https://video.example/watch?v=1,abc123,ABBA - Dancing Queen (Official),1976\n" +
	"2,\"Simon & Garfunkel\",\"Mrs. Robinson\",https://video.example/2,def,x,1968\n" +
	"3,  Joseíto   Fernández ,Guajira Guantanamera,,,,1929\n" +
	"4,,No Artist,,,,2000\n" +
	"5,Someone,Yearless,,,,unknown\n" +
	"6,Short Row,Title\n"

func TestParseSourceKeepsArtistTitleYearOnly(t *testing.T) {
	got, err := ParseSource(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	want := []Card{
		{"ABBA", "Dancing Queen", 1976}, {"Simon & Garfunkel", "Mrs. Robinson", 1968},
		{"Joseíto Fernández", "Guajira Guantanamera", 1929}, {"Someone", "Yearless", 0}, {"Short Row", "Title", 0},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v", got)
	}
	b, err := Encode(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "video.example") || strings.Contains(string(b), "abc123") {
		t.Errorf("snapshot keeps source columns:\n%s", b)
	}
	if !strings.HasPrefix(string(b), "artist,title,year\nABBA,Dancing Queen,1976\n") || !strings.Contains(string(b), "Someone,Yearless,\n") {
		t.Errorf("snapshot:\n%s", b)
	}
}

func TestParseSourceErrors(t *testing.T) {
	for _, in := range []string{"", "Card#,Artist,Title\n1,A,T\n"} {
		if _, err := ParseSource(strings.NewReader(in)); err == nil {
			t.Errorf("ParseSource(%q) succeeded", in)
		}
	}
}

func TestParseSources(t *testing.T) {
	got, err := ParseSources(" one.csv=Essentials ; two file.csv=Mega Mix;;")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].File != "two file.csv" || got[1].Slug() != "mega-mix" || got[0].Slug() != "essentials" {
		t.Errorf("got %+v", got)
	}
	for _, bad := range []string{"", "x.csv", "=Name", "a.csv=!!", "a.csv=Rock;b.csv=rock"} {
		if _, err := ParseSources(bad); err == nil {
			t.Errorf("ParseSources(%q) succeeded", bad)
		}
	}
	if Slug("  Rock Anthems! ") != "rock-anthems" || Slug("Radio Gold 2") != "radio-gold-2" {
		t.Error("Slug")
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lists/two file.csv" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(source))
	}))
	defer srv.Close()
	got, err := Fetch(context.Background(), srv.Client(), srv.URL+"/lists/", Source{File: "two file.csv", Set: "Mega Mix"})
	if err != nil || len(got) != 5 {
		t.Fatalf("%v, %v", got, err)
	}
	if _, err := Fetch(context.Background(), srv.Client(), srv.URL, Source{File: "x.csv", Set: "Rock"}); err == nil ||
		!strings.Contains(err.Error(), "cards Rock: status 404") {
		t.Errorf("404: %v", err)
	}
	srv.Close()
	_, err = Fetch(context.Background(), srv.Client(), srv.URL, Source{File: "secret-name.csv", Set: "Rock"})
	if err == nil || strings.Contains(err.Error(), "secret-name") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("download error leaks the source: %v", err)
	}
	if _, err := Fetch(context.Background(), srv.Client(), "://bad", Source{File: "x", Set: "Rock"}); err == nil {
		t.Error("bad base: no error")
	}
}

func TestFetch_badCSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("a,b\n")) }))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.Client(), srv.URL, Source{File: "x", Set: "Rock"}); err == nil {
		t.Error("bad csv: no error")
	}
}

func TestSnapshotsRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cards")
	if _, err := ReadSnapshots(dir); err == nil {
		t.Error("empty dir: no error")
	}
	if err := WriteSnapshot(dir, "mega-mix", []Card{{"B", "T", 1980}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteSnapshot(dir, "essentials", []Card{{"A", "T, with comma", 1976}, {"C", "Q", 0}}); err != nil {
		t.Fatal(err)
	}
	sets, err := ReadSnapshots(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].Slug != "essentials" || sets[0].Cards[0].Title != "T, with comma" || sets[0].Cards[1].Year != 0 || sets[1].Slug != "mega-mix" {
		t.Errorf("sets %+v", sets)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.csv"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSnapshots(dir); err == nil {
		t.Error("bad snapshot: no error")
	}
	blocker := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(blocker, nil, 0o600)
	if err := WriteSnapshot(blocker, "x", nil); err == nil {
		t.Error("unwritable: no error")
	}
}
