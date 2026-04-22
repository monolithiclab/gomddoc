package server

import (
	"io"
	"log/slog"
	"os"
	"testing"
)

// TestMain suppresses log output during tests to reduce noise.
// Logs generated during tests (INFO about access forbidden, file not found, etc.)
// are intentional behavior being tested, not errors to display.
func TestMain(m *testing.M) {
	// Suppress all log output during tests
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}
