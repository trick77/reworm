// Package song is the dataset's record: what the import writes to data/songs.jsonl and the server upserts at boot.
package song

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Song is one line of songs.jsonl, the songs table's columns. Field order is the file's key order.
type Song struct {
	// ID is stable across re-imports: the earliest matching MusicBrainz recording's MBID, else Slug(artist, title).
	ID         string   `json:"id"`
	MBID       string   `json:"mbid"`
	Artist     string   `json:"artist"`
	Title      string   `json:"title"`
	Year       int      `json:"year"`
	YearSource string   `json:"year_source"`
	ISRC       string   `json:"isrc"`
	AppleID    string   `json:"apple_id"`
	Storefront string   `json:"storefront"`
	AppleAlbum string   `json:"apple_album"`
	CoverURL   string   `json:"cover_url"`
	CoverSrc   string   `json:"cover_source"`
	Genres     []string `json:"genres"`
	Categories []string `json:"categories"`
	Sets       []string `json:"sets"`
	Listeners  int      `json:"listeners"`
}

// Year sources.
const (
	YearCard     = "card"
	YearOverride = "override"
)

// Cover sources; "" is none (the UI draws a record).
const (
	CoverApple = "apple"
	CoverCAA   = "caa"
)

// Encode writes songs as JSON lines sorted by id, nil lists as [], so a re-import diffs only what changed.
func Encode(songs []Song) ([]byte, error) {
	sorted := slices.Clone(songs)
	slices.SortFunc(sorted, func(a, b Song) int { return strings.Compare(a.ID, b.ID) })
	var buf bytes.Buffer
	for i, s := range sorted {
		if i > 0 && s.ID == sorted[i-1].ID {
			return nil, fmt.Errorf("songs: duplicate id %q", s.ID)
		}
		s.Genres, s.Categories, s.Sets = orEmpty(s.Genres), orEmpty(s.Categories), orEmpty(s.Sets)
		b, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// Decode reads JSON lines; blank lines are skipped. A line without id, artist, title or year is an error: the import
// never writes one.
func Decode(b []byte) ([]Song, error) {
	var out []Song
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var s Song
		if err := json.Unmarshal(line, &s); err != nil {
			return nil, fmt.Errorf("songs line %d: %w", n, err)
		}
		if s.ID == "" || s.Artist == "" || s.Title == "" || s.Year == 0 {
			return nil, fmt.Errorf("songs line %d: id, artist, title and year are required", n)
		}
		s.Genres, s.Categories, s.Sets = orEmpty(s.Genres), orEmpty(s.Categories), orEmpty(s.Sets)
		out = append(out, s)
	}
	return out, sc.Err()
}

func orEmpty(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

// fold maps the Latin letters with diacritics the card sets and MusicBrainz use to their base letters, so
// "Joseíto Fernández" and "Joseito Fernandez" compare equal.
var fold = func() map[rune]string {
	m := map[rune]string{}
	for base, variants := range map[string]string{
		"a": "àáâãäåāăą", "c": "çćčĉ", "d": "ďđ", "e": "èéêëēėęěĕ", "g": "ğĝ", "i": "ìíîïīįı", "l": "łľĺ",
		"n": "ñńňņ", "o": "òóôõöøōő", "r": "řŕ", "s": "śšşș", "t": "ťţț", "u": "ùúûüūůűų", "y": "ýÿ",
		"z": "źżž", "ss": "ß", "ae": "æ", "oe": "œ",
	} {
		for _, r := range variants {
			m[r] = base
		}
	}
	return m
}()

// Normalize lowercases s, folds diacritics, turns "&" into "and", drops every other non-letter/digit and collapses
// the spaces: the comparison key for artists and titles.
func Normalize(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		switch {
		case fold[r] != "":
			b.WriteString(fold[r])
			space = false
		case r == '&' || r == '+':
			if b.Len() > 0 && !space {
				b.WriteByte(' ')
			}
			b.WriteString("and ")
			space = true
		case r == '\'' || r == '’' || r == '.':
			// Kept together: "don't", "L.A." → dont, la.
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		default:
			if b.Len() > 0 && !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// Key is the dedupe key of a card: normalized artist and title.
func Key(artist, title string) string { return Normalize(artist) + "|" + Normalize(title) }

// Slug is the id of a song MusicBrainz does not know: normalized artist and title, dashes for spaces, "--" between.
func Slug(artist, title string) string {
	dash := func(s string) string { return strings.ReplaceAll(Normalize(s), " ", "-") }
	return dash(artist) + "--" + dash(title)
}
