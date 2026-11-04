// Package musicbrainz searches MusicBrainz recordings (import only) and picks a card's song among them: the earliest
// matching recording, its ISRCs and the release groups for the original-album match and the cover. MusicBrainz asks
// for at most one request per second and a User-Agent naming the app and a contact (plan D8); the caller's transport
// spaces the requests, this client retries the 503 it answers when they come too fast.
package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/trick77/ship/kit/run"

	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/song"
)

// BaseURL is the web service.
const BaseURL = "https://musicbrainz.org"

const (
	// searchLimit is the API's maximum: a famous song has ~100 recordings (live, remixes, DJ mixes) and the
	// original is not always in the first 25.
	searchLimit = 100
	maxBody     = 32 << 20
	tries       = 6
)

// Client searches recordings. BaseURL defaults to BaseURL; HTTP to a 30 s client.
type Client struct {
	BaseURL   string
	HTTP      *http.Client
	UserAgent string
	// Sleep waits between retries; tests replace it.
	Sleep func(context.Context, time.Duration) error
	// Evict, when set, drops a cached answer the client could not decode (httpcache.Transport.Evict).
	Evict func(method, url string) error
}

// Recording is one search hit.
type Recording struct {
	ID               string
	Score            int
	Title            string
	Disambiguation   string
	FirstReleaseDate string // YYYY, YYYY-MM or YYYY-MM-DD; "" when unknown
	// Credit is the artist credit as printed ("Simon & Garfunkel"); Artists its artists' names; SortNames their
	// sort names, aligned ("Harley, Steve & Cockney Rebel": a person's band; "" when MusicBrainz sent none).
	Credit    string
	Artists   []string
	SortNames []string
	ISRCs     []string
	Releases  []Release
}

// Release is one release a recording is on.
type Release struct {
	ID     string
	Title  string
	Status string // Official, Bootleg, Promotion, …
	Date   string
	Group  Group
}

// Group is a release group.
type Group struct {
	ID             string
	Title          string
	PrimaryType    string // Album, Single, EP, …
	SecondaryTypes []string
	// Date is the earliest date among the releases of the group seen in the search; set by Pick.
	Date string
}

type searchBody struct {
	Count      int `json:"count"`
	Recordings []struct {
		ID               string `json:"id"`
		Score            int    `json:"score"`
		Title            string `json:"title"`
		Disambiguation   string `json:"disambiguation"`
		FirstReleaseDate string `json:"first-release-date"`
		ArtistCredit     []struct {
			Name       string `json:"name"`
			JoinPhrase string `json:"joinphrase"`
			Artist     struct {
				SortName string `json:"sort-name"`
			} `json:"artist"`
		} `json:"artist-credit"`
		ISRCs    []string `json:"isrcs"`
		Releases []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			Status       string `json:"status"`
			Date         string `json:"date"`
			ReleaseGroup struct {
				ID             string   `json:"id"`
				Title          string   `json:"title"`
				PrimaryType    string   `json:"primary-type"`
				SecondaryTypes []string `json:"secondary-types"`
			} `json:"release-group"`
		} `json:"releases"`
	} `json:"recordings"`
}

// Search finds recordings by artist and title (Lucene phrase query on both fields), first released by the end of
// year until when it is not 0, up to maxPages pages. The year bound is what makes the answer usable: Bohemian
// Rhapsody has 233 recordings by Queen, the studio one was on none of the three pages, and pages past the first
// shuffle between calls (ties in score); bounded to 1976 there are 4, the studio one among them.
func (c *Client) Search(ctx context.Context, artist, title string, until int) ([]Recording, error) {
	query := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, escape(title), escape(artist))
	if until > 0 {
		query += fmt.Sprintf(" AND firstreleasedate:[0000 TO %04d-12-31]", until)
	}
	var out []Recording
	for offset, page := 0, 0; page < maxPages; page++ {
		recs, count, err := c.searchPage(ctx, query, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
		offset += searchLimit
		if offset >= count || len(recs) == 0 {
			break
		}
	}
	return out, nil
}

// maxPages bounds one search: the tail past 300 is covers and karaoke.
const maxPages = 3

