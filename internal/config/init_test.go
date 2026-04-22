package config

import (
	"io"
	"log/slog"
	"os"
	"testing"
)

// TestMain suppresses log output during tests to reduce noise.
// Logs generated during tests (WARN about validation, INFO about loading, etc.)
// are intentional behavior being tested, not errors to display.
func TestMain(m *testing.M) {
	// Suppress all log output during tests
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}
