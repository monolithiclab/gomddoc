package main

import (
	"testing"
)

func TestPreviewCmd_Defaults(t *testing.T) {
	t.Parallel()

	cmd := PreviewCmd{}

	if cmd.Dir != "" {
		t.Errorf("Dir default = %q, want empty (Kong sets '.')", cmd.Dir)
	}
	if cmd.Port != "" {
		t.Errorf("Port default = %q, want empty (Kong sets ':auto')", cmd.Port)
	}
	if cmd.Open {
		t.Error("Open should default to false")
	}
}

func TestPreviewCmd_FieldTags(t *testing.T) {
	t.Parallel()

	cmd := PreviewCmd{
		Dir:  "/tmp/docs",
		Port: ":9090",
		Open: true,
	}

	if cmd.Dir != "/tmp/docs" {
		t.Errorf("Dir = %q, want /tmp/docs", cmd.Dir)
	}
	if cmd.Port != ":9090" {
		t.Errorf("Port = %q, want :9090", cmd.Port)
	}
	if !cmd.Open {
		t.Error("Open should be true")
	}
}

func TestPreviewCmd_Setup(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Preview Test")

	cmd := &PreviewCmd{
		Dir:  srcDir,
		Port: ":0",
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
	if result.cfg == nil {
		t.Error("cfg should not be nil")
	}
}

func TestPreviewCmd_Setup_WithDomain(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Domain Test")

	cmd := &PreviewCmd{
		Dir:    srcDir,
		Port:   ":0",
		Domain: "preview.example.com",
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if got := result.cfg.Site.Meta.Domain; got != "preview.example.com" {
		t.Errorf("Domain = %q, want %q", got, "preview.example.com")
	}
}

func TestPreviewCmd_Setup_NonexistentDir(t *testing.T) {
	t.Parallel()

	cmd := &PreviewCmd{
		Dir:  "/nonexistent/path/that/does/not/exist",
		Port: ":0",
	}

	_, err := cmd.setup()
	if err == nil {
		t.Error("setup() with nonexistent dir should return error")
	}
}

func TestPreviewCmd_Setup_AutoPort(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Auto Port")

	cmd := &PreviewCmd{
		Dir:  srcDir,
		Port: ":auto",
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.cfg.Server.Port == ":auto" {
		t.Error("port should be resolved from :auto")
	}
}

func TestPreviewCmd_Setup_FixedPort(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Fixed Port")

	cmd := &PreviewCmd{
		Dir:  srcDir,
		Port: ":0",
	}

	result, err := cmd.setup()
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	defer result.cleanup()

	if result.cfg == nil {
		t.Error("cfg should not be nil")
	}
	if result.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
}

func TestPreviewCmd_Setup_InvalidPort(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Test")

	cmd := &PreviewCmd{
		Dir:  srcDir,
		Port: ":99999", // invalid port number
	}

	_, err := cmd.setup()
	if err == nil {
		t.Error("setup() with invalid port should return error")
	}
}

func TestPreviewCmd_Run_SetupError(t *testing.T) {
	t.Parallel()

	cmd := &PreviewCmd{
		Dir:  "/nonexistent/dir",
		Port: ":8080",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("Run() with nonexistent dir should return error")
	}
}

func TestPreviewCmd_Open_Default(t *testing.T) {
	t.Parallel()

	cmd := &PreviewCmd{
		Dir:  "/tmp",
		Port: ":0",
		Open: true,
	}
	if !cmd.Open {
		t.Error("Open should be true")
	}
}
