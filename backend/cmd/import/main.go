// Command import builds the song dataset (data/songs.jsonl) from the card snapshots; see internal/importer.
package main

import (
	"context"
	"os"

	"github.com/trick77/ship/kit/run"

	"github.com/trick77/reworm/internal/importer"
)

func main() {
	ctx, stop := run.Context(context.Background())
	code := importer.Main(ctx, os.Args[1:], os.Getenv, os.Stderr, importer.DefaultEndpoints)
	stop()
	os.Exit(code)
}
