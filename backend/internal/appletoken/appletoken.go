// Package appletoken keeps Apple Music's web-player developer token in a file across runs, so the next import or
// server start skips the bundle download (applemusic.Source.Seed / OnNew).
package appletoken

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/trick77/ship/kit/fsx"
	"github.com/trick77/toolkit/applemusic"
)

// Load reads a token kept by Save; ok is false when there is none or it expired.
func Load(path string, now time.Time) (applemusic.Token, bool) {
	b, err := os.ReadFile(path) //nolint:gosec // reworm's own cache file
	if err != nil {
		return applemusic.Token{}, false
	}
	var t applemusic.Token
	if json.Unmarshal(b, &t) != nil || t.Value == "" || !now.Before(t.Expires) {
		return applemusic.Token{}, false
	}
	return t, true
}

// Save keeps t at path (a cache dir), creating the dir.
func Save(path string, t applemusic.Token) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(path, b, 0o600)
}
