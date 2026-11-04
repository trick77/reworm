package importer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/trick77/reworm/internal/apple"
	"github.com/trick77/reworm/internal/cards"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

// storefronts are tried in order; the one that answered is stored with the apple_id.
var storefronts = []string{"ch", "de", "us"}

// searchLimit is how many Apple search hits the artist+title fallback looks at.
const searchLimit = 25

// Catalog is what the pipeline asks Apple; *apple.Client implements it.
type Catalog interface {
	SongsByISRC(ctx context.Context, storefront string, isrcs []string) ([]apple.Song, error)
	SearchSongs(ctx context.Context, storefront, term string, limit int) ([]apple.Song, error)
	SearchAlbums(ctx context.Context, storefront, term string, limit int) ([]apple.Album, error)
	AlbumTracks(ctx context.Context, storefront, albumID string) ([]apple.Song, error)
	SongByID(ctx context.Context, storefront, id string) (apple.Song, bool, error)
}

// Recordings is what the pipeline asks MusicBrainz; *musicbrainz.Client implements it.
type Recordings interface {
	Search(ctx context.Context, artist, title string, until int) ([]musicbrainz.Recording, error)
}

// Pipeline resolves card songs against MusicBrainz, Apple and the Cover Art Archive.
type Pipeline struct {
	MB      Recordings
	Apple   Catalog
	Covers  *CoverArt
	Workers int
	Log     *slog.Logger
}

// entry is one unique song of the card sets before resolving.
type entry struct {
	Artist, Title string
	// Year is the earliest card year; Years every distinct one (more than one is a conflict in the report).
	Year  int
	Years []int
	Sets  []string
}

// resolved is an entry with what the sources said: the song and the report's notes.
type resolved struct {
	entry
	Song song.Song
	// MBYear is MusicBrainz's earliest first-release year, 0 when unknown.
	MBYear int
	// AppleYear is the chosen Apple track's release year; AppleOriginal says it is on the original album.
	AppleYear     int
	AppleOriginal bool
	MBMatched     bool
	// Low lists why a match is low confidence; empty when it is not.
	Low []string
	// match is the MusicBrainz match, kept for an override that picks another Apple track.
	match musicbrainz.Match
}

// collect dedupes the sets' cards by normalized artist+title, drops year-less cards and counts per set.
func collect(sets []cards.Set) ([]entry, map[string]*setStats) {
	stats := map[string]*setStats{}
	byKey := map[string]*entry{}
	for _, set := range sets {
		st := &setStats{}
		stats[set.Slug] = st
		for _, c := range set.Cards {
			st.Cards++
			if c.Year == 0 {
				st.NoYear++
				continue
			}
			k := song.Key(c.Artist, c.Title)
			e := byKey[k]
			if e == nil {
				e = &entry{Artist: c.Artist, Title: c.Title, Year: c.Year}
				byKey[k] = e
			}
			if !slices.Contains(e.Years, c.Year) {
				e.Years = append(e.Years, c.Year)
			}
			e.Year = min(e.Year, c.Year)
			if !slices.Contains(e.Sets, set.Slug) {
				e.Sets = append(e.Sets, set.Slug)
			}
		}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]entry, 0, len(keys))
	for _, k := range keys {
		e := byKey[k]
		slices.Sort(e.Years)
		slices.Sort(e.Sets)
		out = append(out, *e)
	}
	return out, stats
}

