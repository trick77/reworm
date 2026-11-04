// Package deck reads the songs table: the deck lists GET /api/decks shows, the songs a deck selector picks, single
// songs for a round, and the history of songs a host's games played.
package deck

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Song is a song as a round needs it. The answer (artist, title, year) never leaves the server before a reveal.
type Song struct {
	ID         string
	Artist     string
	Title      string
	Year       int
	ISRC       string
	AppleID    string
	Storefront string
	// HasCover is whether the import stored a cover URL; the image itself is served by /api/covers/{id}.
	HasCover bool
}

// Revealed is a song as the reveal shows it.
type Revealed struct {
	ID     string  `json:"id"`
	Artist string  `json:"artist"`
	Title  string  `json:"title"`
	Year   int     `json:"year"`
	Cover  *string `json:"coverUrl"`
}

// Reveal is s as the reveal shows it: the cover from our own cache (D4), null when there is none.
func (s Song) Reveal() Revealed {
	r := Revealed{ID: s.ID, Artist: s.Artist, Title: s.Title, Year: s.Year}
	if s.HasCover {
		u := "/api/covers/" + s.ID
		r.Cover = &u
	}
	return r
}

// General is the genre of every song: no song carries it, picking it alone picks all (the widest span, as the picker
// says); beside other ids it narrows nothing.
const General = "general"

// Selector picks a deck: songs in any of the sets, categories or genres (all songs when the three are empty, General
// aside), with a year in [From, To].
type Selector struct {
	Sets       []string `json:"sets"`
	Categories []string `json:"categories"`
	Genres     []string `json:"genres"`
	From       int      `json:"from"`
	To         int      `json:"to"`
}

// Normalize fills the year range (1900 to thisYear) and checks it.
func (s *Selector) Normalize(thisYear int) error {
	if s.From == 0 {
		s.From = 1900
	}
	if s.To == 0 {
		s.To = thisYear
	}
	if s.Sets == nil {
		s.Sets = []string{}
	}
	if s.Categories == nil {
		s.Categories = []string{}
	}
	if s.Genres == nil {
		s.Genres = []string{}
	}
	if s.From > s.To {
		return fmt.Errorf("year range %d–%d is empty", s.From, s.To)
	}
	if n := len(s.Sets) + len(s.Categories) + len(s.Genres); n > 100 {
		return fmt.Errorf("%d deck ids, at most 100", n)
	}
	return nil
}

// ErrNotFound is a song id the table does not have.
var ErrNotFound = errors.New("song not found")

// Repo reads songs and writes the played history.
type Repo struct {
	db *sql.DB
}

// New returns a Repo on db.
func New(db *sql.DB) *Repo { return &Repo{db: db} }

const songColumns = `id, artist, title, year, coalesce(isrc, ''), coalesce(apple_id, ''), coalesce(storefront, ''),
	coalesce(cover_url, '') != ''`

func scanSong(sc interface{ Scan(...any) error }) (Song, error) {
	var s Song
	err := sc.Scan(&s.ID, &s.Artist, &s.Title, &s.Year, &s.ISRC, &s.AppleID, &s.Storefront, &s.HasCover)
	return s, err
}

