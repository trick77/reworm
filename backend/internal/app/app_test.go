package app

import (
	"context"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trick77/reworm/internal/preview"
	"github.com/trick77/reworm/internal/server"
	"github.com/trick77/reworm/internal/store"
)

func lookup(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig(lookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{Addr: ":8080", DataDir: "/data"}) {
		t.Errorf("defaults = %+v", cfg)
	}
	cfg, err = LoadConfig(lookup(map[string]string{
		"BACKEND_ADDR": "127.0.0.1:9000", "BACKEND_DATA_DIR": "/tmp/x", "PUBLIC_FQDN": "reworm.example.com",
		"BACKEND_LOG_LEVEL": "debug", "BACKEND_TRUSTED_PROXIES": "1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Addr: "127.0.0.1:9000", DataDir: "/tmp/x", PublicFQDN: "reworm.example.com", LogLevel: "debug", TrustedProxies: 1}
	if cfg != want {
		t.Errorf("from env = %+v, want %+v", cfg, want)
	}
}

func TestLoadConfig_badTrustedProxies(t *testing.T) {
	for _, v := range []string{"-1", "one"} {
		if _, err := LoadConfig(lookup(map[string]string{"BACKEND_TRUSTED_PROXIES": v})); err == nil {
			t.Errorf("BACKEND_TRUSTED_PROXIES=%s: no error", v)
		}
	}
}

func TestLoadConfig_dev(t *testing.T) {
	cfg, err := LoadConfig(lookup(map[string]string{"BACKEND_DEV": "1", "BACKEND_ADDR": "127.0.0.1:9000",
		"BACKEND_DEV_PREVIEWS": "/clips", "BACKEND_DEV_SONGS": "/s.jsonl"}))
	if err != nil || !cfg.Dev || cfg.DevPreviews != "/clips" || cfg.DevSongs != "/s.jsonl" {
		t.Fatalf("dev = %+v, %v", cfg, err)
	}
	for _, addr := range []string{"[::1]:9000", "localhost:9000", "127.0.0.2:0"} {
		if _, err := LoadConfig(lookup(map[string]string{"BACKEND_DEV": "1", "BACKEND_ADDR": addr})); err != nil {
			t.Errorf("loopback %s: %v", addr, err)
		}
	}
	for name, vars := range map[string]map[string]string{
		// never on a public server: a dev clip URL names the song
		"with PUBLIC_FQDN": {"BACKEND_DEV": "1", "BACKEND_ADDR": "127.0.0.1:9000", "PUBLIC_FQDN": "reworm.example"},
		// only on this machine
		"default address": {"BACKEND_DEV": "1"},
		"every interface": {"BACKEND_DEV": "1", "BACKEND_ADDR": "0.0.0.0:9000"},
		"a LAN address":   {"BACKEND_DEV": "1", "BACKEND_ADDR": "192.168.1.5:9000"},
		"a name":          {"BACKEND_DEV": "1", "BACKEND_ADDR": "reworm.lan:9000"},
		"not an address":  {"BACKEND_DEV": "1", "BACKEND_ADDR": "localhost"},
		// the settings alone do nothing silently: dev mode is one explicit switch
		"previews without BACKEND_DEV": {"BACKEND_DEV_PREVIEWS": "/x"},
		"songs without BACKEND_DEV":    {"BACKEND_DEV_SONGS": "/x"},
		"songs with BACKEND_DEV=0":     {"BACKEND_DEV": "0", "BACKEND_DEV_SONGS": "/x"},
	} {
		if _, err := LoadConfig(lookup(vars)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRun_devRefusesARealDataDirUntouched(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year, year_source)
VALUES ('mbid-1', 'A', 'B', 1999, 'card')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	list := func() map[string]string {
		out := map[string]string{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			st, err := e.Info()
			if err != nil {
				t.Fatal(err)
			}
			out[e.Name()] = st.ModTime().String() + string(b)
		}
		return out
	}
	before := list()
	err = Run(ctx, Config{Addr: "127.0.0.1:0", DataDir: dir, Dev: true})
	if err == nil || !strings.Contains(err.Error(), "real songs") {
		t.Fatalf("Run = %v", err)
	}
	// no session key, no -wal or -shm, the database's bytes as they were
	if after := list(); !maps.Equal(before, after) {
		t.Errorf("the refused data dir changed: %v → %v files", len(before), len(after))
	}
}

func TestNewAPI_devRig(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	clips := filepath.Join(dir, "clips")
	songs := filepath.Join(dir, "songs.jsonl")
	if err := os.Mkdir(clips, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clips, "fixture-dq.wav"), []byte("RIFF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(songs, []byte(`{"id":"fixture-dq","artist":"ABBA","title":"Dancing Queen","year":1976,`+
		`"sets":["essentials"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	dev := Config{DataDir: dir, Dev: true, DevPreviews: clips, DevSongs: songs}
	parts, err := newAPI(ctx, dev, db)
	if err != nil {
		t.Fatal(err)
	}
	defer parts.party.Close()
	h := server.New(server.Options{Static: http.NotFoundHandler(), API: parts.routes})
	for path, want := range map[string]string{"/api/dev/previews/fixture-dq.wav": "RIFF", "/api/decks": `"count":1`} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s = %d %s", path, rec.Code, rec.Body.String())
		}
	}

	for _, cfg := range []Config{{DataDir: dir, Dev: true, DevSongs: filepath.Join(dir, "missing")},
		{DataDir: dir, Dev: true, DevPreviews: filepath.Join(dir, "missing")}} {
		if _, err := newAPI(ctx, cfg, db); err == nil {
			t.Errorf("%+v: no error", cfg)
		}
	}

	// a data dir with real songs refuses dev mode before anything is deactivated
	if _, err := db.ExecContext(ctx, `INSERT INTO songs (id, artist, title, year, year_source)
VALUES ('mbid-1', 'A', 'B', 1999, 'card')`); err != nil {
		t.Fatal(err)
	}
	if _, err := newAPI(ctx, dev, db); err == nil || !strings.Contains(err.Error(), "real songs") {
		t.Errorf("real songs: %v", err)
	}
	var active int
	if err := db.QueryRowContext(ctx, `SELECT active FROM songs WHERE id = 'mbid-1'`).Scan(&active); err != nil || active != 1 {
		t.Errorf("the real song was touched: active = %d, %v", active, err)
	}
	// without dev mode the clip route does not exist
	plain, err := newAPI(ctx, Config{DataDir: dir}, db)
	if err != nil {
		t.Fatal(err)
	}
	defer plain.party.Close()
	rec := httptest.NewRecorder()
	server.New(server.Options{Static: http.NotFoundHandler(), API: plain.routes}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dev/previews/fixture-dq.wav", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("dev route without dev mode = %d", rec.Code)
	}
}

func TestMain_badLogLevelFailsBoot(t *testing.T) {
	if code := Main(context.Background(), nil, lookup(map[string]string{"BACKEND_LOG_LEVEL": "loud"})); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestMain_badConfigFailsBoot(t *testing.T) {
	if code := Main(context.Background(), nil, lookup(map[string]string{"BACKEND_TRUSTED_PROXIES": "one"})); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestNewAPI(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "reworm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	cfg := Config{DataDir: dir, PublicFQDN: "reworm.example"}
	prev := previewSource
	defer func() { previewSource = prev }()
	previewSource = func(Config) (preview.Source, server.Routes, error) {
		return preview.NewCache(&preview.Fake{}, preview.TTL, nil), nil, nil
	}
	parts, err := newAPI(ctx, cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	h := server.New(server.Options{Static: http.NotFoundHandler(), API: parts.routes})
	for path, want := range map[string]int{"/api/decks": http.StatusOK, "/api/me": http.StatusUnauthorized,
		"/api/train/stats": http.StatusUnauthorized, "/api/rooms/ZZZZ": http.StatusNotFound, "/api/covers/x": http.StatusNotFound} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s = %d, want %d", path, rec.Code, want)
		}
	}
	parts.party.Close()
	if _, err := os.Stat(filepath.Join(dir, "session.key")); err != nil {
		t.Errorf("session key not kept: %v", err)
	}

	// A broken key file fails the boot rather than unpair every device silently.
	if err := os.WriteFile(filepath.Join(dir, "session.key"), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newAPI(ctx, cfg, db); err == nil {
		t.Error("short session key accepted")
	}
}

func TestMain_runErrorExitsOne(t *testing.T) {
	// A data dir that is a file cannot hold the database.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := Main(context.Background(), nil, lookup(map[string]string{"BACKEND_DATA_DIR": file})); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestMain_healthcheckWithoutServerFails(t *testing.T) {
	// Port 0 is never a listener a client can reach.
	if code := Main(context.Background(), []string{"healthcheck"}, lookup(map[string]string{"BACKEND_ADDR": "127.0.0.1:0"})); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestMain_servesUntilCancelled(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	// kit's httpx.Serve binds the address itself, so a free port found here can be taken before it does. A bind
	// failure ends Main at once with 1: that attempt is retried on another port, never mistaken for a pass.
	for attempt := range 5 {
		vars := lookup(map[string]string{"BACKEND_ADDR": freeAddr(t), "BACKEND_DATA_DIR": dataDir})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan int, 1)
		go func() { done <- Main(ctx, nil, vars) }()
		if !healthy(vars, done) {
			cancel()
			t.Logf("attempt %d: server did not come up, retrying", attempt)
			continue
		}
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Errorf("exit after shutdown = %d, want 0", code)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("server did not stop")
		}
		return
	}
	t.Fatal("server never became healthy")
}

// healthy polls the container probe until it passes (true) or Main exits or 10 s pass (false).
func healthy(vars func(string) string, done <-chan int) bool {
	deadline := time.Now().Add(10 * time.Second)
	for Main(context.Background(), []string{"healthcheck"}, vars) != 0 {
		select {
		case <-done:
			return false
		default:
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
	return true
}

// freeAddr is a loopback address nothing listened on a moment ago.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