func (c *Client) searchPage(ctx context.Context, query string, offset int) ([]Recording, int, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(searchLimit))
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = BaseURL
	}
	var sb searchBody
	if err := c.get(ctx, base+"/ws/2/recording?"+q.Encode(), &sb); err != nil {
		return nil, 0, err
	}
	out := make([]Recording, 0, len(sb.Recordings))
	for _, r := range sb.Recordings {
		rec := Recording{ID: r.ID, Score: r.Score, Title: r.Title, Disambiguation: r.Disambiguation,
			FirstReleaseDate: r.FirstReleaseDate, ISRCs: r.ISRCs}
		var credit strings.Builder
		for _, a := range r.ArtistCredit {
			credit.WriteString(a.Name + a.JoinPhrase)
			rec.Artists = append(rec.Artists, a.Name)
			rec.SortNames = append(rec.SortNames, a.Artist.SortName)
		}
		rec.Credit = credit.String()
		for _, rel := range r.Releases {
			g := rel.ReleaseGroup
			rec.Releases = append(rec.Releases, Release{ID: rel.ID, Title: rel.Title, Status: rel.Status, Date: rel.Date,
				Group: Group{ID: g.ID, Title: g.Title, PrimaryType: g.PrimaryType, SecondaryTypes: g.SecondaryTypes}})
		}
		out = append(out, rec)
	}
	return out, sb.Count, nil
}

// get fetches url and decodes it into out, retrying a 503 (MusicBrainz's "too fast"), 429, 502, 504, a network
// error, a body cut off mid-transfer and a 200 that does not decode, with a doubling backoff. A full import (~4,000
// requests) met the first ones a few times (2026-11-04). The undecodable 200 is evicted from the cache first, so no
// later run replays it.
func (c *Client) get(ctx context.Context, u string, out any) error {
	wait := 2 * time.Second
	for attempt := 1; ; attempt++ {
		status, body, err := c.once(ctx, u)
		var retry error
		switch {
		case errors.Is(err, httpcache.ErrOffline):
			return err
		case err != nil && ctx.Err() == nil:
			retry = err
		case err != nil:
			return ctx.Err()
		case status == http.StatusOK:
			err := json.Unmarshal(body, out)
			if err == nil {
				return nil
			}
			retry = fmt.Errorf("musicbrainz: decode search: %w", err)
			if c.Evict != nil {
				if err := c.Evict(http.MethodGet, u); err != nil {
					return fmt.Errorf("musicbrainz: evict: %w", err)
				}
			}
		case status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests ||
			status == http.StatusBadGateway || status == http.StatusGatewayTimeout:
			retry = fmt.Errorf("musicbrainz: status %d", status)
		default:
			return fmt.Errorf("musicbrainz: status %d", status)
		}
		if attempt >= tries {
			return fmt.Errorf("%w after %d tries", retry, attempt)
		}
		if err := c.sleep(ctx, wait); err != nil {
			return err
		}
		wait *= 2
	}
}

// once is one GET: status and body, or the transport's error.
func (c *Client) once(ctx context.Context, u string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("musicbrainz: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return 0, nil, fmt.Errorf("musicbrainz: read: %w", err)
	}
	return resp.StatusCode, body, nil
}

func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	if !run.Sleep(ctx, d) {
		return ctx.Err()
	}
	return nil
}

// escape makes s safe inside a Lucene phrase.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// Match is the song a card resolves to.
type Match struct {
	// Recording is the earliest matching recording: its MBID is the song id.
	Recording Recording
	// ISRCs are the recording's, then those of the other matching studio recordings, deduped.
	ISRCs []string
	// Year is the earliest first-release year over the matching recordings; 0 when none has a date.
	Year int
	// Original is the release group of the earliest official studio album the recording is on; zero when none.
	Original Group
	// First is the release group of the recording's earliest release of any type (often the single).
	First Group
	// Exact says the artist credit matched the card's artist as written; false is a looser match (main artist
	// only, or one name inside the other): low confidence.
	Exact bool
}

// CoverGroup is the release group whose front cover stands for the song: the original album, else the first.
func (m Match) CoverGroup() Group {
	if m.Original.ID != "" {
		return m.Original
	}
	return m.First
}

// maxISRCs bounds the ISRC list handed to Apple's filter.
const maxISRCs = 10

