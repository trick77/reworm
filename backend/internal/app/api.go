package app

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"

	"github.com/trick77/ship/kit/logging"
	"github.com/trick77/ship/kit/ratelimit"

	"github.com/trick77/reworm/internal/account"
	"github.com/trick77/reworm/internal/covers"
	"github.com/trick77/reworm/internal/deck"
	"github.com/trick77/reworm/internal/dev"
	"github.com/trick77/reworm/internal/party"
	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/server"
	"github.com/trick77/reworm/internal/train"
)

// previewSource is where clip URLs come from, the one place that decides: in dev mode the clip directory, which
// serves its files on its own route; else Apple Music (applePreviews, cached a day).
var previewSource = func(cfg Config) (preview.Source, server.Routes, error) {
	if cfg.Dev && cfg.DevPreviews != "" {
		p, err := dev.NewPreviews(cfg.DevPreviews)
		if err != nil {
			return nil, nil, err
		}
		slog.Warn("DEV: clips come from a local directory, not Apple Music; their URLs name the song",
			"dir", cfg.DevPreviews, "route", dev.PreviewPath)
		return p, p, nil
	}
	if cfg.Dev {
		return nil, nil, nil // dev mode never asks Apple: its fixture songs are not in the catalog
	}
	return newApplePreviews(cfg.DataDir), nil, nil
}

// apiParts is the API's route groups and the party service, whose rooms need a ticker and a shutdown hook.
type apiParts struct {
	routes []server.Routes
	party  *party.Service
}

// newAPI builds the API services on db.
func newAPI(ctx context.Context, cfg Config, db *sql.DB) (apiParts, error) {
	// Devices stay paired across restarts: the signing key lives in the data dir.
	key, err := account.LoadKey(filepath.Join(cfg.DataDir, "session.key"))
	if err != nil {
		return apiParts{}, err
	}
	publicURL := ""
	if cfg.PublicFQDN != "" {
		publicURL = "https://" + cfg.PublicFQDN
	}
	// Behind Traefik's TLS (PUBLIC_FQDN set) cookies are Secure; a plain-http local run keeps them usable.
	secure := cfg.PublicFQDN != ""
	clientKey := ratelimit.ClientKey(cfg.TrustedProxies)
	accounts, err := account.New(account.Options{DB: db, Key: key, Secure: secure, PublicURL: publicURL, ClientKey: clientKey})
	if err != nil {
		return apiParts{}, err
	}
	token, err := accounts.SetupToken(ctx)
	if err != nil {
		return apiParts{}, err
	}
	if token != "" {
		slog.Warn("no profile yet: pair the first device with the one-time setup link",
			"url", publicURL+"/setup/"+token, "token", token)
	}
	songs := deck.New(db)
	if err := devSongs(ctx, cfg, db); err != nil {
		return apiParts{}, err
	}
	previews, previewRoutes, err := previewSource(cfg)
	if err != nil {
		return apiParts{}, err
	}
	var routes []server.Routes
	if previewRoutes != nil {
		routes = append(routes, previewRoutes)
	}
	if previews == nil {
		slog.Warn("no preview source configured: party rounds and training answer 503")
	}
	rooms, err := party.New(party.Options{Accounts: accounts, Songs: songs, Previews: previews, Key: key, Secure: secure,
		ClientKey: clientKey})
	if err != nil {
		return apiParts{}, err
	}
	training := train.New(train.Options{DB: db, Accounts: accounts, Songs: songs, Previews: previews})
	cov := covers.New(songs.CoverURL, filepath.Join(cfg.DataDir, "covers"), nil)
	// The dataset is loaded by now: a cover none of its songs names any more is dropped (a failed sweep only costs
	// disk, the server starts anyway).
	if stored, err := songs.CoverURLs(ctx); err != nil {
		slog.Warn("cover cache not swept", logging.Err(err))
	} else if n, err := cov.Sweep(stored); err != nil {
		slog.Warn("cover cache not swept", logging.Err(err))
	} else if n > 0 {
		slog.Info("cover cache swept", "removed", n)
	}
	routes = append(routes, accounts, songs, cov, rooms, training)
	return apiParts{routes: routes, party: rooms}, nil
}

// devSongs is dev mode's guard and its song list: a data dir that holds real songs refuses to start (dev mode
// would deactivate them), then BACKEND_DEV_SONGS is loaded. Loud on purpose: such a server must not pass for a
// real one.
func devSongs(ctx context.Context, cfg Config, db *sql.DB) error {
	if !cfg.Dev {
		return nil
	}
	if err := dev.CheckData(ctx, db); err != nil {
		return err
	}
	if cfg.DevSongs == "" {
		return nil
	}
	n, err := dev.LoadSongs(ctx, db, cfg.DevSongs)
	if err != nil {
		return err
	}
	slog.Warn("DEV: fixture songs are the song list", "file", cfg.DevSongs, "songs", n)
	return nil
}
