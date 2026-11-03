// Package app wires the server: config, logging, the database, the HTTP handler and the shutdown.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/trick77/ship/kit/env"
	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/version"

	"github.com/trick77/reworm/internal/server"
	"github.com/trick77/reworm/internal/store"
	"github.com/trick77/reworm/web"
)

// shutdownTimeout is how long in-flight requests get to finish after SIGTERM.
const shutdownTimeout = 10 * time.Second

// Main is the process: args without the program name, lookup the environment. It returns the exit code.
func Main(ctx context.Context, args []string, lookup func(string) string) int {
	// The container probe (compose healthcheck) runs before any setup: it must answer even when the rest of the
	// environment is broken.
	if len(args) > 0 && args[0] == "healthcheck" {
		return httpx.Healthcheck(httpx.LocalURL(env.New(lookup).String("BACKEND_ADDR", defaultAddr), "/healthz"))
	}
	// A bad BACKEND_LOG_LEVEL fails boot instead of logging at info.
	if _, err := logging.Setup(logging.Options{Level: lookup("BACKEND_LOG_LEVEL"), LevelVar: "BACKEND_LOG_LEVEL"}); err != nil {
		slog.Error("fatal", logging.Err(err))
		return 1
	}
	if err := Run(ctx, lookup); err != nil {
		slog.Error("fatal", logging.Err(err))
		return 1
	}
	return 0
}

// Run serves until ctx ends or a shutdown signal arrives.
func Run(ctx context.Context, lookup func(string) string) error {
	// First, so the running build is identifiable even when startup fails.
	slog.Info("starting reworm", "version", version.String())

	cfg, err := LoadConfig(lookup)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	db, err := store.Open(ctx, filepath.Join(cfg.DataDir, "reworm.db"))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	handler := server.New(web.SPAHandler(), httpx.Check{Name: "database", Run: db.PingContext})
	srv := httpx.NewServer(cfg.Addr, handler)
	slog.Info("listening", "addr", cfg.Addr, "public_fqdn", cfg.PublicFQDN)
	return httpx.Serve(ctx, srv, shutdownTimeout)
}
