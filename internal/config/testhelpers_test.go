package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSiteConfig writes body to <dir>/.gomddoc/config.yml, creating the
// directory.
func writeSiteConfig(t *testing.T, dir, body string) {
	t.Helper()

	gomddocDir := filepath.Join(dir, ConfigDirName)
	if err := os.MkdirAll(gomddocDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", gomddocDir, err)
	}
	if err := os.WriteFile(filepath.Join(gomddocDir, ConfigFileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}
}