// Pick chooses the card's song among recs: titles that match, artists that match exactly (else loosely), then the
// earliest first-release year; within it more releases, the earlier full date, the higher score, then the lower
// MBID, so the id is stable across re-imports. ok is false when nothing matches. Recordings on an official release
// and first released no earlier than the year before cardYear (0: no card) are preferred when there are any: an
// early date on a bootleg ("Let It Be" on a 1969 bootleg) or a misdated entry (a 1978 "Billie Jean") is not the
// song the card dates.
func Pick(recs []Recording, artist, title string, cardYear int) (m Match, ok bool) {
	var exact, loose []Recording
	for _, r := range recs {
		if !SameTitle(r.Title, title) {
			continue
		}
		switch {
		case exactArtist(r, artist):
			exact = append(exact, r)
		case looseArtist(r, artist):
			loose = append(loose, r)
		}
	}
	cands, isExact := exact, true
	if len(cands) == 0 {
		cands, isExact = loose, false
	}
	if len(cands) == 0 {
		return Match{}, false
	}
	// A live take, remix, demo, … the card does not name is the song only when nothing else matches; one the card
	// names ("Help! (Demo)") is preferred.
	plain := slices.DeleteFunc(slices.Clone(cands), func(r Recording) bool { return Version(r.Disambiguation+" "+r.Title, title) })
	if len(plain) > 0 {
		cands = plain
	}
	named := slices.DeleteFunc(slices.Clone(cands), func(r Recording) bool { return !names(r.Disambiguation+" "+r.Title, title) })
	if len(named) > 0 {
		cands = named
	}
	official := slices.DeleteFunc(slices.Clone(cands), func(r Recording) bool {
		return !slices.ContainsFunc(r.Releases, func(rel Release) bool { return rel.Status == "Official" })
	})
	if len(official) > 0 {
		cands = official
	}
	if cardYear > 0 {
		dated := slices.DeleteFunc(slices.Clone(cands), func(r Recording) bool { return Year(r.FirstReleaseDate) < cardYear-1 })
		if len(dated) > 0 {
			cands = dated
		}
	}
	slices.SortFunc(cands, func(a, b Recording) int {
		// By year first: within one year the recording on more releases is the canonical one; a stray copy
		// dated "1975" must not beat the album's "1975-04-21" on the shorter date.
		if c := compareDates(yearOf(a.FirstReleaseDate), yearOf(b.FirstReleaseDate)); c != 0 {
			return c
		}
		if len(a.Releases) != len(b.Releases) {
			return len(b.Releases) - len(a.Releases)
		}
		if c := compareDates(a.FirstReleaseDate, b.FirstReleaseDate); c != 0 {
			return c
		}
		if a.Score != b.Score {
			return b.Score - a.Score
		}
		return strings.Compare(a.ID, b.ID)
	})
	m = Match{Recording: cands[0], Exact: isExact, Year: Year(cands[0].FirstReleaseDate)}
	// ISRCs find the track on Apple, so only the song's own recordings may contribute: always the chosen one (its
	// "single/LP/12″ version" note names the original, not a variant; a loosely credited one is still the song),
	// then the others credited exactly to the card's artist that are no variant either, "version" and "edit"
	// included. A tribute band's or a remix's ISRC would find their track.
	// The original album is the earliest studio album of those recordings too: MusicBrainz files the album under a
	// sibling of the chosen one often, which then shows only a later soundtrack ("Nights in White Satin": A Bronx
	// Tale, 1993, while Days of Future Passed is the sibling's).
	m.Original, m.First = groups(m.Recording.Releases)
	seen := map[string]bool{}
	for i, r := range cands {
		text := r.Disambiguation + " " + r.Title
		if i > 0 && (!exactArtist(r, artist) || Version(text, title) || variant(text, title)) {
			continue
		}
		for _, isrc := range r.ISRCs {
			if !seen[isrc] && len(m.ISRCs) < maxISRCs {
				seen[isrc] = true
				m.ISRCs = append(m.ISRCs, isrc)
			}
		}
		// A sibling's album only from the card's time: its re-recording ("re-recorded" kept it out above) or a
		// reissue's album ("Tea for the Tillerman²", 2020) is no original.
		og, _ := groups(r.Releases)
		inTime := cardYear == 0 || (Year(og.Date) >= cardYear-1 && Year(og.Date) <= cardYear+1)
		if i > 0 && og.ID != "" && inTime && (m.Original.ID == "" || compareDates(og.Date, m.Original.Date) < 0) {
			m.Original = og
		}
	}
	return m, true
}

// groups finds the original album's and the first release's groups among releases.
func groups(rels []Release) (original, first Group) {
	dates := map[string]string{}
	for _, r := range rels {
		if r.Date != "" && (dates[r.Group.ID] == "" || compareDates(r.Date, dates[r.Group.ID]) < 0) {
			dates[r.Group.ID] = r.Date
		}
	}
	var origDate, firstDate string
	for _, r := range rels {
		if r.Date == "" || r.Group.ID == "" {
			continue
		}
		g := r.Group
		g.Date = dates[g.ID]
		if firstDate == "" || compareDates(r.Date, firstDate) < 0 {
			first, firstDate = g, r.Date
		}
		// A soundtrack is a studio album too (Help!, Saturday Night Fever); compilations, live and remix albums not.
		studioAlbum := g.PrimaryType == "Album" && !slices.ContainsFunc(g.SecondaryTypes, func(t string) bool { return t != "Soundtrack" }) &&
			(r.Status == "" || r.Status == "Official")
		if studioAlbum && (origDate == "" || compareDates(r.Date, origDate) < 0) {
			original, origDate = g, r.Date
		}
	}
	return original, first
}

