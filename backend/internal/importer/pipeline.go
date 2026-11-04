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

	"github.com/trick77/toolkit/applemusic"

	"github.com/trick77/reworm/internal/cards"
	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

// storefronts are tried in order; the one that answered is stored with the apple_id.
var storefronts = []string{"ch", "de", "us"}

// searchLimit is how many Apple search hits the artist+title fallback looks at.
const searchLimit = 25

// Catalog is what the pipeline asks Apple; *applemusic.Client implements it.
type Catalog interface {
	SongsByISRC(ctx context.Context, storefront string, isrcs []string) ([]applemusic.Song, error)
	SearchSongs(ctx context.Context, storefront, term string, limit int) ([]applemusic.Song, error)
	SearchAlbums(ctx context.Context, storefront, term string, limit int) ([]applemusic.Album, error)
	AlbumTracks(ctx context.Context, storefront, albumID string) ([]applemusic.Song, error)
	SongByID(ctx context.Context, storefront, id string) (applemusic.Song, bool, error)
}

// Recordings is what the pipeline asks MusicBrainz; *musicbrainz.Client implements it.
type Recordings interface {
	Search(ctx context.Context, artist, title string, until int) ([]musicbrainz.Recording, error)
}

// Pipeline resolves card songs against MusicBrainz, Apple and the Cover Art Archive, then Last.fm (listeners,
// genres; none when LastFM is nil).
type Pipeline struct {
	MB      Recordings
	Apple   Catalog
	Covers  *CoverArt
	LastFM  Listening
	Workers int
	Log     *slog.Logger
	// Previous, set offline, is the last dataset by card key: a song whose resolve would need the network
	// (httpcache.ErrOffline) keeps its entry from there; one not in it is left out.
	Previous map[string]song.Song
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
	// LastFM says Last.fm knows the track (by one of its readings).
	LastFM bool
	// Low lists why a match is low confidence; empty when it is not.
	Low []string
	// match and track are the MusicBrainz match and the Apple track: kept to redo the cover when the year or the
	// track changes after the resolve (a merge, an override).
	match musicbrainz.Match
	track applemusic.Song
	// trackYear is the song's year setTrack judged the track by; kept marks a song taken from the previous dataset.
	trackYear int
	kept      bool
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
//
// An Apple 429 stops the whole run at once: every worker's requests are cancelled, nothing more is asked of Apple,
// and the error counts the songs still missing (a rerun resolves them from the cache once the window is over).
func (p *Pipeline) resolveAll(ctx context.Context, entries []entry) ([]resolved, error) {
	parent := ctx
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	out := make([]resolved, len(entries))
	errs := make([]error, len(entries))
	ok := make([]bool, len(entries))
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for range max(p.Workers, 1) {
		wg.Go(func() {
			for i := range jobs {
				out[i], errs[i] = p.resolve(ctx, entries[i])
				if errors.Is(errs[i], httpcache.ErrOffline) && p.Previous != nil {
					out[i], errs[i] = p.previous(entries[i])
				}
				ok[i] = errs[i] == nil
				if errors.Is(errs[i], applemusic.ErrRateLimited) {
					stop(errs[i])
				}
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
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if cause := context.Cause(ctx); errors.Is(cause, applemusic.ErrRateLimited) {
		missing := 0
		for _, k := range ok {
			if !k {
				missing++
			}
		}
		return nil, fmt.Errorf("stopped at the first Apple 429, %d of %d songs still missing (rerun later; the rest is cached): %w",
			missing, len(entries), cause)
	}
	var all []error
	kept := out[:0]
	left := 0
	for i, err := range errs {
		switch {
		case errors.Is(err, errLeftOut):
			left++
		case err != nil:
			all = append(all, fmt.Errorf("%s – %s: %w", entries[i].Artist, entries[i].Title, err))
		default:
			kept = append(kept, out[i])
		}
	}
	if len(all) > 0 {
		return nil, fmt.Errorf("%d songs failed (re-run: answers so far are cached): %w", len(all), errors.Join(all[:min(len(all), 10)]...))
	}
	if left > 0 {
		p.log().Warn("offline: songs that need the network and have no previous entry left out", "songs", left)
	}
	return kept, nil
}

// errLeftOut marks a song an offline run cannot resolve and has no previous entry for.
var errLeftOut = errors.New("offline and not in the previous dataset")

// previousNote is the low-confidence note of a song an offline run kept from the previous dataset.
const previousNote = "kept from the previous import (offline, needs the network)"

// previous is e as the previous dataset has it, its sets and year from the cards as they are now.
func (p *Pipeline) previous(e entry) (resolved, error) {
	s, ok := p.Previous[song.Key(e.Artist, e.Title)]
	if !ok {
		s, ok = p.Previous[song.Key(e.Artist, musicbrainz.CleanTitle(e.Title))]
	}
	if !ok {
		return resolved{}, errLeftOut
	}
	s.Title, s.Sets = musicbrainz.CleanTitle(e.Title), e.Sets
	if s.YearSource == song.YearCard {
		s.Year = e.Year
	}
	return resolved{entry: e, Song: s, MBMatched: s.MBID != "", AppleOriginal: s.CoverSrc == song.CoverApple,
		Low: []string{previousNote}, trackYear: s.Year, kept: true}, nil
}

// rejudge redoes the original-album flag and the cover after the song's year moved (a merge, a year override).
// A song kept from the previous dataset has no track to judge: it stays as it was.
func (p *Pipeline) rejudge(ctx context.Context, r *resolved) error {
	if r.kept || r.Song.Year == r.trackYear {
		return nil
	}
	return p.setTrack(ctx, r, r.track, r.Song.Storefront)
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
	// The title as shown: release notes the card list copied from a streaming title ("Heroes - 2017 Remaster", 137
	// of them) dropped. The id's slug keeps the card's spelling, so it does not move.
	s := song.Song{Artist: e.Artist, Title: musicbrainz.CleanTitle(e.Title), Year: e.Year, YearSource: song.YearCard, Sets: e.Sets}

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
func (p *Pipeline) setTrack(ctx context.Context, r *resolved, t applemusic.Song, sf string) error {
	s, m := &r.Song, r.match
	r.track, r.trackYear = t, s.Year
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
// writes it, then with the main artist and each other reading of the title (musicbrainz.SongTitles); then without
// the year bound; last, missed by all of those or matched there only undated or later than the card, with the
// card's first name and each reading (MusicBrainz finds "Queen, David Bowie" only as Queen), the pick checked
// against the card's whole artist: "Earth" or "Simon" alone is not the card's. Only the first search can match
// exactly; the others are low confidence, why says how.
func (p *Pipeline) findRecording(ctx context.Context, e entry) (m musicbrainz.Match, ok bool, why string, err error) {
	type query struct{ artist, title string }
	titles := musicbrainz.SongTitles(e.Title, e.Artist)
	main := musicbrainz.MainArtist(e.Artist)
	queries := []query{{e.Artist, e.Title}}
	for _, t := range titles {
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
	// The first name's searches also when the unbounded match is weak, undated or after the card's time: theirs are
	// bounded by it, so a pick there is the better one.
	if lead := musicbrainz.Lead(e.Artist); lead != main && (!ok || m.Recording.FirstReleaseDate == "" || m.Year > e.Year+1) {
		for _, t := range titles {
			recs, err := p.MB.Search(ctx, lead, t, e.Year+1)
			if err != nil {
				return m, false, "", err
			}
			if lm, lok := musicbrainz.Pick(recs, e.Artist, t, e.Year); lok {
				lm.Exact = false
				return lm, true, "MusicBrainz matched as " + lead + " – " + t, nil
			}
		}
	}
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
// ISRC lookup, then (ch only) the original album found by name. The first original-album track wins; else the
// first storefront's best ISRC hit; else an artist+title search in ch and us (viaSearch: low confidence). Every
// hit must be the card's song by title and artist (target). A live take, remix, … the card does not name (one the
// matched recording is: a song MusicBrainz knows only from "MTV Unplugged") is taken last, when no plain take turned
// up anywhere. Apple's search endpoint has a quota of its own: a full import that searched every storefront hit
// 429s that lasted ~3.5 h (2026-11-04), while the ISRC lookups kept answering.
func (p *Pipeline) findTrack(ctx context.Context, e entry, m musicbrainz.Match) (t applemusic.Song, sf string, viaSearch bool, err error) {
	var fallback, weak applemusic.Song
	var fallbackSF, weakSF string
	weakSearch := false
	keepWeak := func(t applemusic.Song, sf string, search bool) {
		if weak.ID == "" {
			weak, weakSF, weakSearch = t, sf, search
		}
	}
	tg := newTarget(e, m)
	for i, sf := range storefronts {
		if len(m.ISRCs) > 0 {
			songs, err := p.Apple.SongsByISRC(ctx, sf, m.ISRCs)
			if err != nil {
				return applemusic.Song{}, "", false, err
			}
			if t, ok := best(songs, m, e, tg.isrcHit); ok {
				switch {
				case !plainTake(t, e):
					keepWeak(t, sf, false)
				case onOriginal(t, m, e.Year):
					return t, sf, false, nil
				case fallback.ID == "":
					fallback, fallbackSF = t, sf
				}
			}
		}
		if i > 0 {
			continue
		}
		t, ok, err := p.originalAlbumTrack(ctx, sf, e, m, tg)
		if err != nil {
			return applemusic.Song{}, "", false, err
		}
		if ok && plainTake(t, e) {
			return t, sf, false, nil
		}
		if ok {
			keepWeak(t, sf, false)
		}
	}
	if fallback.ID != "" {
		return fallback, fallbackSF, false, nil
	}
	titles := tg.titles
	if m.Recording.Title != "" && !slices.Contains(titles, m.Recording.Title) {
		titles = append([]string{m.Recording.Title}, titles...)
	}
	// A hit counts by any reading of the card title, not just the one searched: "Luis Bacalov Django Unchained
	// (Django)" finds "Django".
	for _, title := range titles[:min(len(titles), searchTitles)] {
		for _, sf := range searchStorefronts {
			songs, err := p.Apple.SearchSongs(ctx, sf, tg.by.Search()+" "+title, searchLimit)
			if err != nil {
				return applemusic.Song{}, "", false, err
			}
			if t, ok := best(songs, m, e, tg.searchHit); ok {
				if plainTake(t, e) {
					return t, sf, true, nil
				}
				keepWeak(t, sf, true)
			}
		}
	}
	return weak, weakSF, weakSearch, nil
}

// plainTake says a track's name marks no live take, remix, … the card title does not ask for.
func plainTake(t applemusic.Song, e entry) bool { return !musicbrainz.Version(t.Name, e.Title) }

// target is the card's song as Apple hits are checked against it, worked out once per song: the card title's
// readings (musicbrainz.SongTitles), their keys with the matched recording's title, and who it is by.
type target struct {
	titles []string
	keys   [][2]string
	by     musicbrainz.Credit
}

func newTarget(e entry, m musicbrainz.Match) target {
	t := target{titles: musicbrainz.SongTitles(e.Title, e.Artist, m.Recording.Credit),
		by: musicbrainz.NewCredit(e.Artist, e.Title, m.Recording)}
	for _, s := range append(slices.Clip(t.titles), m.Recording.Title) {
		if s != "" {
			full, stripped := musicbrainz.TitleKey(s)
			t.keys = append(t.keys, [2]string{full, stripped})
		}
	}
	return t
}

// title says an Apple track's name is one of the readings or the recording's title (musicbrainz.SameTitle).
func (t target) title(s applemusic.Song) bool {
	full, stripped := musicbrainz.TitleKey(s.Name)
	return slices.ContainsFunc(t.keys, func(k [2]string) bool { return k[0] == full || k[1] == stripped })
}

// isrcHit says a track found by ISRC is the song: its title, and a credit led by the song's main artist at least
// (Credit.Led). Apple files unrelated tracks under a famous ISRC (Yesterday's returned a 2023 single by another act).
func (t target) isrcHit(s applemusic.Song) bool { return t.title(s) && t.by.Led(s.Artist) }

// searchHit says a track found by search is the song: its title and its artist (Credit.By).
func (t target) searchHit(s applemusic.Song) bool { return t.title(s) && t.by.By(s.Artist) }

// searchTitles bounds the title readings the Apple search tries.
const searchTitles = 2

// searchStorefronts are where the Apple search runs: de's catalog is ch's in all but a few licences.
var searchStorefronts = []string{"ch", "us"}

// albumsTried bounds the albums of one name whose tracks are read (Apple lists remasters and deluxe editions too).
const albumsTried = 2

// originalAlbumTrack finds the song on the MusicBrainz original album in Apple's catalog: an album search by
// artist and album title, then the album's tracks by ISRC, else by title. The ISRC lookup alone misses it for big
// hits: Apple answers at most 25 tracks per ISRC, mostly compilations.
func (p *Pipeline) originalAlbumTrack(ctx context.Context, sf string, e entry, m musicbrainz.Match, tg target) (applemusic.Song, bool, error) {
	g := m.CoverGroup()
	if g.ID == "" {
		return applemusic.Song{}, false, nil
	}
	albums, err := p.Apple.SearchAlbums(ctx, sf, tg.by.Search()+" "+musicbrainz.Strip(g.Title), 10)
	if err != nil {
		return applemusic.Song{}, false, err
	}
	tried := 0
	for _, a := range albums {
		if tried == albumsTried {
			break
		}
		if !musicbrainz.SameTitle(a.Name, g.Title) || !tg.by.By(a.Artist) ||
			!originalYear(a.Year(), g, e.Year) {
			continue
		}
		tried++
		tracks, err := p.Apple.AlbumTracks(ctx, sf, a.ID)
		if err != nil {
			return applemusic.Song{}, false, err
		}
		for _, byISRC := range []bool{true, false} {
			for _, t := range tracks {
				hit := slices.Contains(m.ISRCs, t.ISRC)
				if !byISRC {
					hit = tg.title(t) && !musicbrainz.Version(t.Name, recordingIs(e, m))
				}
				if hit && t.Preview {
					return t, true, nil
				}
			}
		}
	}
	return applemusic.Song{}, false, nil
}

// withinAYear says two years are at most one apart; an unknown (0) one passes.
func withinAYear(a, b int) bool { return a == 0 || b == 0 || (a-b <= 1 && b-a <= 1) }

// best picks among songs those with a preview (and passing keep, when set), never a track whose name marks a live
// take, remix, re-recording, … neither the card nor the matched recording is (recordingIs): no clip beats a wrong
// recording ("Father and Son (Re-Recorded Version)" for the 1970 card, "Glory … [Live]"). Then a plain take (by
// the card title, plainTake) before such a version the recording is, then the original-album track, then one on no
// album named like a version either, then one on the artist's own album (not a various-artists compilation), then
// the earliest release, then the lowest id.
func best(songs []applemusic.Song, m musicbrainz.Match, e entry, keep func(applemusic.Song) bool) (applemusic.Song, bool) {
	is := recordingIs(e, m)
	var cands []applemusic.Song
	for _, s := range songs {
		if s.Preview && !musicbrainz.Version(s.Name, is) && (keep == nil || keep(s)) {
			cands = append(cands, s)
		}
	}
	if len(cands) == 0 {
		return applemusic.Song{}, false
	}
	// By the card title only: "Layla (Live)" asks for the live one.
	plain := func(s applemusic.Song) bool { return !musicbrainz.Version(s.Album, e.Title) }
	slices.SortFunc(cands, func(a, b applemusic.Song) int {
		if pa, pb := plainTake(a, e), plainTake(b, e); pa != pb {
			if pa {
				return -1
			}
			return 1
		}
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

// recordingIs is what names the version the song is: the card title ("Layla (Live)") and the matched recording's
// title and note, for a song MusicBrainz knows only live ("MTV Unplugged"). Such a take still yields to a plain one
// (plainTake): MusicBrainz takes a remix for the song when only it is credited exactly as the card ("One Kiss …
// Jauz Remix").
func recordingIs(e entry, m musicbrainz.Match) string {
	return e.Title + " " + m.Recording.Title + " " + m.Recording.Disambiguation
}

// onOriginal says the Apple track is on the song's original release: its album is the MusicBrainz original album
// (else the first release, for a song that only came out as a single) by name, from the original's time.
func onOriginal(t applemusic.Song, m musicbrainz.Match, year int) bool {
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
func ownAlbum(t applemusic.Song) bool { return musicbrainz.SameArtist(t.AlbumArtist, t.Artist) }

// merge folds resolved entries that came out as the same song id (two card spellings of one recording) into one:
// the sets joined, the earliest card year kept. A song whose year moved has its track's original-album flag and
// cover redone: both were judged against the year it had.
func (p *Pipeline) merge(ctx context.Context, rs []resolved) (out []resolved, merged []string, err error) {
	out, merged = merge(rs)
	for i := range out {
		if err := p.rejudge(ctx, &out[i]); err != nil {
			return nil, nil, err
		}
	}
	return out, merged, nil
}

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
