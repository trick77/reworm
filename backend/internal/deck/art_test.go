package deck

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// The UI picks a deck's picture by slug (ui/src/components/DeckArt.tsx, BY_ID). A slug added or renamed here and
// not there silently falls back to a generic note, so both lists must hold the same sets and categories.
func TestDeckArtCoversSetsAndCategories(t *testing.T) {
	src, err := os.ReadFile("../../../ui/src/components/DeckArt.tsx")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)const BY_ID: Record<string, Art> = \{(.*?)\n\};`).FindSubmatch(src)
	if block == nil {
		t.Fatal("BY_ID not found in DeckArt.tsx")
	}
	var ui []string
	for _, m := range regexp.MustCompile(`(?m)^\s*"?([a-z-]+)"?:`).FindAllSubmatch(block[1], -1) {
		ui = append(ui, string(m[1]))
	}
	var want []string
	for _, k := range known {
		if k.id == "general" { // genres from here on: no picture of their own
			break
		}
		want = append(want, k.id)
	}
	slices.Sort(ui)
	slices.Sort(want)
	if !slices.Equal(ui, want) {
		t.Fatalf("DeckArt BY_ID slugs\n got %v\nwant %v", ui, want)
	}
}