// compareDates orders partial ISO dates; an empty date sorts last. "1976" sorts before "1976-08-15".
func compareDates(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	return strings.Compare(a, b)
}

func yearOf(date string) string { return date[:min(len(date), 4)] }

// Year is the year of a partial ISO date, 0 when there is none.
func Year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

const (
	// featMarks are the featuring marks, shared by mainSep, joiner and guestNote ("Feather" is no "feat").
	featMarks = `featuring\b|feat\b\.?|ft\.`
	// xMarks join two acts as "A x B": Latin, multiplication sign, Cyrillic (MusicBrainz credits "MEDUZA х Becky Hill").
	xMarks = `[x×х]`
)

var (
	// trailing matches a trailing "(Remastered 2011)", "[Live]" or " - Single Version".
	trailing = regexp.MustCompile(`\s*(\([^()]*\)|\[[^\[\]]*\]|\s-\s.*)$`)
	// mainSep splits a credit at a featuring mark only: "A feat. B", "A ft. B", "A featuring B", "A (with B)", "A vs.
	// B", "A x B". Never at "&", ",", "and", "+" or "/": they are inside band names (Simon & Garfunkel; Earth, Wind
	// & Fire; Florence + the Machine; AC/DC). Several artists are told by MusicBrainz's credit (Credit).
	mainSep = regexp.MustCompile(`(?i)\s*(?:\(?\b(?:` + featMarks + `)|\(\s*with\b|\bvs\.?\s|\s` + xMarks + `\s)\s*`)
	// joiner splits a credit at every mark that joins two names: the featuring marks and "&", ",", "+", " / ", "and"
	// (und, e, y, et). An unspaced slash is in the name (AC/DC). It splits band names too: only for a card
	// MusicBrainz does not know, or both sides alike.
	joiner = regexp.MustCompile(`(?i)\s*(?:[,&+]|\s/\s|\(\s*(?:` + featMarks + `|with\b)|\s(?:` + featMarks + `|with|and|und|e|y|et|vs\.?|` + xMarks + `)\s)\s*`)
	// guestNote matches a title parenthetical's "feat. X" or "with X"; guests holds X of each in a title.
	guestNote = regexp.MustCompile(`(?i)^(?:(` + featMarks + `)|with\b)\s*(.*)$`)
	guests    = regexp.MustCompile(`(?i)\((?:` + featMarks + `|with\b)\s*([^()]+)\)`)
	// aliasSep parts the names of one act: "Yusuf / Cat Stevens". Unspaced, a slash is in the name (AC/DC).
	aliasSep = regexp.MustCompile(`\s+/\s+`)
	// superscripts are digits, not separators: "Tea for the Tillerman²" is not the 1970 album.
	superscripts = strings.NewReplacer("⁰", "0", "¹", "1", "²", "2", "³", "3", "⁴", "4", "⁵", "5", "⁶", "6", "⁷", "7",
		"⁸", "8", "⁹", "9", "₀", "0", "₁", "1", "₂", "2", "₃", "3", "₄", "4", "₅", "5", "₆", "6", "₇", "7", "₈", "8", "₉", "9")
)

// Strip drops trailing parentheticals and " - …" suffixes from a title or album name: what versions add.
func Strip(s string) string {
	for {
		s = strings.TrimSpace(s)
		t := trailing.ReplaceAllString(s, "")
		if t == s || strings.TrimSpace(t) == "" {
			return s
		}
		s = t
	}
}

// noteWords mark a trailing title segment as a release note, not part of the song's name.
var noteWords = []string{"remaster", "remastered", "version", "edit", "mono", "stereo", "single",
	"radio", "mix", "from", "anniversary", "deluxe", "bonus", "mixed", "extended"}

var yearWord = regexp.MustCompile(`(^| )(19|20)\d\d( |$)`)

// CleanTitle drops trailing release notes from a card title: "Heroes - 2017 Remaster" → "Heroes", "Surfin' U.S.A.
// (Mono)" → "Surfin' U.S.A.", "You're The One That I Want - From Grease" → its name. A segment without such a
// note stays: "Bodyguard (I Will Always Love You)" names the song.
func CleanTitle(s string) string {
	for {
		s = strings.TrimSpace(s)
		loc := trailing.FindStringIndex(s)
		if loc == nil || loc[0] == 0 {
			return s
		}
		seg := " " + song.Normalize(s[loc[0]:]) + " "
		note := yearWord.MatchString(strings.TrimSpace(seg))
		for _, w := range noteWords {
			note = note || strings.Contains(seg, " "+w+" ")
		}
		if !note {
			return s
		}
		s = s[:loc[0]]
	}
}

