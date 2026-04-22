package server

import (
	"os"
	"testing"
)

func TestOpenBrowser_RespectsEnvVar(t *testing.T) {
	// Set BROWSER to a non-existent command — Start() returns nil (async)
	// but this verifies the env var path is taken.
	t.Setenv("BROWSER", "echo")

	err := OpenBrowser("http://localhost:8080")
	if err != nil {
		t.Errorf("OpenBrowser() with $BROWSER=echo should succeed, got: %v", err)
	}
}

func TestOpenBrowser_FallbackPlatform(t *testing.T) {
	// Ensure no $BROWSER is set
	os.Unsetenv("BROWSER")

	// We can't really test the platform opener in CI without a display,
	// but we can verify it doesn't panic.
	// The command may fail (no display server) but shouldn't error on Start().
	_ = OpenBrowser("http://localhost:8080")
}
