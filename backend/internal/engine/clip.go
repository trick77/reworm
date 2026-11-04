package engine

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

// PreviewLength is the length of an Apple Music preview in seconds; it starts near the chorus.
const PreviewLength = 90

// Clip limits in seconds.
const (
	MinClip     = 5
	MaxClip     = 60
	DefaultClip = 20
)

// ClipSettings is how a game plays its clips.
type ClipSettings struct {
	// Length in seconds.
	Length int `json:"length"`
	// Start is "chorus" (the preview's own start, which sits near the chorus) or "random" (anywhere in it).
	Start string `json:"start"`
}

// Normalize fills the defaults and checks the values.
func (c *ClipSettings) Normalize() error {
	if c.Length == 0 {
		c.Length = DefaultClip
	}
	if c.Start == "" {
		c.Start = "chorus"
	}
	if c.Length < MinClip || c.Length > MaxClip {
		return fmt.Errorf("clip length %d outside %d–%d s", c.Length, MinClip, MaxClip)
	}
	if c.Start != "chorus" && c.Start != "random" {
		return fmt.Errorf("clip start %q is neither chorus nor random", c.Start)
	}
	return nil
}

// Clip is what the stage plays: from seconds into the preview, for length seconds.
type Clip struct {
	PreviewURL string `json:"previewUrl"`
	From       int    `json:"from"`
	Length     int    `json:"length"`
}

// ClipFrom is where a clip starts in the preview: 0 for chorus, else anywhere that leaves the whole length.
func ClipFrom(rng *rand.Rand, c ClipSettings) int {
	if c.Start != "random" || c.Length >= PreviewLength {
		return 0
	}
	return rng.IntN(PreviewLength - c.Length + 1)
}

// Order shuffles ids into play order: songs not in played first, then the played ones, least recently played
// first, so a host's crowd does not hear last week's songs again before the rest.
func Order(rng *rand.Rand, ids []string, played map[string]time.Time) []string {
	out := slices.Clone(ids)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	slices.SortStableFunc(out, func(a, b string) int {
		ta, oka := played[a]
		tb, okb := played[b]
		switch {
		case !oka && !okb:
			return 0
		case !oka:
			return -1
		case !okb:
			return 1
		}
		return ta.Compare(tb)
	})
	return out
}
