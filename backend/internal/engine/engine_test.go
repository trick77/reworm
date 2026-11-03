package engine

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) }

const draws = 4000

// tolerance is generous for 4000 draws: one standard deviation of a 50 % share is 0.8 %.
const tolerance = 0.03

// shares draws the tier of draws proposals and returns each tier's share.
func shares(t *testing.T, rng *rand.Rand, difficulty string, shift, year int) [4]float64 {
	t.Helper()
	var n [4]int
	for range draws {
		p, tier := Propose(rng, year, difficulty, shift, 2026)
		d := p - year
		if d < 0 {
			d = -d
		}
		if lo, hi := tierRange[tier][0], tierRange[tier][1]; d < lo || d > hi {
			t.Fatalf("%s proposal %d for %d is %d off", tier, p, year, d)
		}
		n[tier]++
	}
	var s [4]float64
	for i := range n {
		s[i] = float64(n[i]) / draws
	}
	return s
}

func near(a, b [4]float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > tolerance {
			return false
		}
	}
	return true
}

func normalized(m Mix) [4]float64 {
	sum := m[0] + m[1] + m[2] + m[3]
	return [4]float64{m[0] / sum, m[1] / sum, m[2] / sum, m[3] / sum}
}

func TestPropose_baseMixPerDifficulty(t *testing.T) {
	rng := seeded(1)
	for _, d := range Difficulties {
		got := shares(t, rng, d, 0, 1976)
		if want := normalized(Mixes[d]); !near(got, want) {
			t.Errorf("%s: shares %.3f, want %.3f ± %.2f", d, got, want, tolerance)
		}
	}
}

func TestPropose_shiftedMixesMatchTheMock(t *testing.T) {
	// The mock's measured Medium shares (docs/mock Notes): after a miss 47/35/8/11, on a streak tighter.
	rng := seeded(2)
	if got := shares(t, rng, "medium", -1, 1985); !near(got, [4]float64{.47, .35, .08, .11}) {
		t.Errorf("after a miss: %.3f", got)
	}
	if got := shares(t, rng, "medium", 1, 1985); !near(got, normalized(Mix{.20, .30, .2975, .2025})) {
		t.Errorf("on a streak: %.3f", got)
	}
	// Two steps harder on Hard leaves no easy ones.
	if got := shares(t, rng, "hard", 2, 1985); got[TierEasy] != 0 {
		t.Errorf("hard +2 easy share = %.3f, want 0", got[TierEasy])
	}
}

func TestShifted_keepsTheTotal(t *testing.T) {
	for _, d := range Difficulties {
		for s := -2; s <= 2; s++ {
			m := Shifted(Mixes[d], s)
			sum := m[0] + m[1] + m[2] + m[3]
			if math.Abs(sum-1) > 1e-9 {
				t.Errorf("%s shift %d sums to %v", d, s, sum)
			}
			for i, w := range m {
				if w < 0 {
					t.Errorf("%s shift %d tier %d = %v", d, s, i, w)
				}
			}
		}
	}
}

func TestShift(t *testing.T) {
	cases := []struct {
		a    Adapt
		want int
	}{
		{Adapt{Index: 0}, -1},
		{Adapt{Index: 5}, 0},
		{Adapt{Index: 5, Streak: 3}, 1},
		{Adapt{Index: 5, LastMiss: true}, -1},
		{Adapt{Index: 5, SongState: "learning"}, -1},
		{Adapt{Index: 5, SongState: "mastered", Streak: 4}, 2},
		{Adapt{Index: 5, Retry: true, Streak: 5, SongState: "mastered"}, 2}, // clamped
		{Adapt{Index: 1, LastMiss: true, SongState: "learning"}, -2},        // clamped from -3
		{Adapt{Index: 5, Retry: true, LastMiss: true}, 1},
	}
	for _, c := range cases {
		if got := Shift(c.a); got != c.want {
			t.Errorf("Shift(%+v) = %d, want %d", c.a, got, c.want)
		}
	}
}

