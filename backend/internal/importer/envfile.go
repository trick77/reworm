package importer

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

// readEnvFile reads KEY=VALUE lines: # comments and blank lines skipped, an optional "export " prefix, values
// optionally in single or double quotes. A missing file is an error: -env names one on purpose.
func readEnvFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the operator's own config file
	if err != nil {
		return nil, fmt.Errorf("env file: %w", err)
	}
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			// Never echo the line: it may hold a secret.
			return nil, fmt.Errorf("env file %s line %d: not KEY=VALUE", path, n)
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		out[key] = val
	}
	return out, sc.Err()
}

// lookupWith is the process environment first, then the file's values.
func lookupWith(file map[string]string, getenv func(string) string) func(string) string {
	return func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return file[k]
	}
}
