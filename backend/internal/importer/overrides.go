package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/trick77/reworm/internal/song"
)

// overridesDoc explains the file in the file: JSON has no comments, the loader skips this key.
const overridesDoc = "Hand fixes by song id (see data/songs.jsonl), applied after the import: " +
	`"songs": {"<id>": {"year": 1976, "apple_id": "1422648512", "storefront": "ch", "exclude": true}}. ` +
	"year replaces the card year (year_source becomes override); apple_id (+ storefront, default ch) picks the Apple " +
	"track; exclude drops the song from the dataset. Every field is optional."

// Override is one hand fix.
type Override struct {
	Year       int    `json:"year,omitempty"`
	AppleID    string `json:"apple_id,omitempty"`
	Storefront string `json:"storefront,omitempty"`
	Exclude    bool   `json:"exclude,omitempty"`
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
		if o.Year != 0 && (o.Year < 1800 || o.Year > 2100) {
			return nil, fmt.Errorf("overrides: %s: year %d", id, o.Year)
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

// applyOverrides applies ov to songs (in place) and returns the kept songs, the ids applied and the unknown ones.
func applyOverrides(songs []song.Song, ov map[string]Override) (kept []song.Song, applied, unknown []string) {
	seen := map[string]bool{}
	for _, s := range songs {
		o, ok := ov[s.ID]
		if !ok {
			kept = append(kept, s)
			continue
		}
		seen[s.ID] = true
		applied = append(applied, s.ID)
		if o.Exclude {
			continue
		}
		if o.Year != 0 {
			s.Year, s.YearSource = o.Year, song.YearOverride
		}
		if o.AppleID != "" {
			s.AppleID, s.Storefront, s.AppleAlbum = o.AppleID, o.Storefront, ""
			if s.Storefront == "" {
				s.Storefront = storefronts[0]
			}
		}
		kept = append(kept, s)
	}
	for id := range ov {
		if !seen[id] {
			unknown = append(unknown, id)
		}
	}
	slices.Sort(applied)
	slices.Sort(unknown)
	return kept, applied, unknown
}
