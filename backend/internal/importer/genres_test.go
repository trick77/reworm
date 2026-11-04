package importer

import (
	"slices"
	"testing"

	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/lastfm"
)

func tags(names ...string) []lastfm.Tag {
	out := make([]lastfm.Tag, len(names))
	for i, n := range names {
		out[i] = lastfm.Tag{Name: n}
	}
	return out
}

func weighed(name string, count int) lastfm.Tag { return lastfm.Tag{Name: name, Count: count} }

func TestGenres(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sets          []string
		track, artist []lastfm.Tag
		want          []string
	}{
		// Dancing Queen's tags (2026-11-04): decade and artist tags are no genre.
		{"track tags", nil, tags("Disco", "pop", "70s", "abba", "dance"), nil, []string{"dance", "disco-funk", "pop"}},
		{"spellings", nil, tags("Hip-Hop", "R&B", "hip_hop", "Italo  Disco"), nil, []string{"hip-hop", "italo", "rnb-soul"}},
		{"at most three", nil, tags("pop", "rock", "soul", "funk", "reggae"), nil, []string{"pop", "rnb-soul", "rock"}},
		{"only the top five track tags", nil, tags("a", "b", "c", "d", "e", "reggae"), nil, nil},
		// Udo Lindenberg's track has no tags; his artist's do, with weights.
		{"artist fallback", nil, nil, []lastfm.Tag{weighed("Deutschrock", 100), weighed("german", 81), weighed("rock", 48),
			weighed("singer-songwriter", 36), weighed("pop", 3)}, []string{"rock"}},
		{"artist only without track genres", nil, tags("pop"), []lastfm.Tag{weighed("metal", 100)}, []string{"pop"}},
		{"set genre", []string{"schlager-fest", "essentials"}, tags("pop"), nil, []string{"pop", "schlager"}},
		{"nothing", nil, nil, nil, nil},
	} {
		if got := genres(tc.sets, tc.track, tc.artist); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Every genre the import writes has a display name and a place in the list; "general" is not one of them.
func TestGenres_known(t *testing.T) {
	for g := range genreTags {
		if !deck.Known(g) || g == deck.General {
			t.Errorf("genre %q: known %v", g, deck.Known(g))
		}
	}
	for s, g := range setGenres {
		if !deck.Known(s) || !knownGenre(g) {
			t.Errorf("set genre %s → %s", s, g)
		}
	}
	seen := map[string]string{}
	for g, ts := range genreTags {
		for _, tg := range ts {
			if tagKey(tg) != tg {
				t.Errorf("%s: tag %q is not in tagKey form", g, tg)
			}
			if o, dup := seen[tg]; dup {
				t.Errorf("tag %q maps to %s and %s", tg, o, g)
			}
			seen[tg] = g
		}
	}
}
