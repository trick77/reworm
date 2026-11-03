package app

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
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
