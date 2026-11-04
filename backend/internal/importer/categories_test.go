package importer

import (
	"slices"
	"testing"

	"github.com/trick77/reworm/internal/song"
)

func TestCategories_genAlphaFrom2016WithoutUpperBound(t *testing.T) {
	for _, c := range []struct {
		year int
		want []string
	}{
		{1976, []string{}},
		{2015, []string{}},
		{2016, []string{"gen-alpha"}},
		{2024, []string{"gen-alpha"}},
		{2031, []string{"gen-alpha"}},
	} {
		if got := categories(song.Song{Year: c.year}); !slices.Equal(got, c.want) {
			t.Errorf("categories(%d) = %v, want %v", c.year, got, c.want)
		}
	}
}

// The rules own the field: a stale category from a previous dataset (offline run) goes.
func TestCategorize_replacesWhatTheSongHad(t *testing.T) {
	ss := []song.Song{{Year: 1999, Categories: []string{"gen-alpha"}}, {Year: 2019}}
	categorize(ss)
	if len(ss[0].Categories) != 0 || !slices.Equal(ss[1].Categories, []string{"gen-alpha"}) {
		t.Errorf("categorize = %+v", ss)
	}
}
