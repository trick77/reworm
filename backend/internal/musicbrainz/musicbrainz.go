// Package musicbrainz searches MusicBrainz recordings (import only) and picks a card's song among them: the earliest
// matching recording, its ISRCs and the release groups for the original-album match and the cover. MusicBrainz asks
// for at most one request per second and a User-Agent naming the app and a contact (plan D8); the caller's transport
// spaces the requests, this client retries the 503 it answers when they come too fast.
package musicbrainz

import (
	"context"
	"encoding/json"
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
}

// Recording is one search hit.
type Recording struct {
	ID               string
	Score            int
	Title            string
	Disambiguation   string
	FirstReleaseDate string // YYYY, YYYY-MM or YYYY-MM-DD; "" when unknown
	// Credit is the artist credit as printed ("Simon & Garfunkel"); Artists its artists' names.
	Credit   string
	Artists  []string
	ISRCs    []string
	Releases []Release
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

// Search finds recordings by artist and title (Lucene phrase query on both fields), up to maxPages pages: Bohemian
// Rhapsody has 233 recordings by Queen and the studio one is not among the first 100.
func (c *Client) Search(ctx context.Context, artist, title string) ([]Recording, error) {
	var out []Recording
	for offset, page := 0, 0; page < maxPages; page++ {
		recs, count, err := c.searchPage(ctx, artist, title, offset)
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

func (c *Client) searchPage(ctx context.Context, artist, title string, offset int) ([]Recording, int, error) {
	q := url.Values{}
	q.Set("query", fmt.Sprintf(`recording:"%s" AND artist:"%s"`, escape(title), escape(artist)))
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(searchLimit))
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = BaseURL
	}
	body, err := c.get(ctx, base+"/ws/2/recording?"+q.Encode())
	if err != nil {
		return nil, 0, err
	}
	var sb searchBody
	if err := json.Unmarshal(body, &sb); err != nil {
		return nil, 0, fmt.Errorf("musicbrainz: decode search: %w", err)
	}
	out := make([]Recording, 0, len(sb.Recordings))
	for _, r := range sb.Recordings {
		rec := Recording{ID: r.ID, Score: r.Score, Title: r.Title, Disambiguation: r.Disambiguation,
			FirstReleaseDate: r.FirstReleaseDate, ISRCs: r.ISRCs}
		var credit strings.Builder
		for _, a := range r.ArtistCredit {
			credit.WriteString(a.Name + a.JoinPhrase)
			rec.Artists = append(rec.Artists, a.Name)
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

// get fetches url, retrying a 503 (MusicBrainz's "too fast"), 429, 502 and 504 with a doubling backoff.
func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	wait := 2 * time.Second
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		client := c.HTTP
		if client == nil {
			client = &http.Client{Timeout: 30 * time.Second}
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("musicbrainz: %w", err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("musicbrainz: read: %w", err)
		}
		switch resp.StatusCode {
		case http.StatusOK:
			return body, nil
		case http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusGatewayTimeout:
			if attempt >= tries {
				return nil, fmt.Errorf("musicbrainz: status %d after %d tries", resp.StatusCode, attempt)
			}
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
			wait *= 2
		default:
			return nil, fmt.Errorf("musicbrainz: status %d", resp.StatusCode)
		}
	}
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
// MBID, so the id is stable across re-imports. ok is false when nothing matches.
func Pick(recs []Recording, artist, title string) (m Match, ok bool) {
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
	seen := map[string]bool{}
	for i, r := range cands {
		if i > 0 && strings.Contains(strings.ToLower(r.Disambiguation), "live") {
			continue
		}
		for _, isrc := range r.ISRCs {
			if !seen[isrc] && len(m.ISRCs) < maxISRCs {
				seen[isrc] = true
				m.ISRCs = append(m.ISRCs, isrc)
			}
		}
	}
	m.Original, m.First = groups(m.Recording.Releases)
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
		studioAlbum := g.PrimaryType == "Album" && len(g.SecondaryTypes) == 0 && (r.Status == "" || r.Status == "Official")
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

var (
	// trailing matches a trailing "(Remastered 2011)", "[Live]" or " - Single Version".
	trailing = regexp.MustCompile(`\s*(\([^()]*\)|\[[^\[\]]*\]|\s-\s.*)$`)
	// mainSep splits a credit into its artists: "A feat. B", "A & B", "A, B", "A x B", "A vs. B", "A with B".
	mainSep = regexp.MustCompile(`(?i)\s*(\bfeat\.?|\bft\.?|\bfeaturing\b|&|,|\bx\b|\bvs\.?|\bwith\b|\band\b|\+|/)\s*`)
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

// SameTitle compares two titles normalized, as written or without what versions append.
func SameTitle(a, b string) bool {
	na, nb := song.Normalize(a), song.Normalize(b)
	return na == nb || song.Normalize(Strip(a)) == song.Normalize(Strip(b))
}

// MainArtist is the first artist of a credit: "Shakira feat. Wyclef Jean" → "Shakira".
func MainArtist(s string) string {
	parts := mainSep.Split(s, 2)
	if strings.TrimSpace(parts[0]) == "" {
		return s
	}
	return strings.TrimSpace(parts[0])
}

func exactArtist(r Recording, artist string) bool {
	want := song.Normalize(artist)
	if song.Normalize(r.Credit) == want {
		return true
	}
	return len(r.Artists) > 0 && song.Normalize(r.Artists[0]) == want
}

// SameArtist is the loose artist match: main artists equal, or one name inside the other on word bounds.
func SameArtist(a, b string) bool {
	na, nb := song.Normalize(a), song.Normalize(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb || song.Normalize(MainArtist(a)) == song.Normalize(MainArtist(b)) {
		return true
	}
	return strings.Contains(" "+na+" ", " "+nb+" ") || strings.Contains(" "+nb+" ", " "+na+" ")
}

func looseArtist(r Recording, artist string) bool {
	if SameArtist(r.Credit, artist) {
		return true
	}
	for _, a := range r.Artists {
		if SameArtist(a, artist) {
			return true
		}
	}
	return false
}
