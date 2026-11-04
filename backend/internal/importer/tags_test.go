package importer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/lastfm"
	"github.com/trick77/reworm/internal/song"
)

// offlineLastFM answers nothing from the network.
type offlineLastFM struct{}

func (offlineLastFM) Track(context.Context, string, string) (lastfm.Track, bool, error) {
	return lastfm.Track{}, false, httpcache.ErrOffline
}

func (offlineLastFM) ArtistTags(context.Context, string) ([]lastfm.Tag, error) {
	return nil, httpcache.ErrOffline
}

// refusingLastFM refuses the key, counting the requests.
type refusingLastFM struct{ calls *atomic.Int32 }

func (f refusingLastFM) Track(context.Context, string, string) (lastfm.Track, bool, error) {
	f.calls.Add(1)
	return lastfm.Track{}, false, fmt.Errorf("%w: error 10", lastfm.ErrKey)
}

func (f refusingLastFM) ArtistTags(context.Context, string) ([]lastfm.Tag, error) {
	f.calls.Add(1)
	return nil, lastfm.ErrKey
}

// A refused key stops the run at once: not one request per song.
func TestTagAll_stopsAtARefusedKey(t *testing.T) {
	var calls atomic.Int32
	p := &Pipeline{LastFM: refusingLastFM{&calls}, Workers: 1}
	rs := make([]resolved, 50)
	for i := range rs {
		rs[i] = resolved{entry: entry{Artist: "A", Title: fmt.Sprint(i)}, Song: song.Song{Title: fmt.Sprint(i)}}
	}
	err := p.tagAll(context.Background(), rs, nil)
	if !errors.Is(err, lastfm.ErrKey) || calls.Load() > 3 {
		t.Errorf("err %v after %d requests", err, calls.Load())
	}
}

// An override's genres keep the set's genre.
func TestApplyOverrides_genresKeepTheSetGenre(t *testing.T) {
	rs := []resolved{{Song: song.Song{ID: "s", Sets: []string{"schlager-fest"}, Genres: []string{"rock", "schlager"}}}}
	kept, _, _, err := (&Pipeline{}).applyOverrides(context.Background(), rs, map[string]Override{"s": {Genres: []string{"pop"}}})
	if err != nil || !slices.Equal(kept[0].Song.Genres, []string{"pop", "schlager"}) {
		t.Errorf("genres %v, %v", kept[0].Song.Genres, err)
	}
}

// Offline, a song whose Last.fm answers are not cached keeps the previous dataset's genres and listeners, found by
// the card key as the resolve finds it: the cleaned title too. One not in it has none.
func TestTag_offlineKeepsThePrevious(t *testing.T) {
	p := &Pipeline{LastFM: offlineLastFM{}, Previous: map[string]song.Song{
		song.Key("Lonely", "Ballad"): {Genres: []string{"pop"}, Listeners: 12},
	}}
	rs := []resolved{
		{entry: entry{Artist: "Lonely", Title: "Ballad - 2017 Remaster"}, Song: song.Song{Title: "Ballad", Sets: []string{"schlager-fest"}}},
		{entry: entry{Artist: "New", Title: "Song"}, Song: song.Song{Title: "Song", Genres: []string{"pop"}, Listeners: 5}},
		{entry: entry{Artist: "New", Title: "Rocker"}, Song: song.Song{Title: "Rocker", Sets: []string{"rock-anthems"}}},
		{entry: entry{Artist: "Gone", Title: "Song"}, Song: song.Song{ID: "gone", Genres: []string{"pop"}}},
	}
	if err := p.tagAll(context.Background(), rs, func(id string) bool { return id == "gone" }); err != nil {
		t.Fatal(err)
	}
	// Kept, its set's genre added as the sets are now.
	if !slices.Equal(rs[0].Song.Genres, []string{"pop", "schlager"}) || rs[0].Song.Listeners != 12 || !rs[0].LastFM {
		t.Errorf("kept = %+v", rs[0].Song)
	}
	if len(rs[1].Song.Genres) != 0 || rs[1].Song.Listeners != 0 || rs[1].LastFM {
		t.Errorf("unknown = %+v", rs[1].Song)
	}
	if !slices.Equal(rs[2].Song.Genres, []string{"rock"}) {
		t.Errorf("unknown in a set with a genre = %+v", rs[2].Song)
	}
	// Skipped (an excluded song): not looked up, left as it was.
	if !slices.Equal(rs[3].Song.Genres, []string{"pop"}) {
		t.Errorf("skipped = %+v", rs[3].Song)
	}
}
