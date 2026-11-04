package train

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	// The zone database in the binary: the distroless image may not carry one.
	_ "time/tzdata"
)

// masteredBox is the box from which a song counts as mastered: three first-try hits in a row, days apart.
const masteredBox = 3

// intervals is how long a song rests after a hit, by its new box (spaced repetition).
var intervals = []time.Duration{0, 24 * time.Hour, 3 * 24 * time.Hour, 7 * 24 * time.Hour, 16 * 24 * time.Hour, 35 * 24 * time.Hour}

func interval(box int) time.Duration { return intervals[min(box, len(intervals)-1)] }

// Award is a record award (D9), reached at XP.
type Award struct {
	Name string
	XP   int
}

// Awards are the record awards, bronze to diamond: about 15 XP a card, so silver after a couple of sessions,
// diamond after a few weeks of daily training.
var Awards = []Award{{"bronze", 0}, {"silver", 250}, {"gold", 750}, {"platinum", 1500}, {"diamond", 3000}}

// AwardFor is the award reached at xp and the XP of the next one (nil at diamond).
func AwardFor(xp int) (string, *int) {
	i := 0
	for i+1 < len(Awards) && xp >= Awards[i+1].XP {
		i++
	}
	if i+1 == len(Awards) {
		return Awards[i].Name, nil
	}
	next := Awards[i+1].XP
	return Awards[i].Name, &next
}

// progress is a song's spaced-repetition row.
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

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func loadProgress(ctx context.Context, q querier, profileID, songID string) (progress, error) {
	var p progress
	var due int64
	err := q.QueryRowContext(ctx, `SELECT box, due_at, attempts, correct FROM train_progress WHERE profile_id = ? AND song_id = ?`,
		profileID, songID).Scan(&p.box, &due, &p.attempts, &p.correct)
	if errors.Is(err, sql.ErrNoRows) {
		return progress{}, nil
	}
	p.found, p.due = err == nil, time.UnixMilli(due)
	return p, err
}

// saveAnswer updates the song's progress and the profile's XP and streak for one answer, in tx. A first-try answer
// moves the box (a hit up, a miss back to 0); a re-ask only counts toward the accuracy.
func saveAnswer(ctx context.Context, tx *sql.Tx, profileID string, a answer, now time.Time, loc *time.Location) error {
	p, err := loadProgress(ctx, tx, profileID, a.SongID)
	if err != nil {
		return err
	}
	ok := 0
	if a.Correct {
		ok = 1
	}
	// A miss is due again at once (the next session asks it first); a hit on a due song moves it up a box and rests
	// it by that box. A hit before the song is due (a drill, a small deck) proves nothing about memory over days: the
	// box and the due date stay, so mastered always means hits spaced out by the intervals.
	due := now
	keep := a.Retry && p.found // the year was just shown: no evidence for the box
	switch {
	case keep:
	case a.Correct && p.found && now.Before(p.due):
		keep = true
	case a.Correct:
		p.box++
		due = now.Add(interval(p.box))
	default:
		p.box = 0
	}
	if keep {
		_, err = tx.ExecContext(ctx, `UPDATE train_progress SET attempts = attempts + 1, correct = correct + ?, last_at = ?
WHERE profile_id = ? AND song_id = ?`, ok, now.UnixMilli(), profileID, a.SongID)
	} else {
		_, err = tx.ExecContext(ctx, `
INSERT INTO train_progress (profile_id, song_id, box, due_at, attempts, correct, last_at) VALUES (?, ?, ?, ?, 1, ?, ?)
ON CONFLICT (profile_id, song_id) DO UPDATE SET box = excluded.box, due_at = excluded.due_at,
    attempts = attempts + 1, correct = correct + excluded.correct, last_at = excluded.last_at`,
			profileID, a.SongID, p.box, due.UnixMilli(), ok, now.UnixMilli())
	}
	if err != nil {
		return err
	}
	today := day(now, loc)
	yesterday := day(now.In(loc).AddDate(0, 0, -1), loc)
	_, err = tx.ExecContext(ctx, `
INSERT INTO train_profiles (profile_id, xp, streak_days, last_day) VALUES (?, ?, 1, ?)
ON CONFLICT (profile_id) DO UPDATE SET xp = xp + excluded.xp,
    streak_days = CASE WHEN last_day = ? THEN streak_days WHEN last_day = ? THEN streak_days + 1 ELSE 1 END,
    last_day = excluded.last_day`, profileID, a.XP, today, today, yesterday)
	return err
}

// day is the calendar day of t in the player's zone, what the daily streak counts.
func day(t time.Time, loc *time.Location) string { return t.In(loc).Format(time.DateOnly) }

// TimezoneHeader carries the client's IANA zone (Intl.DateTimeFormat().resolvedOptions().timeZone).
const TimezoneHeader = "X-Timezone"

// zoneOf is the zone a request names, nil when it names none or an invalid one. "Local" is refused: it would be the
// server's zone, not the player's.
func zoneOf(r *http.Request) *time.Location {
	name := r.Header.Get(TimezoneHeader)
	if name == "" || name == "Local" || len(name) > 64 {
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	return loc
}

// zone is the player's zone: the request's (remembered on the profile when it changed), else the profile's,
// else UTC.
func zone(ctx context.Context, db *sql.DB, r *http.Request, profileID string) (*time.Location, error) {
	if loc := zoneOf(r); loc != nil {
		_, err := db.ExecContext(ctx, `UPDATE profiles SET tz = ? WHERE id = ? AND tz != ?`, loc.String(), profileID, loc.String())
		return loc, err
	}
	var name string
	err := db.QueryRowContext(ctx, `SELECT tz FROM profiles WHERE id = ?`, profileID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return time.UTC, nil
	}
	if err != nil {
		return nil, err
	}
	loc, lerr := time.LoadLocation(name)
	if lerr != nil {
		return time.UTC, nil //nolint:nilerr // a zone the tz database lost: count in UTC
	}
	return loc, nil
}
