package app

import (
	"github.com/trick77/ship/kit/env"
)

// Config is the runtime configuration, every variable REWORM_-prefixed.
type Config struct {
	// Addr is the listen address.
	Addr string
	// DataDir holds the database (and later the cover cache): the container's volume.
	DataDir string
	// PublicFQDN is the hostname the app is reached on (join and pairing links); empty for local use.
	PublicFQDN string
}

// defaultAddr is the listen address, also what the container probe dials when REWORM_ADDR is unset.
const defaultAddr = ":8080"

// LoadConfig reads the configuration through lookup, every fault at once.
func LoadConfig(lookup func(string) string) (Config, error) {
	r := env.New(lookup)
	cfg := Config{
		Addr:       r.String("REWORM_ADDR", defaultAddr),
		DataDir:    r.String("REWORM_DATA_DIR", "/data"),
		PublicFQDN: r.String("REWORM_PUBLIC_FQDN", ""),
	}
	return cfg, r.Err()
}
