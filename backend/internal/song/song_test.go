package song

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Joseíto Fernández":       "joseito fernandez",
		"  Simon & Garfunkel ":    "simon and garfunkel",
		"Don't Stop Me Now":       "dont stop me now",
		"Ein Bett im Kornfeld!":   "ein bett im kornfeld",
		"Rock'n'Roll – Part 2":    "rocknroll part 2",
		"Straße":                  "strasse",
		"Earth, Wind + Fire":      "earth wind and fire",
		"L.A. Woman":              "la woman",
		"":                        "",
		"Mötley Crüe":             "motley crue",
		"(I Can't Get No) Satisf": "i cant get no satisf",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidYear(t *testing.T) {
	for y, want := range map[int]bool{1799: false, 1800: true, 1976: true, 2100: true, 2101: false, 0: false} {
		if ValidYear(y) != want {
			t.Errorf("ValidYear(%d) != %v", y, want)
		}
	}
}

func TestKeyAndSlug(t *testing.T) {
	if Key("ABBA", "Dancing Queen") != Key("Abba", "dancing  queen!") {
		t.Error("keys differ for the same song")
	}
	if got := Slug("Joseíto Fernández", "Guajira Guantanamera"); got != "joseito-fernandez--guajira-guantanamera" {
		t.Errorf("Slug = %q", got)
	}
}

func TestEncodeDecode_roundTripSortedWithEmptyLists(t *testing.T) {
	songs := []Song{
		{ID: "b", Artist: "B", Title: "T", Year: 1980, YearSource: YearCard, Sets: []string{"essentials"}},
		{ID: "a", Artist: "A", Title: "T", Year: 1976, YearSource: YearCard},
	}
	b, err := Encode(songs)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], `{"id":"a",`) {
		t.Fatalf("not sorted by id:\n%s", b)
	}
	if !strings.Contains(lines[0], `"genres":[],"categories":[],"sets":[]`) {
		t.Errorf("nil lists not written as []: %s", lines[0])
	}
	got, err := Decode(append(b, '\n', '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Sets[0] != "essentials" || got[0].Genres == nil {
		t.Errorf("decoded %+v", got)
	}
}

func TestEncode_duplicateID(t *testing.T) {
	if _, err := Encode([]Song{{ID: "a"}, {ID: "a"}}); err == nil {
		t.Error("duplicate id accepted")
	}
}

func TestDecode_rejectsBadLines(t *testing.T) {
	for _, in := range []string{`{"id":"a"`, `{"id":"a","artist":"A","title":"T"}`} {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("Decode(%s) succeeded", in)
		}
	}
}
