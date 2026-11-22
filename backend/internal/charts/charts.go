// Package charts snapshots the charts the song database adds to the card sets: per year the Swiss, German and
// Austrian number-one singles and year-end top 10 (de.wikipedia), the Billboard Year-End Hot 100 and the UK top-ten
// singles (en.wikipedia), committed as data/charts/<year>.csv. The chart year only bounds a song's year: the import dates
// it by MusicBrainz. hitparade.ch itself sits behind a Cloudflare challenge (403, 2026-11-22).
package charts

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/trick77/ship/kit/fsx"

	"github.com/trick77/reworm/internal/song"
)

// Sources: the chart a row comes from.
const (
	CH        = "ch"
	DE        = "de"
	AT        = "at"
	Billboard = "billboard"
	// UK is every single in the UK top ten of the year: the number-one and year-end lists alone gave 31 songs
	// released 2025 (2026-11-22), the US list adds mostly country hardly known here.
	UK = "uk"
)

// API endpoints of the two wikis.
const (
	DeWiki = "https://de.wikipedia.org/w/api.php"
	EnWiki = "https://en.wikipedia.org/w/api.php"
)

// Entry is one chart hit as the page writes it.
type Entry struct {
	Artist, Title, Source string
}

// Row is a snapshot line: an entry and its chart year.
type Row struct {
	Entry
	Year int
}

// Page is one chart list of a year.
type Page struct {
	Source string
	// Wiki is DeWiki or EnWiki; Title the article.
	Wiki, Title string
}

// Pages are the five lists of year. The Swiss one is "in der Schweiz", the others take no article.
func Pages(year int) []Page {
	de := func(src, country string) Page {
		return Page{Source: src, Wiki: DeWiki, Title: fmt.Sprintf("Liste der Nummer-eins-Hits in %s (%d)", country, year)}
	}
	return []Page{
		de(CH, "der Schweiz"), de(DE, "Deutschland"), de(AT, "Österreich"),
		{Source: Billboard, Wiki: EnWiki, Title: fmt.Sprintf("Billboard Year-End Hot 100 singles of %d", year)},
		{Source: UK, Wiki: EnWiki, Title: fmt.Sprintf("List of UK top-ten singles in %d", year)},
	}
}

// Years reads "2023-2025" or "2024".
func Years(spec string) ([]int, error) {
	from, to, isRange := strings.Cut(strings.TrimSpace(spec), "-")
	if !isRange {
		to = from
	}
	a, errA := strconv.Atoi(from)
	b, errB := strconv.Atoi(to)
	if errA != nil || errB != nil || a > b || a < 1950 || b > 2100 {
		return nil, fmt.Errorf("charts: years %q is not YYYY or YYYY-YYYY", spec)
	}
	var out []int
	for y := a; y <= b; y++ {
		out = append(out, y)
	}
	return out, nil
}

