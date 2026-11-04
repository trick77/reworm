package app

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"

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

// previewSource is where clip URLs come from: the Apple Music client (internal/apple) plugs in here, wrapped in
// preview.NewCache(src, preview.TTL, nil). nil until then: every endpoint that needs a clip answers 503.
var previewSource = func(Config) preview.Source { return nil }

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
	previews := previewSource(cfg)
	devPreviews, err := devRig(ctx, cfg, db)
	if err != nil {
		return apiParts{}, err
	}
	var routes []server.Routes
	if devPreviews != nil {
		previews = devPreviews
		routes = append(routes, devPreviews)
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
	routes = append(routes, accounts, songs, cov, rooms, training)
	return apiParts{routes: routes, party: rooms}, nil
}

// devRig applies the BACKEND_DEV_* settings: the fixture songs loaded, the clip directory as the preview source
// (nil without BACKEND_DEV_PREVIEWS). Loud on purpose: a server running on them must not pass for a real one.
func devRig(ctx context.Context, cfg Config, db *sql.DB) (*dev.Previews, error) {
	if cfg.DevSongs != "" {
		n, err := dev.LoadSongs(ctx, db, cfg.DevSongs)
		if err != nil {
			return nil, err
		}
		slog.Warn("DEV: fixture songs loaded, the dataset is inactive", "file", cfg.DevSongs, "songs", n)
	}
	if cfg.DevPreviews == "" {
		return nil, nil
	}
	p, err := dev.NewPreviews(cfg.DevPreviews)
	if err != nil {
		return nil, err
	}
	slog.Warn("DEV: clips come from a local directory, not Apple Music; their URLs name the song",
		"dir", cfg.DevPreviews, "route", dev.PreviewPath)
	return p, nil
}
