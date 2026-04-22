package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCLI_NoSubcommand_ShowsUsageAndExitsNonZero(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected non-zero exit code when no subcommand is given")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Error("Expected non-zero exit code")
	}

	out := string(output)
	if !strings.Contains(out, "Usage:") {
		t.Errorf("Expected usage information in output, got:\n%s", out)
	}
	if !strings.Contains(out, "serve") {
		t.Errorf("Expected 'serve' command listed in output, got:\n%s", out)
	}
}

func TestCLI_ServeHelp_ShowsFlagsAndEnvVars(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "serve", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Expected zero exit code for --help, got error: %v", err)
	}

	out := string(output)

	// Verify flags are listed
	for _, want := range []string{"--dir", "--port", "--dev", "--git-key-file"} {
		if !strings.Contains(out, want) {
			t.Errorf("Expected %q in serve --help output, got:\n%s", want, out)
		}
	}

	// Verify environment variables section with all config env vars
	if !strings.Contains(out, "Environment variables:") {
		t.Errorf("Expected 'Environment variables:' section in serve --help output, got:\n%s", out)
	}

	wantVars := []string{
		"GOMDDOC_SERVER_PORT",
		"GOMDDOC_SERVER_DIR",
		"GOMDDOC_SERVER_DEV_MODE",
		"GOMDDOC_SERVER_GIT_SSH_KEY",
		"GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT",
		"GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT",
		"GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT",
		"GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT",
		"GOMDDOC_SERVER_HTTP_MAX_HEADER_MB",
		"GOMDDOC_SITE_DEFAULT_INDEX",
		"GOMDDOC_SITE_DIR_INDEX",
		"GOMDDOC_SITE_EDIT_URL",
		"GOMDDOC_SITE_META_TITLE",
		"GOMDDOC_SITE_META_DESCRIPTION",
		"GOMDDOC_SITE_META_DOMAIN",
		"GOMDDOC_SITE_THEME_NAME",
		"GOMDDOC_SITE_HIGHLIGHTING_THEME",
	}
	for _, v := range wantVars {
		if !strings.Contains(out, v) {
			t.Errorf("Expected %q in serve --help output, got:\n%s", v, out)
		}
	}
}

func TestCLI_Version_ShowsVersion(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Expected zero exit code for --version, got error: %v", err)
	}

	out := strings.TrimSpace(string(output))
	if out == "" {
		t.Error("Expected version output, got empty string")
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()

	binary := t.TempDir() + "/gomddoc-test"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build test binary: %v\n%s", err, output)
	}
	return binary
}
