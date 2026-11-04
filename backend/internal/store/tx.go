package store

import (
	"context"
	"database/sql"
)

// InTx runs f in a transaction on db, committing when f returns nil and rolling back otherwise. The DSN's
// _txlock=immediate takes the write lock at BEGIN, so a check inside f and the write after it cannot interleave with
// another writer.
func InTx(ctx context.Context, db *sql.DB, f func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := f(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
