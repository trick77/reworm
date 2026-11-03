package app

import (
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
	}
	if cfg.TrustedProxies < 0 {
		r.Fail("BACKEND_TRUSTED_PROXIES: %d is negative", cfg.TrustedProxies)
	}
	return cfg, r.Err()
}
