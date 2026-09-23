package main

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/doctor"
	gmcp "github.com/monolithiclab/gomddoc/internal/mcp"
	"github.com/monolithiclab/gomddoc/internal/provider"
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
	app := testModel(t)
	go func() { runErr <- (&MCPCmd{Dir: dir}).Run(app) }()

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

	// gomddoc's own namespace rides along on stdio, describing this site.
	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: capabilities.CapabilitiesURI})
	if err != nil {
		t.Fatalf("reading %s: %v", capabilities.CapabilitiesURI, err)
	}
	var report capabilities.Report
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &report); err != nil {
		t.Fatal(err)
	}
	if report.Config.Dir != dir || report.Config.FileStatus != capabilities.FileFound || report.Theme.Source != "template-scan" || len(report.Guide) < 15 {
		t.Errorf("capabilities over stdio: config=%+v theme=%+v guide pages=%d", report.Config, report.Theme.ThemeInfo, len(report.Guide))
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(tools.Tools, func(tl *mcp.Tool) bool { return tl.Name == "gomddoc_guide" }) {
		t.Error("gomddoc_guide missing over stdio")
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
	err := cmd.Run(testModel(t))
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

// listTools connects an in-memory client to s and returns its tool names.
func listTools(t *testing.T, s *gmcp.MCPServer) []string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := s.Server().Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	return names
}

// TestSiteMCPServer_NoSelfDocs: the MCP endpoint serve mounts at /_mcp/ is the
// site's, for its readers' agents; it must not carry gomddoc's own namespace.
func TestSiteMCPServer_NoSelfDocs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, dir, "README.md", "# Home\n")
	cfg, err := config.NewFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	prov, err := provider.NewProvider(dir, cfg.Site.DefaultIndex, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer prov.Close()
	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{EnableMetadata: true, EnableSearch: true, EnableNavigation: true})
	if err != nil {
		t.Fatal(err)
	}
	tools := listTools(t, siteMCPServer(prov, pipeline, nil))
	if !slices.Contains(tools, "search_docs") {
		t.Fatalf("site MCP server has no search_docs: %q", tools)
	}
	for _, name := range tools {
		if strings.HasPrefix(name, "gomddoc_") {
			t.Errorf("site MCP server exposes %s", name)
		}
	}
}

// TestMCPCmd_DoctorSeesEditsOnDisk: gomddoc_doctor re-reads the site on each
// call, so an agent can edit config.yml and check again without restarting.
func TestMCPCmd_DoctorSeesEditsOnDisk(t *testing.T) {
	// No t.Parallel: replaces os.Stdin and os.Stdout.
	dir := t.TempDir()
	writeTestFile(t, dir, "README.md", "---\ntitle: Home\ndescription: Welcome\n---\n# Home\n")

	serverIn, clientW := testPipe(t)
	clientR, serverOut := testPipe(t)
	swapFile(t, &os.Stdin, serverIn)
	swapFile(t, &os.Stdout, serverOut)
	runErr := make(chan error, 1)
	app := testModel(t)
	go func() { runErr <- (&MCPCmd{Dir: dir}).Run(app) }()

	ctx := context.Background()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).
		Connect(ctx, &mcp.IOTransport{Reader: clientR, Writer: clientW}, nil)
	if err != nil {
		t.Fatal(err)
	}
	check := func() doctor.Report {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "gomddoc_doctor", Arguments: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		var r doctor.Report
		if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	if r := check(); r.Summary.Errors != 0 {
		t.Errorf("clean site reported errors: %+v", r.Findings)
	}
	writeTestFile(t, dir, ".gomddoc/config.yml", "meta:\n  titel: x\n")
	r := check()
	if !slices.ContainsFunc(r.Findings, func(f diag.Finding) bool { return f.Code == "config.unknown-key" && f.Line == 2 }) {
		t.Errorf("edit on disk not seen: %+v", r.Findings)
	}

	if err := session.Close(); err != nil {
		t.Errorf("closing client session: %v", err)
	}
	if err := <-runErr; err != nil {
		t.Errorf("MCPCmd.Run returned %v", err)
	}
}
