package app

import (
	"net"

	"github.com/trick77/ship/kit/env"
)

// Config is the runtime configuration: BACKEND_* variables, plus PUBLIC_FQDN, which the Traefik rule reads too (one
// name, so the app and the router never disagree on the host).
type Config struct {
	// Addr is the listen address.
	Addr string
	// DataDir holds the database (and later the cover cache): the container's volume.
	DataDir string
	// PublicFQDN is the hostname the app is reached on (join and pairing links); empty for local use.
	PublicFQDN string
	// LogLevel is debug, info, warn or error; empty is info.
	LogLevel string
	// TrustedProxies is the number of proxies in front of the app whose X-Forwarded-For entries are believed: 1
	// behind Traefik, 0 (the TCP peer) when reached directly. Too high lets a caller pick its own address.
	TrustedProxies int
	// Dev is dev mode (BACKEND_DEV=1), local tests only: the one switch the BACKEND_DEV_* settings need. Refused
	// with PUBLIC_FQDN, on a BACKEND_ADDR that is not loopback, and at boot on a data dir holding real songs.
	Dev bool
	// DevPreviews is a directory of <song id>.wav clips served instead of Apple previews (dev mode).
	DevPreviews string
	// DevSongs is a JSON Lines fixture made the active song list at boot (dev mode).
	DevSongs string
}

// LoadConfig reads the configuration through lookup, every fault at once.
func LoadConfig(lookup func(string) string) (Config, error) {
	r := env.New(lookup)
	cfg := Config{
		Addr:           r.String("BACKEND_ADDR", ":8080"),
		DataDir:        r.String("BACKEND_DATA_DIR", "/data"),
		PublicFQDN:     r.String("PUBLIC_FQDN", ""),
		LogLevel:       r.String("BACKEND_LOG_LEVEL", ""),
		TrustedProxies: r.Int("BACKEND_TRUSTED_PROXIES", 0),
		Dev:            r.Bool("BACKEND_DEV", false),
		DevPreviews:    r.String("BACKEND_DEV_PREVIEWS", ""),
		DevSongs:       r.String("BACKEND_DEV_SONGS", ""),
	}
	if cfg.TrustedProxies < 0 {
		r.Fail("BACKEND_TRUSTED_PROXIES: %d is negative", cfg.TrustedProxies)
	}
	// A dev clip URL names its song: a public server would hand every phone the answer.
	switch {
	case cfg.Dev && cfg.PublicFQDN != "":
		r.Fail("BACKEND_DEV is for local tests only, not with PUBLIC_FQDN")
	case cfg.Dev && !loopback(cfg.Addr):
		r.Fail("BACKEND_DEV is for local tests only: BACKEND_ADDR %q must be 127.0.0.1, ::1 or localhost", cfg.Addr)
	case !cfg.Dev && (cfg.DevPreviews != "" || cfg.DevSongs != ""):
		r.Fail("BACKEND_DEV_PREVIEWS and BACKEND_DEV_SONGS need BACKEND_DEV=1 (local tests only)")
	}
	return cfg, r.Err()
}

// loopback reports whether addr listens on this machine only (":8080" listens everywhere).
func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}
