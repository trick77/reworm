// Command reworm is the server: API + embedded SPA, backed by SQLite.
package main

import (
	"context"
	"os"

	"github.com/trick77/reworm/internal/app"
)

func main() {
	os.Exit(app.Main(context.Background(), os.Args[1:], os.Getenv))
}
