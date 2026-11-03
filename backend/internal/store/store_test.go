package store

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestOpen_migratesAndReopens(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reworm.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations`).Scan(&name); err != nil || name != "0001_init.sql" {
		t.Fatalf("schema_migrations = %q, %v; want 0001_init.sql", name, err)
	}
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info('songs') ORDER BY cid`)
	if err != nil {
		t.Fatal(err)
	}
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, c)
	}
	_ = rows.Close()
	want := []string{"id", "mbid", "artist", "title", "year", "year_source", "isrc", "apple_id", "storefront",
		"apple_album", "cover_url", "cover_source", "genres", "categories", "sets", "listeners", "active"}
	if !slices.Equal(cols, want) {
		t.Errorf("songs columns = %v, want %v", cols, want)
	}
	_ = db.Close()

	// A second boot finds the schema current and applies nothing again.
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 1 {
		t.Errorf("schema_migrations rows = %d, %v; want 1", n, err)
	}
}

func TestSongs_yearIsRequired(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	// A song without a trustworthy year is dropped by the import, never stored.
	if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year_source) VALUES ('a', 'A', 'T', 'card')`); err == nil {
		t.Error("insert without year succeeded")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year, year_source) VALUES ('a', 'A', 'T', 1976, 'card')`); err != nil {
		t.Errorf("insert: %v", err)
	}
	var sets string
	var active int
	if err := db.QueryRowContext(ctx, `SELECT sets, active FROM songs WHERE id = 'a'`).Scan(&sets, &active); err != nil || sets != "[]" || active != 1 {
		t.Errorf("sets, active = %q, %d, %v; want [], 1", sets, active, err)
	}
}

func TestOpen_unwritablePathFails(t *testing.T) {
	if db, err := Open(context.Background(), filepath.Join(t.TempDir(), "missing", "reworm.db")); err == nil {
		_ = db.Close()
		t.Fatal("Open in a missing directory succeeded")
	}
}