// resolveAll runs resolve over entries on Workers goroutines (MusicBrainz's throttle serializes its share). Every
// error is collected; any one fails the run, whose cache makes the next one cheap.
func (p *Pipeline) resolveAll(ctx context.Context, entries []entry) ([]resolved, error) {
	out := make([]resolved, len(entries))
	errs := make([]error, len(entries))
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for range max(p.Workers, 1) {
		wg.Go(func() {
			for i := range jobs {
				out[i], errs[i] = p.resolve(ctx, entries[i])
				mu.Lock()
				done++
				if done%100 == 0 || done == len(entries) {
					p.log().Info("import progress", "done", done, "of", len(entries))
				}
				mu.Unlock()
			}
		})
	}
	for i := range entries {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var all []error
	for i, err := range errs {
		if err != nil {
			all = append(all, fmt.Errorf("%s – %s: %w", entries[i].Artist, entries[i].Title, err))
		}
	}
	if len(all) > 0 {
		return nil, fmt.Errorf("%d songs failed (re-run: answers so far are cached): %w", len(all), errors.Join(all[:min(len(all), 10)]...))
	}
	return out, nil
}

func (p *Pipeline) log() *slog.Logger {
	if p.Log != nil {
		return p.Log
	}
	return slog.Default()
}

// resolve finds one song: MusicBrainz recording, Apple track, cover.
func (p *Pipeline) resolve(ctx context.Context, e entry) (resolved, error) {
	r := resolved{entry: e}
	s := song.Song{Artist: e.Artist, Title: e.Title, Year: e.Year, YearSource: song.YearCard, Sets: e.Sets}

	m, ok, why, err := p.findRecording(ctx, e)
	if err != nil {
		return r, err
	}
	r.match = m
	if ok {
		r.MBMatched, r.MBYear = true, m.Year
		s.ID, s.MBID = m.Recording.ID, m.Recording.ID
		switch {
		case why != "":
			r.Low = append(r.Low, why)
		case !m.Exact:
			r.Low = append(r.Low, "MusicBrainz artist matched loosely ("+m.Recording.Credit+")")
		}
	} else {
		s.ID = song.Slug(e.Artist, e.Title)
	}

	track, sf, viaSearch, err := p.findTrack(ctx, e, m)
	if err != nil {
		return r, err
	}
	r.Song = s
	if viaSearch {
		r.Low = append(r.Low, searchNote+" ("+track.Artist+" – "+track.Name+")")
	}
	return r, p.setTrack(ctx, &r, track, sf)
}

// searchNote starts the low-confidence note of an Apple track found by search.
const searchNote = "Apple track found by search"

// setTrack makes t (on storefront sf; zero: none found) the song's Apple track and picks its cover: Apple's
// artwork when t is on the original album, else the Cover Art Archive's front of the MusicBrainz release group,
// else none.
func (p *Pipeline) setTrack(ctx context.Context, r *resolved, t apple.Song, sf string) error {
	s, m := &r.Song, r.match
	s.AppleID, s.Storefront, s.AppleAlbum, s.ISRC = t.ID, sf, t.Album, t.ISRC
	if s.ISRC == "" && len(m.ISRCs) > 0 {
		s.ISRC = m.ISRCs[0]
	}
	r.AppleYear, r.AppleOriginal = t.Year(), t.ID != "" && onOriginal(t, m, s.Year)
	s.CoverURL, s.CoverSrc = "", ""
	switch g := m.CoverGroup(); {
	case r.AppleOriginal && t.Artwork != "":
		s.CoverURL, s.CoverSrc = t.Artwork, song.CoverApple
	case g.ID != "":
		u, found, err := p.Covers.Front(ctx, g.ID)
		if err != nil {
			return err
		}
		if found {
			s.CoverURL, s.CoverSrc = u, song.CoverCAA
		}
	}
	return nil
}

// findRecording searches MusicBrainz for recordings first released by the year after the card's: as the card
// writes it, then with the main artist and each other reading of the title (musicbrainz.SongTitles); last, without
// the year bound. Only the first search can match exactly; the others are low confidence, why says how.
func (p *Pipeline) findRecording(ctx context.Context, e entry) (m musicbrainz.Match, ok bool, why string, err error) {
	type query struct{ artist, title string }
	queries := []query{{e.Artist, e.Title}}
	main := musicbrainz.MainArtist(e.Artist)
	for _, t := range musicbrainz.SongTitles(e.Title) {
		if q := (query{main, t}); !slices.Contains(queries, q) {
			queries = append(queries, q)
		}
	}
	for i, q := range queries {
		recs, err := p.MB.Search(ctx, q.artist, q.title, e.Year+1)
		if err != nil {
			return m, false, "", err
		}
		if m, ok := musicbrainz.Pick(recs, q.artist, q.title, e.Year); ok {
			if i == 0 {
				return m, true, "", nil
			}
			m.Exact = false
			return m, true, "MusicBrainz matched as " + q.artist + " – " + q.title, nil
		}
	}
	recs, err := p.MB.Search(ctx, e.Artist, e.Title, 0)
	if err != nil {
		return m, false, "", err
	}
	m, ok = musicbrainz.Pick(recs, e.Artist, e.Title, e.Year)
	m.Exact = false
	switch {
	case !ok:
	case m.Recording.FirstReleaseDate == "":
		why = "MusicBrainz knows no release date for it"
	case m.Year > e.Year+1:
		why = fmt.Sprintf("MusicBrainz has it only from %d, after the card year", m.Year)
	default:
		// Dated within the bound, yet the bounded searches missed it (an alias the bound query did not hit).
		why = "MusicBrainz matched only by a search without the year bound"
	}
	return m, ok, why, nil
}

// findTrack looks the song up on Apple, only tracks with a preview counting. Per storefront (ch → de → us): the
// ISRC lookup, then the original album found by name. The first original-album track wins; else the first
// storefront's best ISRC hit; else an artist+title search (viaSearch: low confidence).
func (p *Pipeline) findTrack(ctx context.Context, e entry, m musicbrainz.Match) (t apple.Song, sf string, viaSearch bool, err error) {
	var fallback apple.Song
	var fallbackSF string
	for _, sf := range storefronts {
		if len(m.ISRCs) > 0 {
			songs, err := p.Apple.SongsByISRC(ctx, sf, m.ISRCs)
			if err != nil {
				return apple.Song{}, "", false, err
			}
			if t, ok := best(songs, m, e, func(s apple.Song) bool { return sameSong(s, e, m) }); ok {
				if onOriginal(t, m, e.Year) {
					return t, sf, false, nil
				}
				if fallback.ID == "" {
					fallback, fallbackSF = t, sf
				}
			}
		}
		t, ok, err := p.originalAlbumTrack(ctx, sf, e, m)
		if err != nil {
			return apple.Song{}, "", false, err
		}
		if ok {
			return t, sf, false, nil
		}
	}
	if fallback.ID != "" {
		return fallback, fallbackSF, false, nil
	}
	titles := musicbrainz.SongTitles(e.Title)
	if m.Recording.Title != "" && !slices.Contains(titles, m.Recording.Title) {
		titles = append([]string{m.Recording.Title}, titles...)
	}
	for _, title := range titles[:min(len(titles), searchTitles)] {
		for _, sf := range storefronts {
			songs, err := p.Apple.SearchSongs(ctx, sf, musicbrainz.MainArtist(e.Artist)+" "+title, searchLimit)
			if err != nil {
				return apple.Song{}, "", false, err
			}
			same := func(s apple.Song) bool {
				return musicbrainz.SameTitle(s.Name, title) && musicbrainz.SameArtist(s.Artist, e.Artist)
			}
			if t, ok := best(songs, m, e, same); ok {
				return t, sf, true, nil
			}
		}
	}
	return apple.Song{}, "", false, nil
}

// sameSong says an Apple track found by ISRC is the card's song: its title one of the card's readings or the
// recording's, its artist the card's or the recording's. Apple files unrelated tracks under a famous ISRC
// (Yesterday's returned a 2023 single by another act).
func sameSong(s apple.Song, e entry, m musicbrainz.Match) bool {
	titles := musicbrainz.SongTitles(e.Title)
	if m.Recording.Title != "" {
		titles = append(titles, m.Recording.Title)
	}
	if !slices.ContainsFunc(titles, func(t string) bool { return musicbrainz.SameTitle(s.Name, t) }) {
		return false
	}
	return musicbrainz.SameArtist(s.Artist, e.Artist) || (m.Recording.Credit != "" && musicbrainz.SameArtist(s.Artist, m.Recording.Credit))
}

// searchTitles bounds the title readings the Apple search tries.
const searchTitles = 3

// albumsTried bounds the albums of one name whose tracks are read (Apple lists remasters and deluxe editions too).
const albumsTried = 2

// originalAlbumTrack finds the song on the MusicBrainz original album in Apple's catalog: an album search by
// artist and album title, then the album's tracks by ISRC, else by title. The ISRC lookup alone misses it for big
// hits: Apple answers at most 25 tracks per ISRC, mostly compilations.
func (p *Pipeline) originalAlbumTrack(ctx context.Context, sf string, e entry, m musicbrainz.Match) (apple.Song, bool, error) {
	g := m.CoverGroup()
	if g.ID == "" {
		return apple.Song{}, false, nil
	}
	albums, err := p.Apple.SearchAlbums(ctx, sf, musicbrainz.MainArtist(e.Artist)+" "+musicbrainz.Strip(g.Title), 10)
	if err != nil {
		return apple.Song{}, false, err
	}
	tried := 0
	for _, a := range albums {
		if tried == albumsTried {
			break
		}
		if !musicbrainz.SameTitle(a.Name, g.Title) || !musicbrainz.SameArtist(a.Artist, e.Artist) ||
			!originalYear(a.Year(), g, e.Year) {
			continue
		}
		tried++
		tracks, err := p.Apple.AlbumTracks(ctx, sf, a.ID)
		if err != nil {
			return apple.Song{}, false, err
		}
		for _, byISRC := range []bool{true, false} {
			for _, t := range tracks {
				hit := slices.Contains(m.ISRCs, t.ISRC)
				if !byISRC {
					hit = musicbrainz.SameTitle(t.Name, e.Title)
				}
				if hit && t.Preview {
					return t, true, nil
				}
			}
		}
	}
	return apple.Song{}, false, nil
}

// withinAYear says two years are at most one apart; an unknown (0) one passes.
func withinAYear(a, b int) bool { return a == 0 || b == 0 || (a-b <= 1 && b-a <= 1) }

// best picks among songs those with a preview (and passing keep, when set): the original-album track first, then
// one that is no live take, remix, karaoke, … the card does not ask for (by track and album name), then one on the
// artist's own album (not a various-artists compilation), then the earliest release, then the lowest id.
func best(songs []apple.Song, m musicbrainz.Match, e entry, keep func(apple.Song) bool) (apple.Song, bool) {
	var cands []apple.Song
	for _, s := range songs {
		if s.Preview && (keep == nil || keep(s)) {
			cands = append(cands, s)
		}
	}
	if len(cands) == 0 {
		return apple.Song{}, false
	}
	plain := func(s apple.Song) bool { return !musicbrainz.Version(s.Name+" "+s.Album, e.Title) }
	slices.SortFunc(cands, func(a, b apple.Song) int {
		oa, ob := onOriginal(a, m, e.Year), onOriginal(b, m, e.Year)
		switch {
		case oa && !ob:
			return -1
		case ob && !oa:
			return 1
		}
		if pa, pb := plain(a), plain(b); pa != pb {
			if pa {
				return -1
			}
			return 1
		}
		if sa, sb := ownAlbum(a), ownAlbum(b); sa != sb {
			if sa {
				return -1
			}
			return 1
		}
		if c := strings.Compare(a.ReleaseDate, b.ReleaseDate); c != 0 && a.ReleaseDate != "" && b.ReleaseDate != "" {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return cands[0], true
}

// onOriginal says the Apple track is on the song's original release: its album is the MusicBrainz original album
// (else the first release, for a song that only came out as a single) by name, from the original's time.
func onOriginal(t apple.Song, m musicbrainz.Match, year int) bool {
	g := m.CoverGroup()
	return g.ID != "" && musicbrainz.SameTitle(t.Album, g.Title) && originalYear(t.Year(), g, year)
}

// originalYear says an Apple release year fits the original: within a year of the card year or of the release
// group's date. The group's date is the earliest release of it the search showed, which can be a reissue
// (Waterloo's album shows as 2004), so the card year decides as well.
func originalYear(y int, g musicbrainz.Group, cardYear int) bool {
	return withinAYear(y, cardYear) || withinAYear(y, musicbrainz.Year(g.Date))
}

// ownAlbum says the track is on an album of its own artist.
func ownAlbum(t apple.Song) bool { return musicbrainz.SameArtist(t.AlbumArtist, t.Artist) }

// merge folds resolved entries that came out as the same song id (two card spellings of one recording) into one:
// the sets joined, the earliest card year kept.
func merge(rs []resolved) (out []resolved, merged []string) {
	byID := map[string]int{}
	for _, r := range rs {
		i, ok := byID[r.Song.ID]
		if !ok {
			byID[r.Song.ID] = len(out)
			out = append(out, r)
			continue
		}
		o := &out[i]
		merged = append(merged, fmt.Sprintf("| %s | %s | %s | %s |", cell(r.Artist), cell(r.Title), cell(o.Artist), cell(o.Title)))
		for _, s := range r.Song.Sets {
			if !slices.Contains(o.Song.Sets, s) {
				o.Song.Sets = append(o.Song.Sets, s)
			}
		}
		slices.Sort(o.Song.Sets)
		for _, y := range r.Years {
			if !slices.Contains(o.Years, y) {
				o.Years = append(o.Years, y)
			}
		}
		slices.Sort(o.Years)
		o.Song.Year = min(o.Song.Year, r.Song.Year)
		o.Year = o.Song.Year
	}
	return out, merged
}

// CoverArt checks the Cover Art Archive for a release group's front cover.
type CoverArt struct {
	BaseURL string
	// HTTP must not follow redirects (CheckRedirect = ErrUseLastResponse): the 307 is the answer.
	HTTP *http.Client
}

// CAAURL is the Cover Art Archive.
const CAAURL = "https://coverartarchive.org"

// Front returns the group's 500 px front-cover URL when the archive has one (it redirects to the image: 307, or
// 200), found false on 404.
func (c *CoverArt) Front(ctx context.Context, groupID string) (string, bool, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = CAAURL
	}
	u := base + "/release-group/" + groupID + "/front-500"
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return "", false, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("cover art archive: %w", err)
	}
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusTemporaryRedirect, http.StatusFound, http.StatusMovedPermanently,
		http.StatusSeeOther, http.StatusPermanentRedirect:
		return CAAURL + "/release-group/" + groupID + "/front-500", true, nil
	case http.StatusNotFound, http.StatusBadRequest:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("cover art archive: status %d", resp.StatusCode)
	}
}
