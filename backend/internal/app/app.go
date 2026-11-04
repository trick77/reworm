// Package app wires the server: config, logging, the database, the HTTP handler and the shutdown.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/trick77/ship/kit/httpx"
	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/version"

	"github.com/trick77/reworm/internal/dataset"
	"github.com/trick77/reworm/internal/dev"
	"github.com/trick77/reworm/internal/server"
	"github.com/trick77/reworm/internal/store"
	"github.com/trick77/reworm/web"
)

// shutdownTimeout is how long in-flight requests get to finish after SIGTERM.
const shutdownTimeout = 10 * time.Second

// Main is the process: args without the program name, lookup the environment. It returns the exit code.
func Main(ctx context.Context, args []string, lookup func(string) string) int {
	cfg, cfgErr := LoadConfig(lookup)
	// The container probe (compose healthcheck) must answer even when the rest of the environment is broken: Addr
	// is a plain string, never a fault, so a bad other variable does not stop it.
	if len(args) > 0 && args[0] == "healthcheck" {
		return httpx.Healthcheck(httpx.LocalURL(cfg.Addr, "/healthz"))
	}
	// A bad BACKEND_LOG_LEVEL fails boot instead of logging at info.
	if _, err := logging.Setup(logging.Options{Level: cfg.LogLevel, LevelVar: "BACKEND_LOG_LEVEL"}); err != nil {
		slog.Error("fatal", logging.Err(err))
		return 1
	}
	// First, so the running build is identifiable even when startup fails.
	slog.Info("starting reworm", "version", version.String())
	if cfgErr != nil {
		slog.Error("fatal", logging.Err(cfgErr))
		return 1
	}
	if err := Run(ctx, cfg); err != nil {
		slog.Error("fatal", logging.Err(err))
		return 1
	}
	return 0
}

// Run serves until ctx ends or a shutdown signal arrives.
func Run(ctx context.Context, cfg Config) error {
	dbPath := filepath.Join(cfg.DataDir, "reworm.db")
	// Dev mode's guard first, read-only: a refused data dir is left untouched.
	if cfg.Dev {
		if err := dev.CheckDir(ctx, dbPath); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	// The embedded dataset, after the dev guard. Dev mode owns the songs table (fixtures only, the guard refuses
	// any other id): no upsert there, it would bring the whole dataset in and active.
	if !cfg.Dev {
		if err := loadDataset(ctx, db, dataset.Songs); err != nil {
			return err
		}
	}

	api, err := newAPI(ctx, cfg, db)
	if err != nil {
		return err
	}
	handler := server.New(server.Options{
		Static:         web.SPAHandler(),
		TrustedProxies: cfg.TrustedProxies,
		Checks:         []httpx.Check{{Name: "database", Run: db.PingContext}},
		API:            api.routes,
	})
	srv := httpx.NewServer(cfg.Addr, handler)
	// Open room streams never go idle: shutdown ends them instead of waiting out its timeout.
	srv.RegisterOnShutdown(api.party.Close)
	go api.party.Run(ctx)
	slog.Info("listening", "addr", cfg.Addr, "public_fqdn", cfg.PublicFQDN)
	return httpx.Serve(ctx, srv, shutdownTimeout)
}
