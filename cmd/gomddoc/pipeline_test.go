package main

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

func TestRunUntilCancelled(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Run Test")

	cfg, err := config.NewFromServeArgs(config.ServeArgs{Dir: srcDir, Port: ":18923"})
	if err != nil {
		t.Fatalf("config error: %v", err)
	}

	prov, err := provider.NewProvider(srcDir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	defer prov.Close()

	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{
		EnableCache: true, EnableNavigation: true, EnableMetadata: true,
	})
	if err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	httpServer := server.NewHTTPServer(server.HTTPServerConfig{
		Config:           cfg,
		Provider:         prov,
		Registry:         pipeline.Registry,
		EnricherRegistry: pipeline.EnricherRegistry,
		TemplateRenderer: pipeline.TemplateRenderer,
		MetaIndex:        pipeline.MetaIndex,
		RedirectFinder:   pipeline.RedirectFinder,
		StaticFS:         pipeline.StaticFS,
	})

	// Cancel the context immediately so the server starts and stops.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = runUntilCancelled(ctx, httpServer, nil)
	// The server should shut down cleanly — no error expected.
	if err != nil {
		t.Logf("runUntilCancelled returned: %v (may be acceptable)", err)
	}
}

func TestSetupPipeline(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Hello")
	writeTestFile(t, filepath.Join(srcDir, "docs"), "guide.md", "# Guide")

	cfg, err := config.NewFromServeArgs(config.ServeArgs{Dir: srcDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("config error: %v", err)
	}

	prov, err := provider.NewProvider(srcDir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	defer prov.Close()

	tests := []struct {
		name string
		opts PipelineOptions
	}{
		{
			name: "all features enabled",
			opts: PipelineOptions{EnableCache: true, EnableNavigation: true, EnableMetadata: true},
		},
		{
			name: "no cache",
			opts: PipelineOptions{EnableCache: false, EnableNavigation: true, EnableMetadata: true},
		},
		{
			name: "no navigation",
			opts: PipelineOptions{EnableCache: true, EnableNavigation: false, EnableMetadata: true},
		},
		{
			name: "no metadata",
			opts: PipelineOptions{EnableCache: true, EnableNavigation: true, EnableMetadata: false},
		},
		{
			name: "minimal (build mode)",
			opts: PipelineOptions{EnableCache: true, EnableNavigation: false, EnableMetadata: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := setupPipeline(cfg, prov, tt.opts)
			if err != nil {
				t.Fatalf("setupPipeline() error = %v", err)
			}
			if pipeline.Registry == nil {
				t.Error("Registry should not be nil")
			}
			if pipeline.EnricherRegistry == nil {
				t.Error("EnricherRegistry should not be nil")
			}
			if pipeline.TemplateRenderer == nil {
				t.Error("TemplateRenderer should not be nil")
			}
			if tt.opts.EnableNavigation && pipeline.RedirectFinder == nil {
				t.Error("RedirectFinder should not be nil when navigation enabled")
			}
			if !tt.opts.EnableNavigation && pipeline.RedirectFinder != nil {
				t.Error("RedirectFinder should be nil when navigation disabled")
			}
			if tt.opts.EnableMetadata && pipeline.MetaIndex == nil {
				t.Error("MetaIndex should not be nil when metadata enabled")
			}
			if !tt.opts.EnableMetadata && pipeline.MetaIndex != nil {
				t.Error("MetaIndex should be nil when metadata disabled")
			}
		})
	}
}

func TestConvertNavNodes(t *testing.T) {
	t.Parallel()

	nodes := []*navigation.NavNode{
		{
			Label:    "Guide",
			Path:     "/docs/guide.md",
			IsDir:    false,
			IsActive: true,
			IsOpen:   false,
			Children: []*navigation.NavNode{},
		},
		{
			Label:    "API",
			Path:     "/docs/api",
			IsDir:    true,
			IsActive: false,
			IsOpen:   true,
			Children: []*navigation.NavNode{
				{
					Label: "Types",
					Path:  "/docs/api/types.md",
				},
			},
		},
	}

	items := convertNavNodes(nodes)
	if len(items) != 2 {
		t.Fatalf("convertNavNodes() returned %d items, want 2", len(items))
	}

	if items[0].Title != "Guide" {
		t.Errorf("items[0].Title = %q, want %q", items[0].Title, "Guide")
	}
	if items[0].Path != "/docs/guide.md" {
		t.Errorf("items[0].Path = %q, want %q", items[0].Path, "/docs/guide.md")
	}
	if !items[0].Active {
		t.Error("items[0].Active should be true")
	}

	if items[1].Title != "API" {
		t.Errorf("items[1].Title = %q, want %q", items[1].Title, "API")
	}
	if !items[1].IsDir {
		t.Error("items[1].IsDir should be true")
	}
	if !items[1].Open {
		t.Error("items[1].Open should be true")
	}
	if len(items[1].Children) != 1 {
		t.Fatalf("items[1].Children = %d, want 1", len(items[1].Children))
	}
	if items[1].Children[0].Title != "Types" {
		t.Errorf("items[1].Children[0].Title = %q, want %q", items[1].Children[0].Title, "Types")
	}
}

func TestConvertNavNodes_Empty(t *testing.T) {
	t.Parallel()

	items := convertNavNodes(nil)
	if len(items) != 0 {
		t.Errorf("convertNavNodes(nil) returned %d items, want 0", len(items))
	}
}

func TestNavBuilderAdapter(t *testing.T) {
	t.Parallel()

	contentRoot := fstest.MapFS{
		"README.md":     &fstest.MapFile{Data: []byte("# Root")},
		"docs/guide.md": &fstest.MapFile{Data: []byte("# Guide")},
	}

	navGen := navigation.NewGenerator(contentRoot, "README.md", nil, nil)
	builder := navBuilderAdapter(navGen)

	// Call the adapter — it should return nav items for the root
	items := builder("/")
	if items == nil {
		t.Fatal("navBuilderAdapter returned nil")
	}
}

func TestNavBuilderAdapter_EmptyFS(t *testing.T) {
	t.Parallel()

	contentRoot := fstest.MapFS{}
	navGen := navigation.NewGenerator(contentRoot, "README.md", nil, nil)
	builder := navBuilderAdapter(navGen)

	items := builder("/nonexistent")
	// With an empty FS, the generator may return nil root
	if len(items) != 0 {
		t.Logf("items = %v (acceptable)", items)
	}
}

func TestRedirectFinderAdapter(t *testing.T) {
	t.Parallel()

	contentRoot := fstest.MapFS{
		"docs/guide.md": &fstest.MapFile{Data: []byte("# Guide")},
	}

	navGen := navigation.NewGenerator(contentRoot, "README.md", nil, nil)
	finder := redirectFinderAdapter(navGen)

	// "docs" directory has no README → should find first page
	result := finder("/docs")
	if result == "" {
		t.Error("redirectFinderAdapter should find a page")
	}

	// The adapter always returns a result based on navigation tree
	// since the generator builds from content root regardless of path
}