// SongTitles are the readings of a card title, as written first: without its trailing parentheticals, then the
// last parenthetical's content, or (no parenthetical at the end) what follows the last " - ". Film and TV cards
// print "Film (Song)" or "Film - Song": "Robin Hood - König der Diebe ((Everything I Do) I Do It For You)" is the song "(Everything I Do) I Do It For You".
// A guest note is no song: "(feat. X)" never, "(with X)" when X is one of artists (the card's, the recording's
// credit): "Miracle (with Ellie Goulding)" is no song "with Ellie Goulding", "Wunderbare Jahre (With a Little Help
// from My Friends)" is one.
func SongTitles(title string, artists ...string) []string {
	out := []string{title}
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	add(Strip(title))
	t := strings.TrimSpace(title)
	if !strings.HasSuffix(t, ")") {
		if i := strings.LastIndex(t, " - "); i > 0 {
			add(Strip(t[i+3:]))
		}
	} else {
		depth := 0
		for i := len(t) - 1; i >= 0; i-- {
			switch t[i] {
			case ')':
				depth++
			case '(':
				depth--
			}
			if depth == 0 {
				if inner := strings.TrimSpace(t[i+1 : len(t)-1]); !isGuest(inner, artists) {
					add(inner)
				}
				break
			}
		}
	}
	return out
}

// isGuest says a title parenthetical's content is a guest note: a featuring mark, or "with" and names all among
// artists.
func isGuest(inner string, artists []string) bool {
	g := guestNote.FindStringSubmatch(inner)
	switch {
	case g == nil:
		return false
	case g[1] != "":
		return true
	}
	known := map[string]bool{}
	for _, a := range artists {
		for _, k := range artistSet(a) {
			known[k] = true
		}
	}
	set := artistSet(g[2])
	return len(set) > 0 && !slices.ContainsFunc(set, func(k string) bool { return !known[k] })
}

// TitleKey is a title normalized as written and without what versions append; SameTitle compares these.
func TitleKey(s string) (full, stripped string) {
	s = superscripts.Replace(s)
	return song.Normalize(s), song.Normalize(Strip(s))
}

// SameTitle compares two titles normalized, as written or without what versions append.
func SameTitle(a, b string) bool {
	fa, sa := TitleKey(a)
	fb, sb := TitleKey(b)
	return fa == fb || sa == sb
}

// MainArtist is the first artist of a credit: "Shakira feat. Wyclef Jean" → "Shakira".
func MainArtist(s string) string {
	parts := mainSep.Split(s, 2)
	if strings.TrimSpace(parts[0]) == "" {
		return s
	}
	return strings.TrimSpace(parts[0])
}

// artistKey is an artist name normalized, a leading "the" dropped: "The Beatles" and "Beatles" are one act.
func artistKey(s string) string { return strings.TrimPrefix(song.Normalize(s), "the ") }

func exactArtist(r Recording, artist string) bool {
	want := artistKey(artist)
	if artistKey(r.Credit) == want {
		return true
	}
	return len(r.Artists) > 0 && artistKey(r.Artists[0]) == want
}

// SameArtist is the loose artist match: the names equal, or their main artists ("Shakira feat. Wyclef Jean" →
// Shakira), or the same artists in any order and joined any way ("Queen, David Bowie" and "Queen & David Bowie";
// "Ricchi E Poveri" and "Ricchi & Poveri"), a leading "the" aside. Never one name inside the other: "Abba Revival
// Band" and "Beatles Tribute" are other acts, and "Simon & Garfunkel" is not Simon. An act written with its names
// ("Yusuf / Cat Stevens") is either of them.
func SameArtist(a, b string) bool { return sameForms(forms(a), forms(b)) }

// form is one name of an act as SameArtist compares it: its key, its main artist's, its artists' as a set.
type form struct {
	key, main string
	set       []string
}

// forms are an act's names (aliasSep) as SameArtist compares them.
func forms(s string) []form {
	var out []form
	for _, x := range aliasSep.Split(s, -1) {
		if k := artistKey(x); k != "" {
			out = append(out, form{key: k, main: artistKey(MainArtist(x)), set: artistSet(x)})
		}
	}
	return out
}

