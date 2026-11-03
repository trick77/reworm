package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/trick77/reworm/internal/song"
)

func TestSyncSongs_upsertsAndDeactivatesLeavers(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	first := []song.Song{
		{ID: "a", MBID: "a", Artist: "ABBA", Title: "Waterloo", Year: 1974, YearSource: song.YearCard, AppleID: "1",
			Storefront: "ch", Sets: []string{"essentials"}},
		{ID: "b", Artist: "B", Title: "T", Year: 1980, YearSource: song.YearCard},
	}
	if n, err := SyncSongs(ctx, db, first); err != nil || n != 0 {
		t.Fatalf("first sync: %d, %v", n, err)
	}
	// Second boot: a changed, b left the dataset, c is new.
	second := []song.Song{
		{ID: "a", MBID: "a", Artist: "ABBA", Title: "Waterloo", Year: 1974, YearSource: song.YearOverride, AppleID: "2",
			Storefront: "de", Sets: []string{"essentials", "mega-mix"}},
		{ID: "c", Artist: "C", Title: "T", Year: 1990, YearSource: song.YearCard},
	}
	if n, err := SyncSongs(ctx, db, second); err != nil || n != 1 {
		t.Fatalf("second sync: %d, %v; want 1 deactivated", n, err)
	}
	type row struct {
		yearSource, sets, genres string
		apple, mbid, isrc        *string
		active                   int
	}
	get := func(id string) row {
		var r row
		if err := db.QueryRowContext(ctx, `SELECT year_source, sets, genres, apple_id, mbid, isrc, active FROM songs WHERE id = ?`, id).
			Scan(&r.yearSource, &r.sets, &r.genres, &r.apple, &r.mbid, &r.isrc, &r.active); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return r
	}
	if a := get("a"); a.yearSource != "override" || a.sets != `["essentials","mega-mix"]` || *a.apple != "2" || a.active != 1 || a.isrc != nil {
		t.Errorf("a = %+v", a)
	}
	if b := get("b"); b.active != 0 {
		t.Errorf("b active = %d, want 0 (kept, hidden)", b.active)
	}
	if c := get("c"); c.active != 1 || c.genres != "[]" || c.mbid != nil {
		t.Errorf("c = %+v", c)
	}
	// b comes back: active again.
	if n, err := SyncSongs(ctx, db, first); err != nil || n != 1 || get("b").active != 1 {
		t.Errorf("third sync: %d, %v", n, err)
	}
}

func TestSyncSongs_badRowRollsBack(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := SyncSongs(ctx, db, []song.Song{{ID: "x", Artist: "A", Title: "T", Year: 1, YearSource: "card"}}); err != nil {
		t.Fatal(err)
	}
	// A trigger refuses y: nothing of the batch is applied, x stays active.
	_, err = db.ExecContext(ctx, `CREATE TRIGGER no_y BEFORE INSERT ON songs WHEN NEW.id = 'y' BEGIN SELECT RAISE(ABORT, 'no'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncSongs(ctx, db, []song.Song{{ID: "y", Artist: "A", Title: "T", Year: 2, YearSource: "card"}}); err == nil {
		t.Fatal("sync with a failing row succeeded")
	}
	var active int
	if err := db.QueryRowContext(ctx, `SELECT active FROM songs WHERE id = 'x'`).Scan(&active); err != nil || active != 1 {
		t.Errorf("x active = %d, %v; the failed sync must not deactivate it", active, err)
	}
	_ = db.Close()
	if _, err := SyncSongs(ctx, db, nil); err == nil {
		t.Error("closed db: no error")
	}
}
