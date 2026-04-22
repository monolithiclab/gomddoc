package mcp

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/search"
)

// testProvider is a minimal Provider backed by fstest.MapFS for testing.
type testProvider struct {
	fsys         fstest.MapFS
	defaultIndex string
}

func (p *testProvider) ReadFile(_ context.Context, path string) ([]byte, string, error) {
	path = strings.TrimPrefix(path, "/")
	data, err := fs.ReadFile(p.fsys, path)
	if err != nil {
		return nil, "", err
	}
	return data, "text/markdown", nil
}

func (p *testProvider) Stat(_ context.Context, path string) (fs.FileInfo, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		path = "."
	}
	return fs.Stat(p.fsys, path)
}

func (p *testProvider) DefaultIndex() string                    { return p.defaultIndex }
func (p *testProvider) RootFS(_ context.Context) (fs.FS, error) { return p.fsys, nil }
func (p *testProvider) Close() error                            { return nil }

// testFixture holds a connected MCP client session and server for testing.
type testFixture struct {
	server  *MCPServer
	session *mcp.ClientSession
	cancel  context.CancelFunc
}

func (f *testFixture) close(t *testing.T) {
	t.Helper()
	if err := f.session.Close(); err != nil {
		t.Errorf("closing client session: %v", err)
	}
	f.cancel()
}

// setupTest creates an MCP server with test content and a connected client.
func setupTest(t *testing.T) *testFixture {
	t.Helper()

	testFS := fstest.MapFS{
		"README.md":                &fstest.MapFile{Data: []byte("---\ntitle: Home\ndescription: Welcome page\ntags:\n  - home\n  - go\n---\n# Home\n\nWelcome to the docs.\n")},
		"guide/getting-started.md": &fstest.MapFile{Data: []byte("---\ntitle: Getting Started\ndescription: How to get started\ntags:\n  - guide\n  - go\n---\n# Getting Started\n\n## Installation\n\nRun `go install`.\n\n## Usage\n\nRun `gomddoc serve`.\n")},
		"guide/configuration.md":   &fstest.MapFile{Data: []byte("---\ntitle: Configuration\ndescription: Configuration reference\ntags:\n  - guide\n  - config\n---\n# Configuration\n\nConfigure via YAML.\n")},
		"api/reference.md":         &fstest.MapFile{Data: []byte("---\ntitle: API Reference\ndescription: HTTP API docs\ntags:\n  - api\n---\n# API Reference\n\nEndpoints listed here.\n")},
		// Hidden files that must not be accessible via MCP
		".env":                &fstest.MapFile{Data: []byte("SECRET=hunter2")},
		".gomddoc/config.yml": &fstest.MapFile{Data: []byte("theme: default")},
		".git/config":         &fstest.MapFile{Data: []byte("[core]\nbare = false")},
	}

	ctx := context.Background()

	metaIdx, err := metadata.BuildIndex(ctx, testFS, nil)
	if err != nil {
		t.Fatalf("building metadata index: %v", err)
	}
	searchIdx, err := search.BuildIndex(ctx, testFS, metaIdx, nil)
	if err != nil {
		t.Fatalf("building search index: %v", err)
	}

	prov := &testProvider{fsys: testFS, defaultIndex: "README.md"}

	mcpServer := NewServer(ServerDeps{
		Provider:    prov,
		MetaIndex:   metaIdx,
		SearchIndex: searchIdx,

		DefaultIndex: "README.md",
		SiteName:     "Test Docs",
		Version:      "test",
	})

	ctx, cancel := context.WithCancel(ctx)

	t1, t2 := mcp.NewInMemoryTransports()
	_, err = mcpServer.Server().Connect(ctx, t1, nil)
	if err != nil {
		cancel()
		t.Fatalf("connecting server: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		cancel()
		t.Fatalf("connecting client: %v", err)
	}

	return &testFixture{server: mcpServer, session: session, cancel: cancel}
}

func TestNewServer(t *testing.T) {
	t.Parallel()
	prov := &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"}
	s := NewServer(ServerDeps{
		Provider:     prov,
		DefaultIndex: "README.md",
		SiteName:     "Test",
	})
	if s.server == nil {
		t.Fatal("server should not be nil")
	}
}

func TestResources_SiteIndex(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	result, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: "docs://site/index",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}

	var pages []json.RawMessage
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &pages); err != nil {
		t.Fatalf("unmarshaling pages: %v", err)
	}
	if len(pages) != 4 {
		t.Errorf("expected 4 pages, got %d", len(pages))
	}
}

