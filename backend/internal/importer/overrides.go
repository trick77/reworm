package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/trick77/reworm/internal/song"
)

// overridesDoc explains the file in the file: JSON has no comments, the loader skips this key.
const overridesDoc = "Hand fixes by song id (see data/songs.jsonl), applied after the import: " +
	`"songs": {"<id>": {"year": 1976, "apple_id": "1422648512", "storefront": "ch", "genres": ["pop"], "exclude": true}}. ` +
	"year replaces the card year (year_source becomes override); apple_id (+ storefront, default ch) picks the Apple " +
	"track, its album, ISRC and cover follow from it; genres replaces the genres from Last.fm ([] for none); exclude " +
	"drops the song from the dataset. Every field is optional."

// Override is one hand fix. Genres nil leaves the song's; an empty list clears them.
type Override struct {
	Year       int      `json:"year,omitempty"`
	AppleID    string   `json:"apple_id,omitempty"`
	Storefront string   `json:"storefront,omitempty"`
	Genres     []string `json:"genres,omitempty"`
	Exclude    bool     `json:"exclude,omitempty"`
}

type overridesFile struct {
	Doc   string              `json:"_doc"`
	Songs map[string]Override `json:"songs"`
}

// loadOverrides reads path; a missing file is no overrides.
func loadOverrides(path string) (map[string]Override, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the repo's own data file
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]Override{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f overridesFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("overrides: %w", err)
	}
	if f.Songs == nil {
		f.Songs = map[string]Override{}
	}
	for id, o := range f.Songs {
		if o.Year != 0 && !song.ValidYear(o.Year) {
			return nil, fmt.Errorf("overrides: %s: year %d", id, o.Year)
		}
		for _, g := range o.Genres {
			if !knownGenre(g) {
				return nil, fmt.Errorf("overrides: %s: genre %q is none of the import's", id, g)
			}
		}
	}
	return f.Songs, nil
}

// templateOverrides is the empty file the import writes when there is none.
func templateOverrides() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // "<id>" stays readable
	enc.SetIndent("", "  ")
	err := enc.Encode(overridesFile{Doc: overridesDoc, Songs: map[string]Override{}})
	return buf.Bytes(), err
}

// applyOverrides applies ov to rs and returns the songs kept, the ids applied and the unknown ones. An apple_id is
// looked up like any track (cached): its album, ISRC and the cover follow from it, the original-album rule
// included. One that names no track with a preview fails the run: a hand fix must not leave the song unplayable.
func (p *Pipeline) applyOverrides(ctx context.Context, rs []resolved, ov map[string]Override) (kept []resolved, applied, unknown []string, err error) {
	seen := map[string]bool{}
	for _, r := range rs {
		o, ok := ov[r.Song.ID]
		if !ok {
			kept = append(kept, r)
			continue
		}
		seen[r.Song.ID] = true
		applied = append(applied, r.Song.ID)
		if o.Exclude {
			continue
		}
		if o.Year != 0 {
			r.Song.Year, r.Song.YearSource = o.Year, song.YearOverride
		}
		if o.Genres != nil {
			r.Song.Genres = slices.Sorted(slices.Values(o.Genres))
		}
		if o.AppleID != "" {
			sf := o.Storefront
			if sf == "" {
				sf = storefronts[0]
			}
			t, found, err := p.Apple.SongByID(ctx, sf, o.AppleID)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("override %s: %w", r.Song.ID, err)
			}
			if !found || !t.Preview {
				return nil, nil, nil, fmt.Errorf("override %s: apple_id %s is no track with a preview in storefront %s",
					r.Song.ID, o.AppleID, sf)
			}
			r.Low = slices.DeleteFunc(slices.Clone(r.Low), func(n string) bool { return strings.HasPrefix(n, searchNote) })
			if err := p.setTrack(ctx, &r, t, sf); err != nil {
				return nil, nil, nil, err
			}
		} else if err := p.rejudge(ctx, &r); err != nil {
			// A year alone moves what counts as the original album: the track's flag and cover are judged anew.
			return nil, nil, nil, err
		}
		kept = append(kept, r)
	}
	for id := range ov {
		if !seen[id] {
			unknown = append(unknown, id)
		}
	}
	slices.Sort(applied)
	slices.Sort(unknown)
	return kept, applied, unknown, nil
}
