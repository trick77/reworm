// Package train is the solo training mode: server-side sessions (a reload resumes at the current card), adaptive
// year proposals, missed songs asked again a few cards later with a new year, and per-song progress across sessions
// (spaced repetition: new → learning → mastered), XP, a daily streak and record awards (plan D9).
package train

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/trick77/reworm/internal/api"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/engine"
)

// Session lengths.
const (
	DefaultLength = 10
	MaxLength     = 50
)

// reaskAfter is how many cards later a missed song comes back: the mock's "~3 cards later".
const reaskAfter = 4

// Modes.
const (
	ModeYears = "years" // earlier / exact / later against a proposed year
	ModePin   = "pin"   // the player pins the year itself; ±1 counts
)

// Config is POST /api/train/sessions.
type Config struct {
	Deck       deck.Selector       `json:"deck"`
	Mode       string              `json:"mode"`
	Difficulty string              `json:"difficulty"`
	Clip       engine.ClipSettings `json:"clip"`
	Length     int                 `json:"length"`
	// SongIDs drills these songs (a summary's misses) instead of the deck.
	SongIDs []string `json:"songIds,omitempty"`
}

// Normalize fills the defaults and checks the values.
func (c *Config) Normalize(thisYear int) error {
	if c.Mode == "" {
		c.Mode = ModeYears
	}
	if c.Difficulty == "" {
		c.Difficulty = "medium"
	}
	bad := func(msg string) error { return api.Fail(http.StatusBadRequest, "invalid_settings", msg) }
	if len(c.SongIDs) > MaxLength {
		return bad("songIds holds at most 50 songs")
	}
	if c.Length == 0 {
		c.Length = DefaultLength
		if len(c.SongIDs) > 0 {
			c.Length = len(c.SongIDs)
		}
	}
	if c.Mode != ModeYears && c.Mode != ModePin {
		return bad(`mode is "years" or "pin"`)
	}
	if _, ok := engine.Mixes[c.Difficulty]; !ok {
		return bad(`difficulty is "easy", "medium" or "hard"`)
	}
	if c.Length < 1 || c.Length > MaxLength {
		return bad("length is 1 to 50")
	}
	if err := c.Clip.Normalize(); err != nil {
		return bad(err.Error())
	}
	if err := c.Deck.Normalize(thisYear); err != nil {
		return bad(err.Error())
	}
	return nil
}

// card is one entry of a session's queue.
type card struct {
	ID     string `json:"id"`
	SongID string `json:"songId"`
	Year   int    `json:"year"`
	Retry  bool   `json:"retry,omitempty"`
	// Proposed is drawn when the card comes up (its shift depends on the answers before it); 0 until then.
	Proposed int         `json:"proposed,omitempty"`
	Tier     engine.Tier `json:"tier"`
	From     int         `json:"from"`
	// Avoid is the year the song was shown with when it was missed: the re-ask shows another.
	Avoid int `json:"avoid,omitempty"`
}

// answer is one answered card.
type answer struct {
	CardID   string      `json:"cardId"`
	SongID   string      `json:"songId"`
	Year     int         `json:"year"`
	Proposed int         `json:"proposed,omitempty"`
	Choice   string      `json:"choice,omitempty"`
	Guess    int         `json:"guess,omitempty"`
	Correct  bool        `json:"correct"`
	Tier     engine.Tier `json:"tier"`
	XP       int         `json:"xp"`
	Retry    bool        `json:"retry,omitempty"`
}

// session is the stored state of a training session.
type session struct {
	Config   Config   `json:"config"`
	Queue    []card   `json:"queue"`
	Index    int      `json:"index"`
	Streak   int      `json:"streak"`
	Best     int      `json:"best"`
	LastMiss bool     `json:"lastMiss"`
	XP       int      `json:"xp"`
	Answers  []answer `json:"answers"`
}

// current is the card being asked, nil when the session is done.
func (s *session) current() *card {
	if s.Index >= len(s.Queue) {
		return nil
	}
	return &s.Queue[s.Index]
}

// judge scores an answer to c: years mode by choice against the proposed year, pin mode by the guessed year (±1).
func judge(c card, mode, choice string, guess int) (correct bool, tier engine.Tier, xp int, err error) {
	if mode == ModePin {
		if guess < engine.MinYear || guess > 2100 {
			return false, 0, 0, api.Fail(http.StatusBadRequest, "invalid_year", "year is a year")
		}
		d := guess - c.Year
		d = max(d, -d)
		correct, tier = d <= 1, engine.TierOf(d)
		if correct {
			xp = 20
			if d == 0 {
				xp = 30
			}
		}
		return correct, tier, xp, nil
	}
	y, p := c.Year, c.Proposed
	correct, valid := engine.Correct(choice, y, p)
	if !valid {
		return false, 0, 0, api.Fail(http.StatusBadRequest, "invalid_choice", `choice is "earlier", "exact" or "later"`)
	}
	if correct {
		d := max(p-y, y-p)
		switch {
		case d == 0:
			xp = 30
		case d <= 2:
			xp = 20
		default:
			xp = 10
		}
	}
	return correct, c.Tier, xp, nil
}

// record applies an answer to the session's counters and queues the re-ask of a first miss.
func (s *session) record(a answer, newID func() string) {
	s.Answers = append(s.Answers, a)
	if a.Correct {
		s.Streak++
		s.Best = max(s.Best, s.Streak)
	} else {
		s.Streak = 0
	}
	s.LastMiss = !a.Correct
	s.XP += a.XP
	c := s.Queue[s.Index]
	if !a.Correct && !c.Retry {
		at := min(s.Index+reaskAfter, len(s.Queue))
		re := card{ID: newID(), SongID: c.SongID, Year: c.Year, Retry: true, From: c.From, Avoid: c.Proposed}
		s.Queue = slices.Insert(s.Queue, at, re)
	}
	s.Index++
}

// learning is the songs missed this session and not answered right since.
func (s *session) learning() int {
	open := map[string]bool{}
	for _, a := range s.Answers {
		if a.Correct {
			delete(open, a.SongID)
		} else {
			open[a.SongID] = true
		}
	}
	return len(open)
}

func (s *session) correct() int {
	n := 0
	for _, a := range s.Answers {
		if a.Correct {
			n++
		}
	}
	return n
}

// decadeCount is one decade of the session summary.
type decadeCount struct {
	Decade int `json:"decade"`
	OK     int `json:"ok"`
	N      int `json:"n"`
}

func (s *session) byDecade() []decadeCount {
	m := map[int]*decadeCount{}
	for _, a := range s.Answers {
		d := a.Year / 10 * 10
		if m[d] == nil {
			m[d] = &decadeCount{Decade: d}
		}
		m[d].N++
		if a.Correct {
			m[d].OK++
		}
	}
	out := make([]decadeCount, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	slices.SortFunc(out, func(a, b decadeCount) int { return cmp.Compare(a.Decade, b.Decade) })
	return out
}
