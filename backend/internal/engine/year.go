// Package engine is the round logic both modes share: the proposed year, the clip and the order songs come up in.
// Pure functions over an injected *rand.Rand, so tests draw thousands of rounds from a fixed seed.
package engine

import (
	"fmt"
	"math/rand/v2"
)

// Tier is how far the proposed year sits from the real one, easiest first.
type Tier int

// The four tiers: easy ±10–25 (another era), clear ±4–9, tight ±1–3 (the game-night pain), exact.
const (
	TierEasy Tier = iota
	TierClear
	TierTight
	TierExact
)

var tierNames = [...]string{"easy", "clear", "tight", "exact"}

// tierRange is each tier's offset range in years.
var tierRange = [...][2]int{{10, 25}, {4, 9}, {1, 3}, {0, 0}}

func (t Tier) String() string { return tierNames[t] }

// MarshalText makes a Tier its name in JSON.
func (t Tier) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

// UnmarshalText reads a tier name.
func (t *Tier) UnmarshalText(b []byte) error {
	for i, n := range tierNames {
		if n == string(b) {
			*t = Tier(i)
			return nil
		}
	}
	return fmt.Errorf("unknown tier %q", b)
}

// TierOf is the tier an offset of d years falls in (a pinned guess d years off).
func TierOf(d int) Tier {
	d = max(d, -d)
	switch {
	case d == 0:
		return TierExact
	case d <= 3:
		return TierTight
	case d <= 9:
		return TierClear
	}
	return TierEasy
}

// Mix is the chance of each tier, easy to exact; it need not sum to one.
type Mix [4]float64

// Mixes is the base mix per difficulty.
var Mixes = map[string]Mix{
	"easy":   {.55, .25, .08, .12},
	"medium": {.35, .30, .20, .15},
	"hard":   {.12, .28, .38, .22},
}

// Difficulties lists the difficulty names, easiest first.
var Difficulties = []string{"easy", "medium", "hard"}

// MinYear is the earliest year a proposal may show.
const MinYear = 1900

// Adapt is what training shifts a card's mix on; party rounds are fair and use the base mix (shift 0).
type Adapt struct {
	// Index is the card's position in the session: the first two are the warm-up.
	Index int
	// Streak is the right answers in a row before this card.
	Streak int
	// LastMiss is whether the card before was wrong.
	LastMiss bool
	// SongState is the song's progress: "new", "learning" or "mastered".
	SongState string
	// Retry marks the re-ask of a song missed earlier in the session.
	Retry bool
}

// Shift is the per-card shift, -2 (easier) to +2 (harder).
func Shift(a Adapt) int {
	s := 0
	if a.Index < 2 {
		s-- // warm-up leans easy
	}
	if a.Streak >= 3 {
		s++ // on a roll: tighten
	}
	if a.LastMiss {
		s-- // just missed: an easier one
	}
	switch a.SongState {
	case "learning":
		s--
	case "mastered":
		s++
	}
	if a.Retry {
		s += 2 // the year was just shown: test recall
	}
	return min(max(s, -2), 2)
}

// Shifted moves weight between the ends, one step per unit: harder takes from easy and gives to tight and exact,
// easier takes from tight and exact and gives to easy and clear.
func Shifted(m Mix, shift int) Mix {
	for range max(shift, -shift) {
		if shift > 0 {
			a := min(.15, m[0])
			m[0] -= a
			m[2] += a * .65
			m[3] += a * .35
		} else {
			a, b := min(.12, m[2]), min(.04, m[3])
			m[2] -= a
			m[3] -= b
			m[0] += a*.6 + b
			m[1] += a * .4
		}
	}
	return m
}

// PickTier draws a tier from m.
func PickTier(rng *rand.Rand, m Mix) Tier {
	sum := m[0] + m[1] + m[2] + m[3]
	r := rng.Float64() * sum
	for i, w := range m {
		if r -= w; r < 0 {
			return Tier(i)
		}
	}
	return TierClear
}

// Offset places a proposal tier years away from year, within [MinYear, maxYear]: up or down 50/50 when both fit,
// else the side that fits. An exact tier is the year itself.
func Offset(rng *rand.Rand, year int, t Tier, maxYear int) int {
	lo, hi := tierRange[t][0], tierRange[t][1]
	if hi == 0 {
		return year
	}
	d := lo + rng.IntN(hi-lo+1)
	canUp, canDown := year+d <= maxYear, year-d >= MinYear
	up := canUp
	if canUp && canDown {
		up = rng.IntN(2) == 0
	}
	if up {
		return year + d
	}
	return year - d
}

// Correct judges an answer to a proposed year, the one rule party and training share: "earlier" is right when the
// song is older than the year shown, "later" when newer, "exact" when it is that year. valid is false for any other
// choice.
func Correct(choice string, year, proposed int) (correct, valid bool) {
	switch choice {
	case "earlier":
		return year < proposed, true
	case "later":
		return year > proposed, true
	case "exact":
		return year == proposed, true
	}
	return false, false
}

// Propose draws a tier from the difficulty's mix shifted by shift and places the proposal: the year shown for a
// song from year.
func Propose(rng *rand.Rand, year int, difficulty string, shift, maxYear int) (int, Tier) {
	t := PickTier(rng, Shifted(Mixes[difficulty], shift))
	return Offset(rng, year, t, maxYear), t
}