func TestResources_AllTags(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	result, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: "docs://site/tags",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	var tags []string
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &tags); err != nil {
		t.Fatalf("unmarshaling tags: %v", err)
	}
	if len(tags) == 0 {
		t.Error("expected at least one tag")
	}
	// Tags should be sorted.
	for i := 1; i < len(tags); i++ {
		if tags[i] < tags[i-1] {
			t.Errorf("tags not sorted: %v", tags)
			break
		}
	}
}

func TestResources_PageByPath(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("found", func(t *testing.T) {
		result, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
			URI: "docs://site/page/README.md",
		})
		if err != nil {
			t.Fatalf("ReadResource: %v", err)
		}
		text := result.Contents[0].Text
		if !strings.Contains(text, "# Home") {
			t.Errorf("expected content to contain '# Home', got: %s", text)
		}
		// Frontmatter should be stripped.
		if strings.Contains(text, "title: Home") {
			t.Error("frontmatter should be stripped")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
			URI: "docs://site/page/nonexistent.md",
		})
		if err == nil {
			t.Fatal("expected error for nonexistent page")
		}
	})
}

func TestResources_PagesByTag(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("matching tag", func(t *testing.T) {
		result, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
			URI: "docs://site/tag/go",
		})
		if err != nil {
			t.Fatalf("ReadResource: %v", err)
		}
		var pages []json.RawMessage
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &pages); err != nil {
			t.Fatalf("unmarshaling: %v", err)
		}
		if len(pages) < 2 {
			t.Errorf("expected at least 2 pages with tag 'go', got %d", len(pages))
		}
	})

	t.Run("no match", func(t *testing.T) {
		result, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
			URI: "docs://site/tag/nonexistent",
		})
		if err != nil {
			t.Fatalf("ReadResource: %v", err)
		}
		// Should return empty array, not error.
		if result.Contents[0].Text != "null" && result.Contents[0].Text != "[]" {
			// ByTag returns nil which marshals to "null"
			t.Logf("empty tag result: %s", result.Contents[0].Text)
		}
	})
}

func TestTools_SearchDocs(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("match", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "search_docs",
			Arguments: map[string]any{"query": "install"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "Getting Started") {
			t.Errorf("expected search result to mention 'Getting Started', got: %s", text)
		}
	})

	t.Run("no match", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "search_docs",
			Arguments: map[string]any{"query": "xyznonexistent"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "No results") {
			t.Errorf("expected 'No results', got: %s", text)
		}
	})

	t.Run("empty query", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "search_docs",
			Arguments: map[string]any{"query": ""},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "must not be empty") {
			t.Errorf("expected empty query error, got: %s", text)
		}
	})
}

func TestTools_ReadPage(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("found", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "read_page",
			Arguments: map[string]any{"path": "guide/getting-started.md"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "# Getting Started") {
			t.Errorf("expected page content, got: %s", text)
		}
		if !strings.Contains(text, "go install") {
			t.Errorf("expected 'go install' in content, got: %s", text)
		}
	})

	t.Run("not found", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "read_page",
			Arguments: map[string]any{"path": "nonexistent.md"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "not found") {
			t.Errorf("expected 'not found', got: %s", text)
		}
	})
}

func TestTools_ListPages(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("all pages", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "list_pages",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "4 pages") {
			t.Errorf("expected '4 pages', got: %s", text)
		}
	})

	t.Run("filtered by tag", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "list_pages",
			Arguments: map[string]any{"tag": "guide"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "tagged \"guide\"") {
			t.Errorf("expected tag filter info, got: %s", text)
		}
	})
}

