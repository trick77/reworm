package importer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/lastfm"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

// Listening is what the pipeline asks Last.fm; *lastfm.Client implements it.
type Listening interface {
	Track(ctx context.Context, artist, title string) (lastfm.Track, bool, error)
	ArtistTags(ctx context.Context, artist string) ([]lastfm.Tag, error)
}

// tagAll sets every song's Last.fm listeners and genres (tag), on Workers goroutines (Last.fm's throttle serializes
// them); a song skip names (one an override excludes) is not looked up. Any error fails the run; the cache makes
// the next one cheap. A refused key (lastfm.ErrKey) stops the run at once: every further request would be refused.
func (p *Pipeline) tagAll(ctx context.Context, rs []resolved, skip func(id string) bool) error {
	if p.LastFM == nil {
		return nil
	}
	parent := ctx
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	errs := make([]error, len(rs))
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for range max(p.Workers, 1) {
		wg.Go(func() {
			for i := range jobs {
				errs[i] = p.tag(ctx, &rs[i])
				if errors.Is(errs[i], lastfm.ErrKey) {
					stop(errs[i])
				}
				mu.Lock()
				done++
				if done%250 == 0 || done == len(rs) {
					p.log().Info("Last.fm progress", "done", done, "of", len(rs))
				}
				mu.Unlock()
			}
		})
	}
	for i := range rs {
		if ctx.Err() != nil {
			break
		}
		if skip != nil && skip(rs[i].Song.ID) {
			continue
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if err := parent.Err(); err != nil {
		return err
	}
	if cause := context.Cause(ctx); errors.Is(cause, lastfm.ErrKey) {
		return cause
	}
	var all []error
	for i, err := range errs {
		if err != nil {
			all = append(all, fmt.Errorf("%s – %s: %w", rs[i].Artist, rs[i].Title, err))
		}
	}
	if len(all) > 0 {
		return fmt.Errorf("lastfm: %d songs failed (re-run: answers so far are cached): %w", len(all),
			errors.Join(all[:min(len(all), 10)]...))
	}
	return nil
}

// tag looks the song up on Last.fm by its readings in order (readings): the first track found counts, its listeners
// are the song's, its top tags give the genres; no genre among them (Udo Lindenberg's tracks carry no tags), the
// artist's top tags do. Offline, a song whose answers are not cached keeps what the previous dataset has, its sets'
// genres as they are now added (a set it left keeps its genre until the next online run).
func (p *Pipeline) tag(ctx context.Context, r *resolved) error {
	err := p.tagOnline(ctx, r)
	if !errors.Is(err, httpcache.ErrOffline) {
		return err
	}
	prev, ok := r.Song, r.kept
	if !ok {
		// By the card key as previous does: as the card writes the title, else cleaned ("Ballad - 2017 Remaster").
		pr, err := p.previous(r.entry)
		prev, ok = pr.Song, err == nil
	}
	var gs []string
	r.Song.Listeners, r.LastFM = 0, false
	if ok {
		gs, r.Song.Listeners, r.LastFM = prev.Genres, prev.Listeners, prev.Listeners > 0
	}
	r.Song.Genres = withSetGenres(gs, r.Song.Sets)
	return nil
}

func (p *Pipeline) tagOnline(ctx context.Context, r *resolved) error {
	// The first reading Last.fm knows: a looser one (the main artist alone) can autocorrect to another, more
	// listened track, whose listeners and tags are not the song's.
	var best lastfm.Track
	found := false
	for _, q := range readings(*r) {
		t, ok, err := p.LastFM.Track(ctx, q[0], q[1])
		if err != nil {
			return err
		}
		if ok {
			best, found = t, true
			break
		}
	}
	artist := musicbrainz.MainArtist(r.Artist)
	if found && best.Artist != "" {
		artist = best.Artist
	}
	var artistTags []lastfm.Tag
	fromTrack := trackGenres(best.Tags)
	if len(fromTrack) == 0 {
		var err error
		if artistTags, err = p.LastFM.ArtistTags(ctx, artist); err != nil {
			return err
		}
	}
	r.LastFM, r.Song.Listeners = found, best.Listeners
	r.Song.Genres = genres(r.Song.Sets, fromTrack, artistTags)
	return nil
}

// readings are the artist + title pairs a song is looked up by on Last.fm, each once (song.Key): as the card
// writes it, with the main artist, as the MusicBrainz recording is credited.
func readings(r resolved) [][2]string {
	var out [][2]string
	seen := map[string]bool{}
	add := func(artist, title string) {
		k := song.Key(artist, title)
		if artist == "" || title == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, [2]string{artist, title})
	}
	add(r.Artist, r.Song.Title)
	add(musicbrainz.MainArtist(r.Artist), r.Song.Title)
	if r.MBMatched {
		add(r.match.Recording.Credit, r.match.Recording.Title)
	}
	return out
}
