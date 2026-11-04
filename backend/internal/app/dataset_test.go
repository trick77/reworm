package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/trick77/reworm/internal/dataset"
	"github.com/trick77/reworm/internal/song"
	"github.com/trick77/reworm/internal/store"
)

func TestLoadDataset(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	count := func() (n int) {
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM songs WHERE active = 1`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	songs := func() ([]song.Song, error) {
		return []song.Song{{ID: "a", Artist: "A", Title: "T", Year: 1976, YearSource: song.YearCard}}, nil
	}
	if err := loadDataset(ctx, db, songs); err != nil || count() != 1 {
		t.Fatalf("load: %v, %d active", err, count())
	}
	// A build without the dataset leaves the table alone.
	missing := func() ([]song.Song, error) { return nil, dataset.ErrMissing }
	if err := loadDataset(ctx, db, missing); err != nil || count() != 1 {
		t.Errorf("missing dataset: %v, %d active", err, count())
	}
	// An empty dataset is refused: the table keeps its active songs.
	empty := func() ([]song.Song, error) { return nil, nil }
	if err := loadDataset(ctx, db, empty); err != nil || count() != 1 {
		t.Errorf("empty dataset: %v, %d active", err, count())
	}
	broken := func() ([]song.Song, error) { return nil, errors.New("bad line") }
	if err := loadDataset(ctx, db, broken); err == nil {
		t.Error("broken dataset: no error")
	}
	_ = db.Close()
	if err := loadDataset(ctx, db, songs); err == nil {
		t.Error("closed db: no error")
	}
}
