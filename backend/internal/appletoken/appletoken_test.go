package appletoken

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trick77/toolkit/applemusic"
)

func TestTokenFile(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sub", "apple-token.json")
	if _, ok := Load(path, now); ok {
		t.Error("missing file loaded")
	}
	if err := Save(path, applemusic.Token{Value: "v", Expires: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if got, ok := Load(path, now); !ok || got.Value != "v" {
		t.Errorf("loaded %+v, %v", got, ok)
	}
	if _, ok := Load(path, now.Add(2*time.Hour)); ok {
		t.Error("expired token loaded")
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load(path, now); ok {
		t.Error("broken file loaded")
	}
	if err := Save(filepath.Join(path, "x", "y"), applemusic.Token{Value: "v"}); err == nil {
		t.Error("save under a file: no error")
	}
}
