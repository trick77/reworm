package train

import "time"

// progress is a song's spaced-repetition row.
//
// Table: train_progress
type progress struct {
	box      int
	due      time.Time
	attempts int
	correct  int
	found    bool
}

// state is the song's progress state as the year proposal adapts to it.
func (p progress) state() string {
	switch {
	case !p.found:
		return "new"
	case p.box >= masteredBox:
		return "mastered"
	}
	return "learning"
}

// statsResponse is a profile's training stats, GET /api/train/stats: totals and sums over its progress.
//
// Table: -
type statsResponse struct {
	Award      string           `json:"award"`
	XP         int              `json:"xp"`
	NextAt     *int             `json:"nextAt"`
	Mastered   int              `json:"mastered"`
	Learning   int              `json:"learning"`
	New        int              `json:"new"`
	StreakDays int              `json:"streakDays"`
	Decades    []decadeAccuracy `json:"decades"`
}