func TestOffset_directionFiftyFiftyUnlessAnEdge(t *testing.T) {
	rng := seeded(3)
	up := 0
	for range draws {
		if Offset(rng, 1976, TierClear, 2026) > 1976 {
			up++
		}
	}
	if share := float64(up) / draws; math.Abs(share-.5) > tolerance {
		t.Errorf("up share = %.3f, want 0.5", share)
	}
	for range 200 {
		if p := Offset(rng, 1905, TierEasy, 2026); p <= 1905 {
			t.Fatalf("1905 easy proposal %d went below 1900", p)
		}
		if p := Offset(rng, 2023, TierEasy, 2026); p >= 2023 {
			t.Fatalf("2023 easy proposal %d went past now", p)
		}
		if p := Offset(rng, 1976, TierExact, 2026); p != 1976 {
			t.Fatalf("exact proposal %d", p)
		}
	}
}

func TestPickTier_degenerateMix(t *testing.T) {
	if got := PickTier(seeded(4), Mix{0, 0, 0, 1}); got != TierExact {
		t.Errorf("all exact = %s", got)
	}
}

func TestTierOfAndText(t *testing.T) {
	for d, want := range map[int]Tier{0: TierExact, -2: TierTight, 3: TierTight, 4: TierClear, -9: TierClear, 10: TierEasy, 40: TierEasy} {
		if got := TierOf(d); got != want {
			t.Errorf("TierOf(%d) = %s, want %s", d, got, want)
		}
	}
	b, err := json.Marshal(struct{ T Tier }{TierTight})
	if err != nil || string(b) != `{"T":"tight"}` {
		t.Errorf("marshal = %s, %v", b, err)
	}
	var v struct{ T Tier }
	if err := json.Unmarshal([]byte(`{"T":"clear"}`), &v); err != nil || v.T != TierClear {
		t.Errorf("unmarshal = %v, %v", v.T, err)
	}
	if err := json.Unmarshal([]byte(`{"T":"wild"}`), &v); err == nil {
		t.Error("unknown tier accepted")
	}
}

func TestClipSettings(t *testing.T) {
	var c ClipSettings
	if err := c.Normalize(); err != nil || c != (ClipSettings{Length: 20, Start: "chorus"}) {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	for _, bad := range []ClipSettings{{Length: 2}, {Length: 61}, {Length: 20, Start: "intro"}} {
		if err := bad.Normalize(); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

func TestClipFrom(t *testing.T) {
	rng := seeded(5)
	if f := ClipFrom(rng, ClipSettings{Length: 20, Start: "chorus"}); f != 0 {
		t.Errorf("chorus from = %d", f)
	}
	seen := map[int]bool{}
	for range draws {
		f := ClipFrom(rng, ClipSettings{Length: 30, Start: "random"})
		if f < 0 || f > 60 {
			t.Fatalf("random from %d leaves less than 30 s", f)
		}
		seen[f] = true
	}
	if !seen[0] || !seen[60] || len(seen) != 61 {
		t.Errorf("random from covers %d of 61 starts", len(seen))
	}
}

func TestOrder_playedGoLastOldestFirst(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	ids := []string{"a", "b", "c", "d", "e", "f"}
	played := map[string]time.Time{"a": now, "c": now.Add(-time.Hour), "e": now.Add(-48 * time.Hour)}
	got := Order(seeded(6), ids, played)
	if len(got) != len(ids) {
		t.Fatalf("order = %v", got)
	}
	fresh := got[:3]
	slices.Sort(fresh)
	if !slices.Equal(fresh, []string{"b", "d", "f"}) {
		t.Errorf("first three = %v, want the unplayed ones", got[:3])
	}
	if !slices.Equal(got[3:], []string{"e", "c", "a"}) {
		t.Errorf("played tail = %v, want least recent first", got[3:])
	}
	if !slices.Equal(ids, []string{"a", "b", "c", "d", "e", "f"}) {
		t.Error("Order changed its input")
	}
}
