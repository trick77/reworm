package importer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/trick77/ship/kit/logging"

	"github.com/trick77/reworm/internal/charts"
	"github.com/trick77/reworm/internal/httpcache"
	"github.com/trick77/reworm/internal/lastfm"
	"github.com/trick77/reworm/internal/musicbrainz"
	"github.com/trick77/reworm/internal/song"
)

// chartStats counts one chart year's rows: all, and those a card names by the same artist + title.
type chartStats struct {
	Rows, OnCard int
}

// resolveChart finds a chart song. Its year is MusicBrainz's first release, kept only where Apple's track agrees
// within a year: no trustworthy year, no song. Released before ChartFrom, it is a recurrent: left out before
// Apple is asked. A dropped song says why (Drop) and stays out of the dataset after merge (a card of the same
// recording still wins there).
func (p *Pipeline) resolveChart(ctx context.Context, e entry) (resolved, error) {
	r := resolved{entry: e}
	r.Song = song.Song{ID: song.Slug(e.Artist, e.Title), Artist: e.Artist, Title: musicbrainz.CleanTitle(e.Title)}
	m, ok, why, err := p.findRecording(ctx, e)
	switch {
	case err != nil && (errors.Is(err, httpcache.ErrOffline) || ctx.Err() != nil):
		return r, err
	case err != nil:
		// MusicBrainz answers some searches with 503 every time, after every retry (Taylor Swift – Is It Over Now?
		// (Taylor's Version), Ed Sheeran & Elton John – Merry Christmas, 2026-11-22): a chart song is no reason to
		// fail the run, it is left out and listed.
		p.log().Warn("chart song left out, MusicBrainz failed", "artist", e.Artist, "title", e.Title, logging.Err(err))
		r.Drop = "MusicBrainz failed: " + err.Error()
		return r, nil
	}
	r.match = m
	if !ok || m.Year == 0 {
		r.Drop = "MusicBrainz knows no dated recording"
		return r, nil
	}
	r.MBMatched, r.MBYear = true, m.Year
	switch {
	case why != "":
		r.Low = append(r.Low, why)
	case !m.Exact:
		r.Low = append(r.Low, "MusicBrainz artist matched loosely ("+m.Recording.Credit+")")
	}
	r.Song.ID, r.Song.MBID = m.Recording.ID, m.Recording.ID
	r.Song.Year, r.Song.YearSource = m.Year, song.YearMusicBrainz
	if m.Year < p.ChartFrom {
		r.Drop = fmt.Sprintf("released %d, before %d", m.Year, p.ChartFrom)
		return r, nil
	}
	// The track is judged by the release year, as a card song's by the card year.
	r.Year = m.Year
	track, sf, viaSearch, err := p.findTrack(ctx, r.entry, m)
	if err != nil {
		return r, err
	}
	if viaSearch {
		r.Low = append(r.Low, searchNote+" ("+track.Artist+" – "+track.Name+")")
	}
	if err := p.setTrack(ctx, &r, track, sf); err != nil {
		return r, err
	}
	switch {
	case track.ID == "":
		r.Drop = "no Apple track with a preview"
	case !withinAYear(r.AppleYear, m.Year):
		r.Drop = fmt.Sprintf("Apple dates it %d, MusicBrainz %d", r.AppleYear, m.Year)
	}
	return r, nil
}

// withoutDropped splits off the chart songs left out.
func withoutDropped(rs []resolved) (kept, dropped []resolved) {
	for _, r := range rs {
		if r.Drop != "" {
			dropped = append(dropped, r)
		} else {
			kept = append(kept, r)
		}
	}
	return kept, dropped
}

// withoutTwins folds a chart song into a song of the same first artist and title (twinKey) kept before it, a card
// song first: chart spellings can match two MusicBrainz recordings of one song ("Huntr/x feat. Ejae, …" and
// "Huntr/x (Ejae, …)": Golden twice, 2026-11-22), which merge by id does not join. The twin only adds its
// sources. Offline the twin is no previous song (only the kept one was written): it stays out there too.
func withoutTwins(rs []resolved) []resolved {
	out := make([]resolved, 0, len(rs))
	at := map[string]int{}
	for _, chart := range []bool{false, true} {
		for _, r := range rs {
			if r.Chart != chart {
				continue
			}
			k := twinKey(r)
			if i, ok := at[k]; ok && r.Chart {
				out[i].Sources = union(out[i].Sources, r.Sources)
				continue
			}
			if _, ok := at[k]; !ok {
				at[k] = len(out)
			}
			out = append(out, r)
		}
	}
	return out
}

