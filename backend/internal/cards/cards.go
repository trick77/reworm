// Package cards snapshots the community card lists the song database starts from: each source CSV is reduced to
// artist, title and year and committed as data/cards/<set-slug>.csv. Where the lists come from and which file is
// which set lives only in the uncommitted local config (plan D10): nothing of the source (URL, file names, video
// links, hashes) is kept.
package cards

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/trick77/ship/kit/fsx"
)

// Card is one card: what the snapshot keeps. Year is 0 when the source had none.
type Card struct {
	Artist string
	Title  string
	Year   int
}

// Source maps one source file to one of our sets.
type Source struct {
	File string
	Set  string
}

// Slug is the set's file name and id: "Mega Mix" → "mega-mix".
func (s Source) Slug() string { return Slug(s.Set) }

// Slug lowercases name and joins its words with dashes.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// ParseSources reads "file=Set Name;file=Set Name".
func ParseSources(spec string) ([]Source, error) {
	var out []Source
	seen := map[string]bool{}
	for _, part := range strings.Split(spec, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		file, set, ok := strings.Cut(part, "=")
		file, set = strings.TrimSpace(file), strings.TrimSpace(set)
		if !ok || file == "" || Slug(set) == "" {
			return nil, fmt.Errorf("cards: entry %d of the file list is not file=Set Name", len(out)+1)
		}
		s := Source{File: file, Set: set}
		if seen[s.Slug()] {
			return nil, fmt.Errorf("cards: set %q listed twice", set)
		}
		seen[s.Slug()] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errors.New("cards: the file list is empty")
	}
	return out, nil
}

// Fetch downloads one source file from base and parses it.
func Fetch(ctx context.Context, client *http.Client, base string, src Source) ([]Card, error) {
	u := strings.TrimRight(base, "/") + "/" + url.PathEscape(src.File)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("cards %s: %w", src.Set, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		// The error names the source's host and URL; the set name is all a log may show.
		return nil, fmt.Errorf("cards %s: download failed (network; check BACKEND_CARDS_BASE_URL)", src.Set)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cards %s: status %d", src.Set, resp.StatusCode)
	}
	cs, err := ParseSource(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cards %s: %w", src.Set, err)
	}
	return cs, nil
}

// ParseSource reads a source CSV by its header: the Artist, Title and Year columns, every other one dropped. Rows
// without artist or title are skipped; an unreadable year is 0.
func ParseSource(r io.Reader) ([]Card, error) {
	rd := csv.NewReader(r)
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	header, err := rd.Read()
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\xef\xbb\xbf")))] = i
	}
	ia, okA := col["artist"]
	it, okT := col["title"]
	iy, okY := col["year"]
	if !okA || !okT || !okY {
		return nil, errors.New("header lacks artist, title or year")
	}
	var out []Card
	for {
		rec, err := rd.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		field := func(i int) string {
			if i < len(rec) {
				return strings.Join(strings.Fields(rec[i]), " ")
			}
			return ""
		}
		c := Card{Artist: field(ia), Title: field(it)}
		if c.Artist == "" || c.Title == "" {
			continue
		}
		if y, err := strconv.Atoi(field(iy)); err == nil && y >= 1800 && y <= 2100 {
			c.Year = y
		}
		out = append(out, c)
	}
}

// snapshotHeader is the committed snapshot's header.
var snapshotHeader = []string{"artist", "title", "year"}

// Encode writes a snapshot CSV, in source order.
func Encode(cs []Card) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write(snapshotHeader)
	for _, c := range cs {
		y := ""
		if c.Year != 0 {
			y = strconv.Itoa(c.Year)
		}
		_ = w.Write([]string{c.Artist, c.Title, y})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// WriteSnapshot writes dir/<slug>.csv.
func WriteSnapshot(dir, slug string, cs []Card) error {
	b, err := Encode(cs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(filepath.Join(dir, slug+".csv"), b, 0o644)
}

// Set is one committed snapshot.
type Set struct {
	Slug  string
	Cards []Card
}

// ReadSnapshots reads every dir/*.csv, sorted by slug.
func ReadSnapshots(dir string) ([]Set, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	var out []Set
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // the repo's own snapshot
		if err != nil {
			return nil, err
		}
		cs, err := ParseSource(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		out = append(out, Set{Slug: strings.TrimSuffix(filepath.Base(f), ".csv"), Cards: cs})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no card snapshots in %s (run the cards step first)", dir)
	}
	return out, nil
}