// Fetch returns the wikitext of the article title from the MediaWiki API at wiki. Wikimedia wants a User-Agent
// naming the client and how to reach its maker.
func Fetch(ctx context.Context, client *http.Client, userAgent, wiki, title string) (string, error) {
	q := url.Values{"action": {"parse"}, "page": {title}, "prop": {"wikitext"}, "format": {"json"},
		"formatversion": {"2"}, "redirects": {"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wiki+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("charts %s: %w", title, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Wikimedia answers a burst with 429 (ten requests back to back, 2026-11-22).
		return "", fmt.Errorf("charts %s: status %d", title, resp.StatusCode)
	}
	var body struct {
		Parse struct {
			Wikitext string `json:"wikitext"`
		} `json:"parse"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("charts %s: %w", title, err)
	}
	if body.Error.Code != "" {
		return "", fmt.Errorf("charts %s: %s", title, body.Error.Code)
	}
	return body.Parse.Wikitext, nil
}

// Parse reads the hits of a page of source: a Billboard table, the UK top-ten table, else a de.wikipedia
// number-one list.
func Parse(source, wikitext string) []Entry {
	switch source {
	case Billboard:
		return parseTable(wikitext)
	case UK:
		return parseQuoted(wikitext, "Top-ten singles", UK)
	}
	return parseDE(source, wikitext)
}

var quoted = regexp.MustCompile(`^"([^"]+)"`)

// parseQuoted reads the rows of the tables in section: the single is the cell in quotes, its artist the next
// cell; an artist cell spanning rows fills the rows after it that lack one (their cell after the single is a
// number, the peak).
func parseQuoted(text, section, source string) []Entry {
	var out []Entry
	var cells []cell
	var span struct {
		text string
		left int
	}
	flush := func() {
		defer func() { cells = nil }()
		i := slices.IndexFunc(cells, func(c cell) bool { return quoted.MatchString(clean(c.text)) })
		if i < 0 {
			return
		}
		title := quoted.FindStringSubmatch(clean(cells[i].text))[1]
		artist := ""
		if i+1 < len(cells) {
			if a := cells[i+1]; !isNumber(clean(a.text)) {
				artist = clean(a.text)
				if a.rows > 1 {
					span.text, span.left = artist, a.rows-1
				}
			}
		}
		if artist == "" && span.left > 0 {
			artist = span.text
			span.left--
		}
		if e := (Entry{Artist: artist, Title: strings.TrimSpace(title), Source: source}); e.Artist != "" && e.Title != "" {
			out = append(out, e)
		}
	}
	in := false
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if m := heading.FindStringSubmatch(line); m != nil {
			flush()
			in = m[1] == section
			continue
		}
		switch {
		case !in:
		case strings.HasPrefix(line, "|-"), strings.HasPrefix(line, "|}"):
			flush()
		case strings.HasPrefix(line, "{|"), strings.HasPrefix(line, "|+"), strings.HasPrefix(line, "!"):
		case strings.HasPrefix(line, "|"):
			for c := range strings.SplitSeq(strings.TrimPrefix(line, "|"), "||") {
				text, rows := cellText(c)
				cells = append(cells, cell{text, rows})
			}
		}
	}
	return dedupe(out)
}

