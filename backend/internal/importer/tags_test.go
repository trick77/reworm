package importer

import (
	"context"
	"slices"
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

// Offline, a song whose Last.fm answers are not cached keeps the previous dataset's genres and listeners, found by
// the card key as the resolve finds it: the cleaned title too. One not in it has none.
func TestTag_offlineKeepsThePrevious(t *testing.T) {
	p := &Pipeline{LastFM: offlineLastFM{}, Previous: map[string]song.Song{
		song.Key("Lonely", "Ballad"): {Genres: []string{"pop"}, Listeners: 12},
	}}
	rs := []resolved{
		{entry: entry{Artist: "Lonely", Title: "Ballad - 2017 Remaster"}, Song: song.Song{Title: "Ballad"}},
		{entry: entry{Artist: "New", Title: "Song"}, Song: song.Song{Title: "Song", Genres: []string{"rock"}, Listeners: 5}},
	}
	if err := p.tagAll(context.Background(), rs); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rs[0].Song.Genres, []string{"pop"}) || rs[0].Song.Listeners != 12 || !rs[0].LastFM {
		t.Errorf("kept = %+v", rs[0].Song)
	}
	if len(rs[1].Song.Genres) != 0 || rs[1].Song.Listeners != 0 || rs[1].LastFM {
		t.Errorf("unknown = %+v", rs[1].Song)
	}
}