func sameForms(a, b []form) bool {
	for _, x := range a {
		for _, y := range b {
			if x.key == y.key || (x.main != "" && x.main == y.main) || (len(x.set) > 0 && slices.Equal(x.set, y.set)) {
				return true
			}
		}
	}
	return false
}

// Artists splits a credit at every joiner: "Queen & David Bowie" → Queen, David Bowie. Band names split too
// ("Earth, Wind & Fire" → Earth, Wind, Fire), so a part alone says nothing; see Credit.
func Artists(s string) []string {
	var out []string
	for _, p := range joiner.Split(s, -1) {
		// "A (with B)" leaves "B)"; "EAV (Erste Allgemeine Verunsicherung)" keeps its parenthetical.
		if p = strings.TrimSpace(p); strings.Count(p, "(") < strings.Count(p, ")") {
			p = strings.TrimSpace(strings.TrimSuffix(p, ")"))
		}
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(s)}
	}
	return out
}

// Lead is a credit's first name split at every joiner: "Queen, David Bowie" → Queen, "Earth, Wind & Fire" → Earth.
// Only a search term: what it finds must still be the whole credit's.
func Lead(s string) string { return Artists(s)[0] }

// artistSet is a credit's artists as sorted keys.
func artistSet(s string) []string {
	var out []string
	for _, a := range Artists(s) {
		if k := artistKey(a); k != "" {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Credit is who a card's song is by, to check another catalog's credits (Apple's) against: the artists of the
// matched recording as MusicBrainz splits its credit, each name whole (Earth, Wind & Fire; Simon & Garfunkel); for
// a card MusicBrainz does not know, the card's names. Build it once per song.
type Credit struct {
	// card and credit are the card's artist and the recording's credit as SameArtist compares them.
	card, credit []form
	search       string
	// pool holds the song's artists as tokens (nameTokens), the main one first, the first n of them; known says
	// they are MusicBrainz's. Then further names the card mentions: its own, the guests in its title.
	pool  [][]string
	n     int
	main  []string
	known bool
}

// NewCredit is the credit of the card's song; rec is the matched recording, zero when none.
func NewCredit(card, title string, rec Recording) Credit {
	c := Credit{card: forms(card), credit: forms(rec.Credit), search: MainArtist(card), main: nameTokens(MainArtist(card))}
	add := func(a string) {
		if t := nameTokens(a); len(t) > 0 && !slices.ContainsFunc(c.pool, func(p []string) bool { return slices.Equal(p, t) }) {
			c.pool = append(c.pool, t)
		}
	}
	if len(rec.Artists) > 0 {
		c.known = true
		for _, a := range rec.Artists {
			add(a)
		}
		if len(rec.Artists) > 1 && !slices.ContainsFunc(rec.Artists, func(a string) bool { return SameArtist(a, card) }) {
			// Apple has "Under Pressure" by Queen, "feat. David Bowie" in the title. A card naming one of the
			// artists is searched as written: "Michael Nyman", not the orchestra MusicBrainz credits first.
			c.search = rec.Artists[0]
		}
	} else {
		for _, a := range Artists(card) {
			add(a)
		}
	}
	c.n = len(c.pool)
	if c.known && c.n > 0 {
		c.main = c.pool[0]
	}
	for _, a := range Artists(card) {
		add(a)
	}
	for _, g := range guests.FindAllStringSubmatch(title, -1) {
		for _, a := range Artists(g[1]) {
			add(a)
		}
	}
	return c
}

// Search is who to look the song up by: MusicBrainz's main artist of a song by several the card names none of, else
// the card's main artist.
func (c Credit) Search() string { return c.search }

// By says name, another catalog's credit, is the song's artist: the card's or the recording's credit
// (SameArtist); a credit made of the song's artists and names the card mentions only, with all of the song's
// artists or led by MusicBrainz's main one ("Queen" for Queen & David Bowie, "Calvin Harris, Dua Lipa" for "One Kiss
// (with Dua Lipa)"); or the main artist with a backing band ("Glenn Miller and His Orchestra"). Never a part of a
// name ("Earth" is not Earth, Wind & Fire) nor a credit naming an act the song does not ("Queen + Adam Lambert" is
// not Queen).
func (c Credit) By(name string) bool {
	if nf := forms(name); sameForms(nf, c.card) || sameForms(nf, c.credit) {
		return true
	}
	toks := artistTokens(name)
	if used, ok := compose(toks, c.pool); ok && c.n > 0 {
		all := true
		for i := range c.n {
			all = all && slices.Contains(used, i)
		}
		if all || (c.known && used[0] == 0) {
			return true
		}
	}
	return backed(toks, c.main)
}

// Led is By, or a co-credit of the song's main artist ("&", "and", ","): "Nino Rota & Carlo Savina" for Nino Rota.
// For a hit an ISRC ties to the recording already, whose credit only has to rule out another act; one joined by
// "+", "x" or "feat." is another act's take ("Queen + Adam Lambert").
func (c Credit) Led(name string) bool {
	if c.By(name) {
		return true
	}
	toks := artistTokens(name)
	end, ok := prefix(toks, 0, c.main)
	return ok && end+1 < len(toks) && toks[end] == coTok
}

// Join tokens stand for the words and marks that join two names in artistTokens, by kind: a co-credit ("&", "and",
// ",", " / ", und, e, y, et), a guest ("feat.", "with", "vs."), "x", "+". Never a word song.Normalize writes: the X
// of "Malcolm X" stays a word.
const (
	coTok   = "&"
	featTok = "&feat"
	xTok    = "&x"
	plusTok = "&+"
)

// joinWords are the joining words as song.Normalize spells them ("&" becomes "and"), with their join token.
var joinWords = map[string]string{"and": coTok, "und": coTok, "e": coTok, "y": coTok, "et": coTok, "with": featTok,
	"feat": featTok, "ft": featTok, "featuring": featTok, "vs": featTok, "x": xTok, "х": xTok, "plus": plusTok}

func isJoin(t string) bool { return t == coTok || t == featTok || t == xTok || t == plusTok }

// joinMarks are the joining marks song.Normalize drops or spells as "and": made words so they still part names, by
// kind. An unspaced slash is in the name (AC/DC).
var joinMarks = strings.NewReplacer(",", " & ", " / ", " & ", "+", " plus ", "×", " x ")

// artistTokens is a credit normalized into words, every joining word or mark between two words as its join token
// ("Malcolm X", "E Nomine" keep theirs), a leading "the" dropped.
func artistTokens(s string) []string {
	toks := strings.Fields(song.Normalize(joinMarks.Replace(s)))
	for i, t := range toks {
		if j := joinWords[t]; j != "" && i > 0 && i < len(toks)-1 {
			toks[i] = j
		}
	}
	if len(toks) > 1 && toks[0] == "the" {
		toks = toks[1:]
	}
	return toks
}

// nameTokens is one name's words without its joins: "Earth, Wind & Fire" reads as Earth Wind and Fire written any way.
func nameTokens(s string) []string { return slices.DeleteFunc(artistTokens(s), isJoin) }

// prefix says toks from i start with name (nameTokens; joins between its words skipped, a "the" before it
// optional), and where it ends.
func prefix(toks []string, i int, name []string) (int, bool) {
	if len(name) == 0 {
		return 0, false
	}
	if i < len(toks) && toks[i] == "the" && name[0] != "the" {
		if end, ok := prefix(toks, i+1, name); ok {
			return end, true
		}
	}
	for k, w := range name {
		for k > 0 && i < len(toks) && isJoin(toks[i]) {
			i++
		}
		if i >= len(toks) || toks[i] != w {
			return 0, false
		}
		i++
	}
	return i, true
}

// compose reads toks (artistTokens) as names of pool (nameTokens) joined by join tokens, each name at most once;
// used lists the names in credit order.
func compose(toks []string, pool [][]string) (used []int, ok bool) {
	var walk func(i int) bool
	walk = func(i int) bool {
		if i == len(toks) {
			return len(used) > 0
		}
		if len(used) > 0 {
			if !isJoin(toks[i]) {
				return false
			}
			for i < len(toks) && isJoin(toks[i]) {
				i++
			}
		}
		for n, name := range pool {
			if slices.Contains(used, n) {
				continue
			}
			if end, ok := prefix(toks, i, name); ok {
				used = append(used, n)
				if walk(end) {
					return true
				}
				used = used[:len(used)-1]
			}
		}
		return false
	}
	ok = len(toks) > 0 && walk(0)
	return used, ok
}

// bandOf says group, one act to MusicBrainz, is the artist's band: "Steve Harley & Cockney Rebel" for Steve Harley.
// Its name is the artist's and another joined by "&", "and" or "und" (not "+", "x", "feat.": "Queen + Adam
// Lambert"), and its sort name (sort) says a person leads a band: "Harley, Steve & Cockney Rebel". Not a duo of
// two people ("Hall, Daryl & Oates, John") nor of two names ("Simon & Garfunkel" sorts as written), nor an
// orchestra that is an act of its own.
func bandOf(group, sort, artist string) bool {
	toks := artistTokens(group)
	end, ok := prefix(toks, 0, nameTokens(artist))
	if !ok || end+1 >= len(toks) || toks[end] != coTok || slices.ContainsFunc(toks[end+1:], func(t string) bool { return separateActs[t] }) {
		return false
	}
	m := personBand.FindStringSubmatch(sort)
	return m != nil && strings.Contains(m[1], ",") && !strings.Contains(sortArticle.ReplaceAllString(m[2], ""), ",")
}

// personBand parts a group's sort name at its first "&", "and" or "und": the lead and the rest.
var personBand = regexp.MustCompile(`(?i)^(.+?)\s+(?:&|and|und)\s+(.+)$`)

// sortArticle is a band's article a sort name puts last: "Dekker, Desmond & Aces, The". Not a person's first name.
var sortArticle = regexp.MustCompile(`(?i),\s*(?:the|die|der|das|les|la|le|los|las|il|i)$`)

// separateActs are words of an ensemble that is an act of its own, never a singer's band: "Elvis Presley & The
// Royal Philharmonic Orchestra".
var separateActs = map[string]bool{"philharmonic": true, "philharmonia": true, "philharmoniker": true,
	"symphony": true, "symphoniker": true, "sinfonieorchester": true, "symphonieorchester": true}

// backingWords start a backing band's name after the main artist: "and His Orchestra", "und seine Solisten".
var backingWords = map[string]bool{"his": true, "her": true, "their": true, "seine": true, "seinem": true,
	"ihr": true, "ihre": true, "ihrem": true}

// backed says toks are main with the band that is theirs: "Glenn Miller and His Orchestra". "& The X" is not
// enough ("Elvis Presley & The Royal Philharmonic Orchestra"); MusicBrainz tells such a band (bandOf).
func backed(toks, main []string) bool {
	end, ok := prefix(toks, 0, main)
	return ok && end+2 < len(toks) && toks[end] == coTok && backingWords[toks[end+1]]
}

// versionWords mark a recording or track that is not the song as released: live, remixed, a demo, … A card that
// names the word itself ("Mi Gente - Original Mix", "Unplugged … (Live)") asks for that one.
var versionWords = []string{"live", "remix", "remixed", "rmx", "dj mix", "demo", "re record", "re recorded",
	"rerecorded", "re recording", "acoustic", "unplugged", "karaoke", "instrumental", "cover", "tribute"}

// variantWords mark other cuts of the song: kept as the song when nothing better matches, but their ISRCs are not
// asked of Apple.
var variantWords = []string{"version", "edit", "mix"}

// Version says text (a disambiguation, title or album name) marks a live take, remix, demo, … the card title does
// not name.
func Version(text, cardTitle string) bool { return hasWord(text, versionWords, cardTitle) }

func variant(text, cardTitle string) bool { return hasWord(text, variantWords, cardTitle) }

// names says text carries a version word the card title names too.
func names(text, cardTitle string) bool {
	t, c := " "+song.Normalize(text)+" ", " "+song.Normalize(cardTitle)+" "
	for _, w := range versionWords {
		if strings.Contains(c, " "+w+" ") && strings.Contains(t, " "+w+" ") {
			return true
		}
	}
	return false
}

func hasWord(text string, words []string, cardTitle string) bool {
	t, c := " "+song.Normalize(text)+" ", " "+song.Normalize(cardTitle)+" "
	for _, w := range words {
		if strings.Contains(t, " "+w+" ") && !strings.Contains(c, " "+w+" ") {
			return true
		}
	}
	return false
}

// looseArtist: the credit or one of its artists is the card's artist, loosely; or the card's artist with a backing
// band ("Glenn Miller and His Orchestra"; "Ian Dury & the Blockheads" as MusicBrainz sorts it, bandOf); or
// MusicBrainz credits several artists and the card names the
// first of them followed by more ("Shakira & Wyclef" for Shakira feat. Wyclef Jean). The card side is never split
// on its own: "Simon & Garfunkel" is one act, and Simon is not.
func looseArtist(r Recording, artist string) bool {
	if SameArtist(r.Credit, artist) || backed(artistTokens(r.Credit), nameTokens(artist)) ||
		(len(r.Artists) == 1 && len(r.SortNames) == 1 && bandOf(r.Artists[0], r.SortNames[0], artist)) {
		return true
	}
	for _, a := range r.Artists {
		if SameArtist(a, artist) {
			return true
		}
	}
	if len(r.Artists) > 1 {
		first := artistKey(r.Artists[0])
		return first != "" && strings.HasPrefix(artistKey(artist)+" ", first+" ")
	}
	return false
}
