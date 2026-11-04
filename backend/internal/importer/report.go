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

// report is what import-report.md shows: counts and the songs worth a look, artist and title only. resolved are
// the songs as written, overrides applied: every section is built from them.
type report struct {
	sets     map[string]*setStats
	resolved []resolved
	merged   []string
	applied  []string
	unknown  []string
}

func (r report) markdown() []byte {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("# Import report\n\n")
	w("Written by `cmd/import` from `data/cards/*.csv`; rerun to refresh. The card year is the song's year (card sets\n")
	w("always win); MusicBrainz and Apple years are only compared. MusicBrainz: recordings first released by the year\n")
	w("after the card's, the earliest official one wins. Apple, per storefront ch → de → us: ISRC lookup (title and\n")
	w("artist must match), in ch also the MusicBrainz original album by name; the first original-album track wins,\n")
	w("else the first ISRC hit, else an artist + title search in ch and us. Only tracks with a preview count. Songs\n")
	w("without one stay in the dataset (no clip until an override). Last.fm, by the card's artist + title, else the\n")
	w("main artist, else the MusicBrainz credit: the first track found gives the listeners and, by its top tags, the\n")
	w("genres (else the artist's top tags); Rock Anthems and Schlager Fest add their genre, overrides included.\n\n")

	bySet := map[string]*struct{ songs, mb, preview int }{}
	for slug := range r.sets {
		bySet[slug] = &struct{ songs, mb, preview int }{}
	}
	var mb, exact, preview, apple, caa, origin, onLastFM, noGenre int
	// Songs per genre; listeners only of those Last.fm knows (an unknown song's 0 would drag the percentiles down).
	genreSongs, byGenre := map[string]int{}, map[string][]int{}
	for _, x := range r.resolved {
		s := x.Song
		if x.LastFM {
			onLastFM++
		}
		if len(s.Genres) == 0 {
			noGenre++
		}
		for _, g := range s.Genres {
			genreSongs[g]++
			if x.LastFM {
				byGenre[g] = append(byGenre[g], s.Listeners)
			}
		}
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
		{"cards", cardsN}, {"cards without a year (dropped)", noYear}, {"songs", len(r.resolved)},
		{"card spellings merged into one recording", len(r.merged)},
		{"MusicBrainz matched", mb}, {"MusicBrainz matched, no low-confidence note", exact},
		{"Apple track with preview", preview}, {"no Apple preview", len(r.resolved) - preview},
		{"Apple track on the original album", origin},
		{"cover: Apple (original album)", apple}, {"cover: Cover Art Archive", caa},
		{"cover: none", len(r.resolved) - apple - caa},
		{"Last.fm track found", onLastFM}, {"no genre", noGenre},
		{"overrides applied", len(r.applied)},
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

	// Every rule, an empty one included: a rule matching nothing (or everything) shows here.
	w("\n## Categories\n\n| category | songs |\n|---|---:|\n")
	for _, rule := range categoryRules {
		n := 0
		for _, x := range r.resolved {
			if slices.Contains(x.Song.Categories, rule.slug) {
				n++
			}
		}
		w("| %s | %d |\n", rule.slug, n)
	}

	// The listener spread per genre is what the well-known filter for songs without a card is calibrated on: a
	// candidate needs about the 10th percentile of the card songs of its genre.
	w("\n## Genres (Last.fm listeners of the songs Last.fm knows)\n\n| genre | songs | on Last.fm | p10 | p50 |\n" +
		"|---|---:|---:|---:|---:|\n")
	genreIDs := make([]string, 0, len(genreSongs))
	for g := range genreSongs {
		genreIDs = append(genreIDs, g)
	}
	slices.Sort(genreIDs)
	for _, g := range genreIDs {
		ls := byGenre[g]
		w("| %s | %d | %d | %d | %d |\n", g, genreSongs[g], len(ls), percentile(ls, 10), percentile(ls, 50))
	}

	rows := slices.Clone(r.resolved)
	slices.SortFunc(rows, func(a, b resolved) int {
		return strings.Compare(song.Key(a.Artist, a.Title), song.Key(b.Artist, b.Title))
	})
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
	section("Year conflicts: MusicBrainz or Apple more than a year off the song's year (card, or override)",
		"| artist | title | year | MusicBrainz | Apple | Apple on original album |\n|---|---|---:|---:|---:|---|",
		func(x resolved) (string, bool) {
			if withinAYear(x.MBYear, x.Song.Year) && withinAYear(x.AppleYear, x.Song.Year) {
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
	w("\n## Cards merged into another card's recording (%d)\n\n", len(r.merged))
	if len(r.merged) == 0 {
		w("None.\n")
	} else {
		merged := slices.Clone(r.merged)
		slices.Sort(merged)
		w("| artist | title | kept as artist | kept as title |\n|---|---|---|---|\n%s\n", strings.Join(merged, "\n"))
	}
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
	section("Not on Last.fm", "| artist | title | genres (artist tags, sets, overrides) |\n|---|---|---|", func(x resolved) (string, bool) {
		return fmt.Sprintf("| %s | %s | %s |", cell(x.Artist), cell(x.Title), strings.Join(x.Song.Genres, ", ")), !x.LastFM
	})
	if len(r.unknown) > 0 {
		w("\n## Overrides for unknown song ids (%d)\n\n", len(r.unknown))
		for _, id := range r.unknown {
			w("- `%s`\n", id)
		}
	}
	return []byte(b.String())
}

// percentile is the p-th percentile of vs by nearest rank, 0 for none.
func percentile(vs []int, p int) int {
	if len(vs) == 0 {
		return 0
	}
	s := slices.Sorted(slices.Values(vs))
	i := (p*len(s)+99)/100 - 1
	return s[max(i, 0)]
}

// cell keeps a value inside its table cell.
func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
