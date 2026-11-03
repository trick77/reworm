package importer

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/trick77/reworm/internal/song"
)

// setStats counts one set's cards.
type setStats struct {
	Cards, NoYear int
}

// report is what import-report.md shows: counts and the songs worth a look, artist and title only.
type report struct {
	sets     map[string]*setStats
	resolved []resolved
	songs    []song.Song
	merged   int
	applied  []string
	unknown  []string
}

func (r report) markdown() []byte {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("# Import report\n\n")
	w("Written by `cmd/import` from `data/cards/*.csv`; rerun to refresh. The card year is the song's year (card sets\n")
	w("always win); MusicBrainz and Apple years are only compared. Apple: ISRC lookup in ch → de → us first, then an\n")
	w("artist + title search in the same order; only tracks with a preview count, the original-album track first.\n")
	w("Songs without a preview stay in the dataset (no clip to play until fixed by an override).\n\n")

	bySet := map[string]*struct{ songs, mb, preview int }{}
	for slug := range r.sets {
		bySet[slug] = &struct{ songs, mb, preview int }{}
	}
	var mb, exact, preview, apple, caa, origin int
	for _, s := range r.songs {
		if s.MBID != "" {
			mb++
		}
		if s.AppleID != "" {
			preview++
		}
		switch s.CoverSrc {
		case song.CoverApple:
			apple++
		case song.CoverCAA:
			caa++
		}
		for _, set := range s.Sets {
			if st := bySet[set]; st != nil {
				st.songs++
				if s.MBID != "" {
					st.mb++
				}
				if s.AppleID != "" {
					st.preview++
				}
			}
		}
	}
	for _, x := range r.resolved {
		if x.MBMatched && len(x.Low) == 0 {
			exact++
		}
		if x.AppleOriginal {
			origin++
		}
	}
	cardsN, noYear := 0, 0
	for _, st := range r.sets {
		cardsN += st.Cards
		noYear += st.NoYear
	}

	w("## Totals\n\n| | count |\n|---|---:|\n")
	for _, row := range []struct {
		k string
		v int
	}{
		{"cards", cardsN}, {"cards without a year (dropped)", noYear}, {"songs", len(r.songs)},
		{"card spellings merged into one recording", r.merged},
		{"MusicBrainz matched", mb}, {"MusicBrainz matched, no low-confidence note", exact},
		{"Apple track with preview", preview}, {"no Apple preview", len(r.songs) - preview},
		{"Apple track on the original album", origin},
		{"cover: Apple (original album)", apple}, {"cover: Cover Art Archive", caa},
		{"cover: none", len(r.songs) - apple - caa}, {"overrides applied", len(r.applied)},
	} {
		w("| %s | %d |\n", row.k, row.v)
	}

	w("\n## Sets\n\n| set | cards | no year | songs | MusicBrainz | Apple preview |\n|---|---:|---:|---:|---:|---:|\n")
	slugs := make([]string, 0, len(r.sets))
	for s := range r.sets {
		slugs = append(slugs, s)
	}
	slices.Sort(slugs)
	for _, slug := range slugs {
		st, n := r.sets[slug], bySet[slug]
		w("| %s | %d | %d | %d | %d | %d |\n", slug, st.Cards, st.NoYear, n.songs, n.mb, n.preview)
	}

	rows := slices.Clone(r.resolved)
	slices.SortFunc(rows, func(a, b resolved) int {
		return strings.Compare(song.Key(a.Artist, a.Title), song.Key(b.Artist, b.Title))
	})
	off := func(y, card int) bool { return y != 0 && (y-card > 1 || card-y > 1) }
	section := func(title, header string, line func(x resolved) (string, bool)) {
		var lines []string
		for _, x := range rows {
			if l, ok := line(x); ok {
				lines = append(lines, l)
			}
		}
		w("\n## %s (%d)\n\n", title, len(lines))
		if len(lines) == 0 {
			w("None.\n")
			return
		}
		w("%s\n", header)
		for _, l := range lines {
			w("%s\n", l)
		}
	}
	yr := func(y int) string {
		if y == 0 {
			return "–"
		}
		return strconv.Itoa(y)
	}
	section("Year conflicts: MusicBrainz or Apple more than a year off the card",
		"| artist | title | card | MusicBrainz | Apple | Apple on original album |\n|---|---|---:|---:|---:|---|",
		func(x resolved) (string, bool) {
			if !off(x.MBYear, x.Song.Year) && !off(x.AppleYear, x.Song.Year) {
				return "", false
			}
			orig := ""
			if x.AppleOriginal {
				orig = "yes"
			}
			return fmt.Sprintf("| %s | %s | %d | %s | %s | %s |", cell(x.Artist), cell(x.Title), x.Song.Year,
				yr(x.MBYear), yr(x.AppleYear), orig), true
		})
	section("Card years that differ between sets (earliest kept)", "| artist | title | years |\n|---|---|---|",
		func(x resolved) (string, bool) {
			if len(x.Years) < 2 {
				return "", false
			}
			ys := make([]string, len(x.Years))
			for i, y := range x.Years {
				ys[i] = strconv.Itoa(y)
			}
			return fmt.Sprintf("| %s | %s | %s |", cell(x.Artist), cell(x.Title), strings.Join(ys, ", ")), true
		})
	section("Low-confidence matches", "| artist | title | why |\n|---|---|---|", func(x resolved) (string, bool) {
		if len(x.Low) == 0 {
			return "", false
		}
		return fmt.Sprintf("| %s | %s | %s |", cell(x.Artist), cell(x.Title), cell(strings.Join(x.Low, "; "))), true
	})
	section("Not found on MusicBrainz", "| artist | title |\n|---|---|", func(x resolved) (string, bool) {
		return fmt.Sprintf("| %s | %s |", cell(x.Artist), cell(x.Title)), !x.MBMatched
	})
	section("No Apple track with a preview", "| artist | title |\n|---|---|", func(x resolved) (string, bool) {
		return fmt.Sprintf("| %s | %s |", cell(x.Artist), cell(x.Title)), x.Song.AppleID == ""
	})
	if len(r.unknown) > 0 {
		w("\n## Overrides for unknown song ids (%d)\n\n", len(r.unknown))
		for _, id := range r.unknown {
			w("- `%s`\n", id)
		}
	}
	return []byte(b.String())
}

// cell keeps a value inside its table cell.
func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