type cell struct {
	text string
	rows int
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

var heading = regexp.MustCompile(`^==\s*([^=]+?)\s*==\s*$`)

// parseDE reads the singles section's number ones ({{Nummer-eins-Hits Zeile}}: Titel, Interpret) and the year-end
// list ({{Jahreshitparaden Zeile}}: Single, InterpretS); the albums are left out.
func parseDE(source, text string) []Entry {
	var out []Entry
	section := ""
	var row map[string]string
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if m := heading.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		switch {
		case strings.HasPrefix(line, "{{Nummer-eins-Hits Zeile") && section == "Singles",
			strings.HasPrefix(line, "{{Jahreshitparaden Zeile"):
			row = map[string]string{}
		case row != nil && line == "}}":
			title, artist := row["Titel"], row["Interpret"]
			if title == "" {
				title, artist = row["Single"], row["InterpretS"]
			}
			if e := (Entry{Artist: clean(artist), Title: unquote(clean(title)), Source: source}); e.Artist != "" && e.Title != "" {
				out = append(out, e)
			}
			row = nil
		case row != nil && strings.HasPrefix(line, "|"):
			if k, v, ok := strings.Cut(strings.TrimPrefix(line, "|"), "="); ok {
				row[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	return dedupe(out)
}

// parseTable reads a wikitable's rows: the last two cells are title and artist(s). An artist cell spanning rows
// (`rowspan="2"| [[SZA]]`, Billboard 2024 #19 and #20) fills the following rows that lack one.
func parseTable(text string) []Entry {
	var out []Entry
	var cells []string
	var span struct {
		text string
		left int
	}
	flush := func() {
		if len(cells) == 2 && span.left > 0 {
			cells = append(cells, span.text)
			span.left--
		}
		if len(cells) >= 3 {
			e := Entry{Artist: clean(cells[len(cells)-1]), Title: unquote(clean(cells[len(cells)-2])), Source: Billboard}
			if e.Artist != "" && e.Title != "" {
				out = append(out, e)
			}
		}
		cells = nil
	}
	in := false
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "{|"):
			in = true
		case !in:
		case strings.HasPrefix(line, "|}"):
			flush()
			in = false
		case strings.HasPrefix(line, "|-"):
			flush()
		case strings.HasPrefix(line, "|+"), strings.HasPrefix(line, "!"):
		case strings.HasPrefix(line, "|"):
			for c := range strings.SplitSeq(strings.TrimPrefix(line, "|"), "||") {
				text, rows := cellText(c)
				cells = append(cells, text)
				if rows > 1 && len(cells) >= 3 {
					span.text, span.left = text, rows-1
				}
			}
		}
	}
	return dedupe(out)
}

var rowspan = regexp.MustCompile(`rowspan\s*=\s*"?(\d+)`)

// cellText drops a cell's attributes (`scope="row" | 1`; an attribute part holds a '=' outside links) and says
// how many rows the cell spans.
func cellText(c string) (string, int) {
	if i := strings.Index(c, "|"); i >= 0 && !strings.Contains(c[:i], "[[") && !strings.Contains(c[:i], "{{") && strings.Contains(c[:i], "=") {
		rows := 1
		if m := rowspan.FindStringSubmatch(c[:i]); m != nil {
			rows, _ = strconv.Atoi(m[1])
		}
		return strings.TrimSpace(c[i+1:]), rows
	}
	return strings.TrimSpace(c), 1
}

var (
	comment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	refPair  = regexp.MustCompile(`(?s)<ref[^>/]*>.*?</ref>`)
	refSolo  = regexp.MustCompile(`<ref[^>]*/>`)
	tags     = regexp.MustCompile(`<[^>]+>`)
	langTpl  = regexp.MustCompile(`\{\{(?:lang|nowrap|Lang|Nowrap)\|(?:[a-z-]+\|)?([^{}|]*)\}\}`)
	sortTpl  = regexp.MustCompile(`\{\{[Ss]ort\|[^{}|]*\|([^{}]*)\}\}`)
	template = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	link     = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]|]*)\]\]`)
)

// clean is a field's plain text: comments, refs, markup and templates gone, links as their label.
func clean(s string) string {
	s = comment.ReplaceAllString(s, "")
	s = refPair.ReplaceAllString(s, "")
	s = refSolo.ReplaceAllString(s, "")
	s = langTpl.ReplaceAllString(s, "$1")
	s = sortTpl.ReplaceAllString(s, "$1")
	for template.MatchString(s) {
		s = template.ReplaceAllString(s, "")
	}
	s = link.ReplaceAllString(s, "$1")
	s = tags.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "''", "")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// unquote drops the quotes a title is written in ("Luther").
func unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// dedupe keeps the first of each song (song.Key) per source: a number one holds for weeks, in several runs.
func dedupe(es []Entry) []Entry {
	seen := map[string]bool{}
	return slices.DeleteFunc(es, func(e Entry) bool {
		k := e.Source + "|" + song.Key(e.Artist, e.Title)
		if seen[k] {
			return true
		}
		seen[k] = true
		return false
	})
}

var header = []string{"artist", "title", "year", "source"}

// WriteYear writes dir/<year>.csv, one row per song and source, in page order.
func WriteYear(dir string, year int, es []Entry) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write(header)
	for _, e := range dedupe(slices.Clone(es)) {
		_ = w.Write([]string{e.Artist, e.Title, strconv.Itoa(year), e.Source})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(filepath.Join(dir, strconv.Itoa(year)+".csv"), buf.Bytes(), 0o644)
}

// ReadAll reads every dir/<year>.csv in year order; none when dir does not exist.
func ReadAll(dir string) ([]Row, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	var out []Row
	for _, f := range files {
		rows, err := readFile(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		out = append(out, rows...)
	}
	return out, nil
}

func readFile(path string) ([]Row, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the repo's own snapshot
	if err != nil {
		return nil, err
	}
	recs, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 || !slices.Equal(recs[0], header) {
		return nil, errors.New("header is not artist,title,year,source")
	}
	out := make([]Row, 0, len(recs)-1)
	for _, r := range recs[1:] {
		y, err := strconv.Atoi(r[2])
		if err != nil || !song.ValidYear(y) {
			return nil, fmt.Errorf("year %q", r[2])
		}
		out = append(out, Row{Entry: Entry{Artist: r[0], Title: r[1], Source: r[3]}, Year: y})
	}
	return out, nil
}
