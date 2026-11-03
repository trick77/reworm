package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/trick77/reworm/internal/song"
)

// SyncSongs upserts the dataset into songs in one transaction: every dataset song active, every row the dataset no
// longer has kept (progress and history reference it) but inactive. It returns how many rows went inactive.
func SyncSongs(ctx context.Context, db *sql.DB, songs []song.Song) (deactivated int64, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO songs (id, mbid, artist, title, year, year_source, isrc, apple_id, storefront, apple_album, cover_url,
                   cover_source, genres, categories, sets, listeners, active)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
ON CONFLICT (id) DO UPDATE SET
    mbid = excluded.mbid, artist = excluded.artist, title = excluded.title, year = excluded.year,
    year_source = excluded.year_source, isrc = excluded.isrc, apple_id = excluded.apple_id,
    storefront = excluded.storefront, apple_album = excluded.apple_album, cover_url = excluded.cover_url,
    cover_source = excluded.cover_source, genres = excluded.genres, categories = excluded.categories,
    sets = excluded.sets, listeners = excluded.listeners, active = 1`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()
	ids := make([]string, 0, len(songs))
	for _, s := range songs {
		if _, err := stmt.ExecContext(ctx, s.ID, null(s.MBID), s.Artist, s.Title, s.Year, s.YearSource, null(s.ISRC),
			null(s.AppleID), null(s.Storefront), null(s.AppleAlbum), null(s.CoverURL), null(s.CoverSrc),
			jsonList(s.Genres), jsonList(s.Categories), jsonList(s.Sets), s.Listeners); err != nil {
			return 0, fmt.Errorf("upsert song %s: %w", s.ID, err)
		}
		ids = append(ids, s.ID)
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE songs SET active = 0 WHERE active = 1 AND id NOT IN (SELECT value FROM json_each(?))`, string(idsJSON))
	if err != nil {
		return 0, err
	}
	deactivated, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return deactivated, tx.Commit()
}

// jsonList is a string list as the JSON array the column holds; nil is [].
func jsonList(l []string) string {
	if l == nil {
		l = []string{}
	}
	b, _ := json.Marshal(l) // a []string always marshals
	return string(b)
}

// null stores an empty string as NULL: the column has no value, not an empty one.
func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}
