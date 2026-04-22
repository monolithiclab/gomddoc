package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{name: "fixed port", port: ":8080", wantErr: false},
		{name: "host and port", port: "localhost:3000", wantErr: false},
		{name: "auto port", port: ":auto", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := resolvePort(tt.port)
			if tt.wantErr {
				if err == nil {
					t.Error("resolvePort() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("resolvePort() error = %v", err)
			}

			if resolved == "" {
				t.Error("resolvePort() returned empty string")
			}

			// Auto port should resolve to something other than ":auto"
			if tt.port == ":auto" && resolved == ":auto" {
				t.Error("resolvePort(\":auto\") should resolve to actual port")
			}

			// Fixed port should pass through unchanged
			if tt.port != ":auto" && resolved != tt.port {
				t.Errorf("resolvePort(%q) = %q, want %q", tt.port, resolved, tt.port)
			}
		})
	}
}

func TestServeCmd_Setup(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Test Serve")

	cmd := &ServeCmd{
		Dir:     srcDir,
		Port:    ":0",
		DevMode: true,
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
}

func TestServeCmd_Setup_DevMode(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Dev Mode Test")

	cmd := &ServeCmd{
		Dir:     srcDir,
		Port:    ":0",
		DevMode: true,
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil in dev mode")
	}
}

func TestServeCmd_Setup_WithGitStorageDir(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Storage Dir Test")

	cmd := &ServeCmd{
		Dir:           srcDir,
		Port:          ":0",
		GitStorageDir: t.TempDir(),
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
}

func TestServeCmd_Setup_ProductionMode(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Prod Mode Test")

	cmd := &ServeCmd{
		Dir:     srcDir,
		Port:    ":0",
		DevMode: false,
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil in production mode")
	}
}

func TestServeCmd_Setup_NonexistentDir(t *testing.T) {
	t.Parallel()

	cmd := &ServeCmd{
		Dir:  "/nonexistent/path/that/does/not/exist",
		Port: ":0",
	}

	_, err := cmd.setup()
	if err == nil {
		t.Error("setup() with nonexistent dir should return error")
	}
}

func TestWriteEnvVarsHelp(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeEnvVarsHelp(&buf)

	output := buf.String()
	if !strings.Contains(output, "Environment variables:") {
		t.Error("output should contain 'Environment variables:'")
	}
	if !strings.Contains(output, "GOMDDOC_SERVER_PORT") {
		t.Error("output should contain GOMDDOC_SERVER_PORT")
	}
	if !strings.Contains(output, "GOMDDOC_SITE_DEFAULT_INDEX") {
		t.Error("output should contain GOMDDOC_SITE_DEFAULT_INDEX")
	}
	// Check that empty defaults show "(empty)"
	if !strings.Contains(output, "(empty)") {
		t.Error("output should contain '(empty)' for vars with no default")
	}
}

func TestServeCmd_Defaults(t *testing.T) {
	t.Parallel()

	cmd := ServeCmd{}
	if cmd.Dir != "" {
		t.Errorf("Dir default = %q, want empty (Kong sets '.')", cmd.Dir)
	}
	if cmd.Port != "" {
		t.Errorf("Port default = %q, want empty (Kong sets ':8080')", cmd.Port)
	}
	if cmd.DevMode {
		t.Error("DevMode should default to false")
	}
}

func TestServeCmd_Setup_WithSSHKey(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# SSH Key Test")

	// Create a fake SSH key file
	keyFile := filepath.Join(t.TempDir(), "fake_key")
	writeTestFile(t, filepath.Dir(keyFile), filepath.Base(keyFile), "fake-key-data")

	cmd := &ServeCmd{
		Dir:       srcDir,
		Port:      ":0",
		GitSSHKey: keyFile,
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
}

func TestServeCmd_Setup_WithAutoPort(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Auto Port")

	cmd := &ServeCmd{
		Dir:  srcDir,
		Port: ":auto",
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
}

func TestServeCmd_Run_SetupError(t *testing.T) {
	t.Parallel()

	cmd := &ServeCmd{
		Dir:  "/nonexistent/dir",
		Port: ":8080",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("Run() with nonexistent dir should return error")
	}
}

func TestServeCmd_Setup_InvalidPort(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Test")

	cmd := &ServeCmd{
		Dir:  srcDir,
		Port: ":99999",
	}

	_, err := cmd.setup()
	if err == nil {
		t.Error("setup() with invalid port should return error")
	}
}

func TestResolvePort_Error(t *testing.T) {
	t.Parallel()

	// ":auto" with a valid range should succeed — testing pass-through for non-auto
	resolved, err := resolvePort(":1234")
	if err != nil {
		t.Fatalf("resolvePort(\":1234\") error = %v", err)
	}
	if resolved != ":1234" {
		t.Errorf("resolvePort(\":1234\") = %q, want %q", resolved, ":1234")
	}
}