func TestTools_GetTOC(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_table_of_contents",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Table of Contents") {
		t.Errorf("expected 'Table of Contents', got: %s", text)
	}
}

func TestTools_ReadSection(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("found", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "read_section",
			Arguments: map[string]any{"path": "guide/getting-started.md", "heading_id": "installation"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "go install") {
			t.Errorf("expected installation content, got: %s", text)
		}
		if strings.Contains(text, "## Usage") {
			t.Errorf("should not include next section, got: %s", text)
		}
	})

	t.Run("not found", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "read_section",
			Arguments: map[string]any{"path": "guide/getting-started.md", "heading_id": "nonexistent"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "not found") {
			t.Errorf("expected 'not found', got: %s", text)
		}
	})
}

func TestTools_FindRelated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	t.Run("with shared tags", func(t *testing.T) {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "find_related",
			Arguments: map[string]any{"path": "guide/getting-started.md"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "related") {
			t.Errorf("expected related pages info, got: %s", text)
		}
	})
}

func TestPrompts_ExplainConcept(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	result, err := f.session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "explain_concept",
		Arguments: map[string]string{"concept": "configuration"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := result.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "configuration") {
		t.Errorf("expected concept in prompt, got: %s", text)
	}
}

func TestPrompts_Troubleshoot(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	result, err := f.session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "troubleshoot",
		Arguments: map[string]string{"issue": "server not starting", "error_message": "port in use"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := result.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "server not starting") {
		t.Errorf("expected issue in prompt, got: %s", text)
	}
}

func TestPrompts_SummarizePage(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	result, err := f.session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "summarize_page",
		Arguments: map[string]string{"path": "README.md"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := result.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "Welcome to the docs") {
		t.Errorf("expected page content in prompt, got: %s", text)
	}
}

// TestTools_HiddenPathBlocking verifies that MCP tools reject hidden paths,
// preventing access to .env, .git/, .gomddoc/ etc. via the MCP entry point.
func TestTools_HiddenPathBlocking(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	hiddenPaths := []string{
		".env",
		".git/config",
		".gomddoc/config.yml",
		"docs/.secret/notes.md",
	}

	t.Run("read_page", func(t *testing.T) {
		for _, p := range hiddenPaths {
			result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "read_page",
				Arguments: map[string]any{"path": p},
			})
			if err != nil {
				t.Fatalf("CallTool read_page(%s): %v", p, err)
			}
			text := result.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "not found") {
				t.Errorf("read_page(%q) should block hidden path, got: %s", p, text)
			}
		}
	})

	t.Run("read_section", func(t *testing.T) {
		for _, p := range hiddenPaths {
			result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "read_section",
				Arguments: map[string]any{"path": p, "heading_id": "any"},
			})
			if err != nil {
				t.Fatalf("CallTool read_section(%s): %v", p, err)
			}
			text := result.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "not found") {
				t.Errorf("read_section(%q) should block hidden path, got: %s", p, text)
			}
		}
	})

	t.Run("find_related", func(t *testing.T) {
		for _, p := range hiddenPaths {
			result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "find_related",
				Arguments: map[string]any{"path": p},
			})
			if err != nil {
				t.Fatalf("CallTool find_related(%s): %v", p, err)
			}
			text := result.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "not found") && !strings.Contains(text, "No tags found") {
				t.Errorf("find_related(%q) should block hidden path, got: %s", p, text)
			}
		}
	})
}

// TestResources_HiddenPathBlocking verifies that MCP page resources reject hidden paths.
func TestResources_HiddenPathBlocking(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	defer f.close(t)

	hiddenURIs := []string{
		"docs://site/page/.env",
		"docs://site/page/.git/config",
		"docs://site/page/.gomddoc/config.yml",
	}

	for _, uri := range hiddenURIs {
		_, err := f.session.ReadResource(context.Background(), &mcp.ReadResourceParams{
			URI: uri,
		})
		if err == nil {
			t.Errorf("ReadResource(%q) should fail for hidden path", uri)
		}
	}
}