// twinKey is a song's first artist (MusicBrainz's when matched) and its title without version notes, normalized.
func twinKey(r resolved) string {
	artist := musicbrainz.MainArtist(r.Artist)
	if as := r.match.Recording.Artists; len(as) > 0 {
		artist = as[0]
	}
	_, title := musicbrainz.TitleKey(r.Song.Title)
	return song.Normalize(artist) + "|" + title
}

// billboardMinListeners is the Last.fm listeners a song only on the Billboard list needs, by release year (a later
// year: the latest's). The US year-end list carries country and rap hardly known here; the Swiss, German,
// Austrian and UK lists need none. A newer song had less time to gather listeners, hence per year. Between the kept
// p10 and the left-out p90 of the run of 2026-11-22 (2023: 437k / 342k, 2024: 439k / 214k, 2025: 304k / 181k);
// the report's chart section shows them per run.
var billboardMinListeners = map[int]int{2023: 400_000, 2024: 300_000, 2025: 200_000}

// minListeners is billboardMinListeners for year: the latest year's at or before it, else the earliest year's.
func minListeners(year int) int {
	ys := slices.Sorted(maps.Keys(billboardMinListeners))
	n := billboardMinListeners[ys[0]]
	for _, y := range ys {
		if y <= year {
			n = billboardMinListeners[y]
		}
	}
	return n
}

// billboardOnly says a song's only chart is Billboard's.
func billboardOnly(r resolved) bool { return slices.Equal(r.Sources, []string{charts.Billboard}) }

// placeCharts drops the Billboard-only chart songs below minListeners (needs tagAll first) and gives every other
// chart song its sets (chartSets) and their genres. A song kept from the previous dataset (offline) keeps its
// stored sets: its track tags are not at hand.
func placeCharts(rs []resolved, configured func(set string) bool) (kept, dropped []resolved) {
	for _, r := range rs {
		if !r.Chart {
			kept = append(kept, r)
			continue
		}
		if billboardOnly(r) && r.Song.Listeners < minListeners(r.Song.Year) {
			r.Drop = fmt.Sprintf("only on Billboard, %d Last.fm listeners (needs %d)", r.Song.Listeners, minListeners(r.Song.Year))
			dropped = append(dropped, r)
			continue
		}
		if !r.kept {
			r.Song.Sets = chartSets(r.Song.Genres, r.christmas || christmasTitle(r.Song.Title), configured)
		}
		r.Song.Genres = withSetGenres(r.Song.Genres, r.Song.Sets)
		kept = append(kept, r)
	}
	return kept, dropped
}

// chartSets are the card sets a chart song joins: Mega Mix, the all-round set, always; Rock Anthems for rock or
// metal that is no pop, rap, dance, R&B or Latin crossover as well (Imagine Dragons are pop, rock and indie);
// Schlager Fest for schlager; Christmas Hits by a Last.fm "christmas" tag or the title. Only sets the import is
// configured with.
func chartSets(genres []string, christmas bool, configured func(set string) bool) []string {
	has := func(gs ...string) bool {
		return slices.ContainsFunc(gs, func(g string) bool { return slices.Contains(genres, g) })
	}
	sets := []string{"mega-mix"}
	if has("rock", "metal") && !has("pop", "hip-hop", "dance", "rnb-soul", "latin") {
		sets = append(sets, "rock-anthems")
	}
	if has("schlager") {
		sets = append(sets, "schlager-fest")
	}
	if christmas {
		sets = append(sets, "christmas-hits")
	}
	sets = slices.DeleteFunc(sets, func(s string) bool { return !configured(s) })
	slices.Sort(sets)
	return sets
}

// christmasTitle says a title names Christmas: a new Christmas song has no Last.fm tag yet (Kylie Minogue – XMAS,
// 2026-11-22: dance, pop).
func christmasTitle(title string) bool {
	t := song.Normalize(title)
	return slices.ContainsFunc(strings.Fields(t), func(w string) bool {
		return strings.HasPrefix(w, "christmas") || w == "xmas" || strings.HasPrefix(w, "weihnacht")
	})
}

// christmasTag says a track's top tags (trackTags) name Christmas.
func christmasTag(tags []lastfm.Tag) bool {
	return slices.ContainsFunc(tags[:min(len(tags), trackTags)], func(t lastfm.Tag) bool {
		k := tagKey(t.Name)
		return strings.Contains(k, "christmas") || k == "xmas"
	})
}
