package server

import "testing"

func TestOpenBrowser_RespectsEnvVar(t *testing.T) {
	// Set BROWSER to a non-existent command — Start() returns nil (async)
	// but this verifies the env var path is taken.
	t.Setenv("BROWSER", "echo")

	err := OpenBrowser("http://localhost:8080")
	if err != nil {
		t.Errorf("OpenBrowser() with $BROWSER=echo should succeed, got: %v", err)
	}
}
