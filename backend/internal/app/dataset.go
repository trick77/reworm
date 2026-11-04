package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/trick77/reworm/internal/dataset"
	"github.com/trick77/reworm/internal/song"
	"github.com/trick77/reworm/internal/store"
)

// loadDataset upserts the embedded dataset into the songs table at boot. A binary built without it (go run without
// the copy step) leaves the table as it is: syncing an empty dataset would deactivate every song.
func loadDataset(ctx context.Context, db *sql.DB, songs func() ([]song.Song, error)) error {
	list, err := songs()
	if errors.Is(err, dataset.ErrMissing) {
		slog.Warn("no song dataset in this build, songs table left as it is", "hint", err.Error())
		return nil
	}
	if err != nil {
		return fmt.Errorf("song dataset: %w", err)
	}
	if len(list) == 0 {
		// An empty file is a broken build, not a dataset: syncing it would deactivate every song.
		slog.Error("embedded song dataset holds no songs, refused; songs table left as it is")
		return nil
	}
	deactivated, err := store.SyncSongs(ctx, db, list)
	if err != nil {
		return fmt.Errorf("song dataset: %w", err)
	}
	slog.Info("song dataset loaded", "songs", len(list), "deactivated", deactivated)
	return nil
}
