package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPCmd_Run_ServesConfiguredContentOverStdio drives MCPCmd.Run end to end
// over the transport it really uses, with a client on the other side of the
// process's own stdin and stdout.
//
// The point is the wire between config and handler: read_page answering "not
// found" for an excluded path is the only thing that shows cfg.Site.Exclude
// reaching mcp.ServerDeps.ExcludePatterns. internal/mcp's own tests set that
// field directly, so between them and this test there was nothing — dropping
// the field from the options literal here left a server that happily read
// every excluded page and a green suite.
func TestMCPCmd_Run_ServesConfiguredContentOverStdio(t *testing.T) {
	// No t.Parallel: mcp.StdioTransport resolves os.Stdin and os.Stdout when it
	// connects, and this test replaces both.
	dir := t.TempDir()
	writeTestFile(t, dir, "README.md", "# Home\n\nWelcome.\n")
	writeTestFile(t, dir, "private/notes.md", "# Private\n\nInternal only.\n")
	writeTestFile(t, dir, ".gomddoc/config.yml", "exclude:\n  - \"private/\"\n")

	// Two pipes, crossed: the client writes requests into the file the server
	// reads as stdin, and reads replies from the file the server writes as
	// stdout.
	serverIn, clientW := testPipe(t)
	clientR, serverOut := testPipe(t)
	swapFile(t, &os.Stdin, serverIn)
	swapFile(t, &os.Stdout, serverOut)

	runErr := make(chan error, 1)
	go func() { runErr <- (&MCPCmd{Dir: dir}).Run() }()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientR, Writer: clientW}, nil)
	if err != nil {
		t.Fatalf("connecting to the server MCPCmd.Run started: %v", err)
	}

	readPage := func(path string) string {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "read_page",
			Arguments: map[string]any{"path": path},
		})
		if err != nil {
			t.Fatalf("CallTool read_page(%q): %v", path, err)
		}
		return result.Content[0].(*mcp.TextContent).Text
	}

	// The readable page comes first: without it, "not found" for the excluded
	// one would also be what a server serving nothing at all returns.
	if got := readPage("README.md"); !strings.Contains(got, "Welcome.") {
		t.Errorf("read_page(README.md) = %q, want the file's body", got)
	}
	if got := readPage("private/notes.md"); !strings.Contains(got, "not found") {
		t.Errorf("read_page(private/notes.md) = %q, want a refusal: .gomddoc/config.yml excludes private/", got)
	}

	if err := session.Close(); err != nil {
		t.Errorf("closing client session: %v", err)
	}
	// Closing the client closes the server's stdin, which is how `gomddoc mcp`
	// exits when its host disconnects.
	if err := <-runErr; err != nil {
		t.Errorf("MCPCmd.Run returned %v, want nil after the client disconnected", err)
	}
}

func TestMCPCmd_Run_MissingDirectory(t *testing.T) {
	t.Parallel()
	cmd := &MCPCmd{Dir: t.TempDir() + "/absent"}
	err := cmd.Run()
	if err == nil {
		t.Fatal("MCPCmd.Run() = nil, want an error for a directory that does not exist")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("MCPCmd.Run() error = %v, want it wrapped with the step that failed", err)
	}
}

// testPipe returns an os.Pipe whose ends are closed at cleanup.
func testPipe(t *testing.T) (r, w *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	// Both ends may already be closed by the transport that owns them; a second
	// Close is harmless and the error is not interesting.
	t.Cleanup(func() { _, _ = r.Close(), w.Close() })
	return r, w
}

// swapFile points a standard stream at f for the rest of the test.
func swapFile(t *testing.T, std **os.File, f *os.File) {
	t.Helper()
	prev := *std
	*std = f
	t.Cleanup(func() { *std = prev })
}
