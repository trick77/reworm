// Package store opens the SQLite database (ncruces driver, no cgo) and applies the embedded migrations.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/trick77/ship/kit/migrate"
	"github.com/trick77/ship/kit/sqlitex"

	// registers the "sqlite3" database/sql driver; the SQLite build comes with it.
	_ "github.com/ncruces/go-sqlite3/driver"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrations is the embedded migrations directory, files at its root as kit's runner reads them.
func migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic(err) // embedded at build time; a programmer error
	}
	return sub
}

// Open opens (creating if needed) the database at path, applies pending migrations and returns the pool.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	// Migrations run on a pool of their own, closed before the app's opens: a table rebuild switches foreign keys
	// off on its connection, and one that fails midway must not hand that connection to the app.
	mdb, err := sql.Open("sqlite3", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	_, err = migrate.Up(ctx, mdb, migrations(), migrate.Options{Dialect: migrate.SQLite})
	_ = mdb.Close()
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	db, err := sql.Open("sqlite3", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// dsn is kit's DSN plus _txlock=immediate: a write transaction takes the lock at BEGIN, so two concurrent writers
// queue on the busy timeout instead of one failing at once when it upgrades from a read.
func dsn(path string) string {
	return sqlitex.DSN(path, "_txlock=immediate")
}
