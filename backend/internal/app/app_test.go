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
	if cfg.Addr != ":8080" || cfg.DataDir != "/data" || cfg.PublicFQDN != "" {
		t.Errorf("defaults = %+v", cfg)
	}
	cfg, err = LoadConfig(lookup(map[string]string{
		"BACKEND_ADDR": "127.0.0.1:9000", "BACKEND_DATA_DIR": "/tmp/x", "PUBLIC_FQDN": "reworm.example.com",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9000" || cfg.DataDir != "/tmp/x" || cfg.PublicFQDN != "reworm.example.com" {
		t.Errorf("from env = %+v", cfg)
	}
}

func TestMain_badLogLevelFailsBoot(t *testing.T) {
	if code := Main(context.Background(), nil, lookup(map[string]string{"BACKEND_LOG_LEVEL": "loud"})); code != 1 {
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
	addr := freeAddr(t)
	if code := Main(context.Background(), []string{"healthcheck"}, lookup(map[string]string{"BACKEND_ADDR": addr})); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestMain_servesUntilCancelled(t *testing.T) {
	addr := freeAddr(t)
	vars := lookup(map[string]string{"BACKEND_ADDR": addr, "BACKEND_DATA_DIR": filepath.Join(t.TempDir(), "data")})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- Main(ctx, nil, vars) }()

	// The container probe answers once the server is up.
	deadline := time.Now().Add(10 * time.Second)
	for Main(context.Background(), []string{"healthcheck"}, vars) != 0 {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("server never became healthy")
		}
		time.Sleep(50 * time.Millisecond)
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
}

// freeAddr is a loopback address nothing listens on.
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
