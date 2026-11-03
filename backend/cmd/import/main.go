// Command import builds the song dataset (data/songs.jsonl) from the card snapshots; see internal/importer.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/trick77/reworm/internal/importer"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := importer.Main(ctx, os.Args[1:], os.Getenv, os.Stderr, importer.DefaultEndpoints)
	stop()
	os.Exit(code)
}
