package main

import (
	"testing"

	"github.com/alecthomas/kong"
)

// TestPreviewCmd_Open parses real command lines: preview opens the browser
// unless told not to.
func TestPreviewCmd_Open(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "default opens", args: []string{"preview", "."}, want: true},
		{name: "--open", args: []string{"preview", "--open", "."}, want: true},
		{name: "--no-open", args: []string{"preview", "--no-open", "."}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parsePreview(t, tt.args).Open; got != tt.want {
				t.Errorf("Open = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPreviewCmd_OpenEnv: GOMDDOC_PREVIEW_OPEN=false turns the default off, and
// a flag still wins. Not parallel: Kong reads the process environment.
func TestPreviewCmd_OpenEnv(t *testing.T) {
	t.Setenv("GOMDDOC_PREVIEW_OPEN", "false")
	if parsePreview(t, []string{"preview", "."}).Open {
		t.Error("Open = true with GOMDDOC_PREVIEW_OPEN=false, want false")
	}
	if !parsePreview(t, []string{"preview", "--open", "."}).Open {
		t.Error("Open = false with --open over GOMDDOC_PREVIEW_OPEN=false, want true")
	}
}

// parsePreview parses args with the options users run and returns the preview command.
func parsePreview(t *testing.T, args []string) PreviewCmd {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, parserOptions()...)
	if err != nil {
		t.Fatalf("kong.New() error = %v", err)
	}
	if _, err := parser.Parse(args); err != nil {
		t.Fatalf("Parse(%v) error = %v", args, err)
	}
	return cli.Preview
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