// Get is one song by id, active or not (a reveal of a song that left the dataset mid-game still works).
func (r *Repo) Get(ctx context.Context, id string) (Song, error) {
	s, err := scanSong(r.db.QueryRowContext(ctx, `SELECT `+songColumns+` FROM songs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Song{}, ErrNotFound
	}
	return s, err
}

// CoverURL is the cover URL the import stored for a song, "" when none.
func (r *Repo) CoverURL(ctx context.Context, id string) (string, error) {
	var u sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT cover_url FROM songs WHERE id = ?`, id).Scan(&u)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return u.String, err
}

// Select is the active songs sel picks, in id order.
func (r *Repo) Select(ctx context.Context, sel Selector) ([]Song, error) {
	q := `SELECT ` + songColumns + ` FROM songs WHERE active = 1 AND year BETWEEN ? AND ?`
	args := []any{sel.From, sel.To}
	// General narrows nothing: beside other ids it drops out (General + Pop is Pop), alone it leaves all songs.
	genres := slices.DeleteFunc(slices.Clone(sel.Genres), func(g string) bool { return g == General })
	var conds []string
	for _, f := range []struct {
		col string
		ids []string
	}{{"sets", sel.Sets}, {"categories", sel.Categories}, {"genres", genres}} {
		if len(f.ids) == 0 {
			continue
		}
		ids, err := json.Marshal(f.ids)
		if err != nil {
			return nil, err
		}
		conds = append(conds, `EXISTS (SELECT 1 FROM json_each(`+f.col+`) WHERE value IN (SELECT value FROM json_each(?)))`)
		args = append(args, string(ids))
	}
	if len(conds) > 0 {
		q += ` AND (` + strings.Join(conds, " OR ") + `)`
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Song
	for rows.Next() {
		s, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Set is a card set in the deck list.
type Set struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
	From  int    `json:"from"`
	To    int    `json:"to"`
}

// Group is a category or genre in the deck list.
type Group struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Decks is GET /api/decks.
type Decks struct {
	Sets       []Set   `json:"sets"`
	Categories []Group `json:"categories"`
	Genres     []Group `json:"genres"`
}

// Decks counts the active songs per set, category and genre; General, first of the genres, counts them all.
func (r *Repo) Decks(ctx context.Context) (Decks, error) {
	d := Decks{Sets: []Set{}, Categories: []Group{}, Genres: []Group{}}
	rows, err := r.db.QueryContext(ctx, `
SELECT 'all', '`+General+`', count(*), 0, 0 FROM songs WHERE active = 1 HAVING count(*) > 0
UNION ALL
SELECT 'sets', j.value, count(*), min(s.year), max(s.year) FROM songs s, json_each(s.sets) j WHERE s.active = 1 GROUP BY j.value
UNION ALL
SELECT 'categories', j.value, count(*), 0, 0 FROM songs s, json_each(s.categories) j WHERE s.active = 1 GROUP BY j.value
UNION ALL
SELECT 'genres', j.value, count(*), 0, 0 FROM songs s, json_each(s.genres) j WHERE s.active = 1 GROUP BY j.value`)
	if err != nil {
		return d, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var kind, id string
		var n, from, to int
		if err := rows.Scan(&kind, &id, &n, &from, &to); err != nil {
			return d, err
		}
		switch kind {
		case "sets":
			d.Sets = append(d.Sets, Set{ID: id, Name: Name(id), Count: n, From: from, To: to})
		case "categories":
			d.Categories = append(d.Categories, Group{ID: id, Name: Name(id), Count: n})
		case "all":
			d.Genres = append(d.Genres, Group{ID: id, Name: Name(id), Count: n})
		default:
			if id == General {
				continue // a song tagged so by hand: General counts every song already
			}
			d.Genres = append(d.Genres, Group{ID: id, Name: Name(id), Count: n})
		}
	}
	slices.SortFunc(d.Sets, func(a, b Set) int { return order(a.ID, b.ID) })
	slices.SortFunc(d.Categories, func(a, b Group) int { return order(a.ID, b.ID) })
	slices.SortFunc(d.Genres, func(a, b Group) int { return order(a.ID, b.ID) })
	return d, rows.Err()
}

// RecordPlayed notes that a host's game played a song.
func (r *Repo) RecordPlayed(ctx context.Context, profileID, songID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO played_history (profile_id, song_id, played_at) VALUES (?, ?, ?)`,
		profileID, songID, at.UnixMilli())
	return err
}

// Played is when each song was last played in the host's games since since.
func (r *Repo) Played(ctx context.Context, profileID string, since time.Time) (map[string]time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT song_id, max(played_at) FROM played_history
WHERE profile_id = ? AND played_at >= ? GROUP BY song_id`, profileID, since.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at int64
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = time.UnixMilli(at)
	}
	return out, rows.Err()
}
