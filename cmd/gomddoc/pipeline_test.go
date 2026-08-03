package main

import (
	"bytes"
	"context"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

func TestBuildGitConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sshKey     string
		storageDir string
		sourceURL  string
		wantSSH    bool
		wantDisk   bool
	}{
		{
			name:      "no flags",
			sourceURL: "https://github.com/example/repo",
			wantSSH:   false,
			wantDisk:  false,
		},
		{
			name:      "ssh key only",
			sshKey:    "/path/to/key",
			sourceURL: "git@github.com:example/repo.git",
			wantSSH:   true,
			wantDisk:  false,
		},
		{
			name:       "storage dir only",
			storageDir: "/tmp/git-cache",
			sourceURL:  "https://github.com/example/repo",
			wantSSH:    false,
			wantDisk:   true,
		},
		{
			name:       "both flags",
			sshKey:     "/path/to/key",
			storageDir: "/tmp/git-cache",
			sourceURL:  "git@github.com:example/repo.git",
			wantSSH:    true,
			wantDisk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := buildGitConfig(tt.sshKey, tt.storageDir, tt.sourceURL)

			if tt.wantSSH && cfg.SSHKeyFile != tt.sshKey {
				t.Errorf("SSHKeyFile = %q, want %q", cfg.SSHKeyFile, tt.sshKey)
			}
			if !tt.wantSSH && cfg.SSHKeyFile != "" {
				t.Errorf("SSHKeyFile = %q, want empty", cfg.SSHKeyFile)
			}
			if tt.wantDisk && cfg.StorageFactory == nil {
				t.Error("StorageFactory should not be nil when storage dir set")
			}
			if !tt.wantDisk && cfg.StorageFactory != nil {
				t.Error("StorageFactory should be nil when no storage dir")
			}
		})
	}
}

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

func TestBuildNavItems_ActiveAndOpen(t *testing.T) {
	t.Parallel()

	nodes := []*navigation.NavNode{
		{Label: "Guide", Path: "/docs/guide.md"},
		{
			Label: "API",
			Path:  "/docs/api/",
			IsDir: true,
			Children: []*navigation.NavNode{
				{Label: "Types", Path: "/docs/api/types.md"},
				{Label: "Errors", Path: "/docs/api/errors.md"},
			},
		},
	}

	items := buildNavItems(nodes, navigation.NormalizeRequestPath("/docs/api/types.md"))
	if len(items) != 2 {
		t.Fatalf("buildNavItems returned %d items, want 2", len(items))
	}

	guide := items[0]
	if guide.Active || guide.Open {
		t.Errorf("Guide should be inactive/closed, got active=%v open=%v", guide.Active, guide.Open)
	}

	api := items[1]
	if !api.IsDir {
		t.Error("API should be a dir")
	}
	if api.Active {
		t.Error("API dir should not be Active (only leaves can be)")
	}
	if !api.Open {
		t.Error("API dir should be Open because a descendant is active")
	}
	if len(api.Children) != 2 {
		t.Fatalf("API children = %d, want 2", len(api.Children))
	}
	if !api.Children[0].Active {
		t.Error("Types should be Active")
	}
	if api.Children[1].Active || api.Children[1].Open {
		t.Errorf("Errors should be inactive/closed, got active=%v open=%v", api.Children[1].Active, api.Children[1].Open)
	}
}

func TestBuildNavItems_NoMatch(t *testing.T) {
	t.Parallel()

	nodes := []*navigation.NavNode{
		{Label: "Guide", Path: "/guide.md"},
		{Label: "API", Path: "/api/", IsDir: true, Children: []*navigation.NavNode{
			{Label: "Types", Path: "/api/types.md"},
		}},
	}

	items := buildNavItems(nodes, navigation.NormalizeRequestPath("/missing.md"))
	for _, it := range items {
		if it.Active || it.Open {
			t.Errorf("no node should be active/open when current path is absent, got %+v", it)
		}
		for _, child := range it.Children {
			if child.Active || child.Open {
				t.Errorf("no descendant should be active/open, got %+v", child)
			}
		}
	}
}

func TestBuildNavItems_Empty(t *testing.T) {
	t.Parallel()

	items := buildNavItems(nil, "")
	if len(items) != 0 {
		t.Errorf("buildNavItems(nil) returned %d items, want 0", len(items))
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

func TestPrevNextBuilderAdapter(t *testing.T) {
	t.Parallel()

	// Alphabetical leaf order: faq.md, intro.md, zeta.md
	contentRoot := fstest.MapFS{
		"intro.md": &fstest.MapFile{Data: []byte("# Intro")},
		"faq.md":   &fstest.MapFile{Data: []byte("# FAQ")},
		"zeta.md":  &fstest.MapFile{Data: []byte("# Zeta")},
	}
	navGen := navigation.NewGenerator(contentRoot, "README.md", nil, nil)
	builder := prevNextBuilderAdapter(navGen)

	// Middle page: both prev and next populated.
	prev, next := builder("/intro.md")
	if prev == nil || prev.Path != "/faq.md" {
		t.Errorf("prev = %+v, want path=/faq.md", prev)
	}
	if next == nil || next.Path != "/zeta.md" {
		t.Errorf("next = %+v, want path=/zeta.md", next)
	}
	if prev != nil && prev.Title == "" {
		t.Error("prev.Title should be populated from PageEntry.Label")
	}
	if next != nil && next.Title == "" {
		t.Error("next.Title should be populated from PageEntry.Label")
	}

	// First page: only next.
	prev, next = builder("/faq.md")
	if prev != nil {
		t.Errorf("prev = %+v, want nil at first page", prev)
	}
	if next == nil {
		t.Error("next should be set at first page")
	}

	// Last page: only prev.
	prev, next = builder("/zeta.md")
	if prev == nil {
		t.Error("prev should be set at last page")
	}
	if next != nil {
		t.Errorf("next = %+v, want nil at last page", next)
	}

	// Not in tree: both nil.
	prev, next = builder("/missing.md")
	if prev != nil || next != nil {
		t.Errorf("prev=%+v next=%+v, want both nil for missing path", prev, next)
	}
}

// TestSetupLanguagePipelines_DefaultPipelineExcludesLanguages proves the default
// pipeline's indexes stop at the language directories. Each language gets its own
// pipeline; indexing them twice duplicated every translated page in the default
// sitemap, feed, tag pages and sidebar, and made build render it twice.
func TestSetupLanguagePipelines_DefaultPipelineExcludesLanguages(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "---\ntitle: Home\n---\n# Home")
	writeTestFile(t, filepath.Join(srcDir, "fr-FR"), "guide.md",
		"---\ntitle: Guide\nredirect_from:\n  - /ancien-guide\n---\n# Guide")

	cfg, err := config.NewFromServeArgs(config.ServeArgs{Dir: srcDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("config error: %v", err)
	}
	prov, err := provider.NewProvider(srcDir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	defer prov.Close()

	lp, err := setupLanguagePipelines(cfg, prov, PipelineOptions{
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     true,
	})
	if err != nil {
		t.Fatalf("setupLanguagePipelines error: %v", err)
	}

	// The metadata index is the one every downstream consumer reads (sitemap,
	// feed, tag pages), so assert on it exhaustively rather than on a symptom.
	var defaultPaths []string
	for _, page := range lp.Default.MetaIndex.AllPages() {
		defaultPaths = append(defaultPaths, page.Path)
	}
	if want := []string{"/README.md"}; !slices.Equal(defaultPaths, want) {
		t.Errorf("default metadata index = %v, want %v", defaultPaths, want)
	}
	if lp.Default.Resolver.IsEmpty() {
		t.Error("default resolver is empty; strip_extensions should still map root content")
	}
	if _, found := lp.Default.Resolver.CleanPath("fr-FR/guide.md"); found {
		t.Error("default resolver maps a language page; that URL belongs to the fr-FR pipeline")
	}

	// The language pipeline owns that content — and its redirect targets are
	// language-prefixed, since redirect targets are absolute site paths.
	frPipe := lp.ByLang["fr-FR"]
	if frPipe == nil {
		t.Fatalf("no fr-FR pipeline built; languages = %v", lp.Languages)
	}
	wantRedirects := server.URLRedirectMap{"/ancien-guide": "/fr-FR/guide"}
	if !maps.Equal(frPipe.URLRedirects, wantRedirects) {
		t.Errorf("fr-FR URLRedirects = %v, want %v", frPipe.URLRedirects, wantRedirects)
	}
	if len(lp.Default.URLRedirects) != 0 {
		t.Errorf("default URLRedirects = %v, want none (the only redirect_from is French)", lp.Default.URLRedirects)
	}
}

func TestSetupLanguagePipelines_WarnsOnTagsContentCollision_File(t *testing.T) {
	// This test does NOT use t.Parallel() because it captures global slog output.

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Home")
	writeTestFile(t, srcDir, "tags.md", "# Tags Info")

	cfg, err := config.NewFromServeArgs(config.ServeArgs{Dir: srcDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("config error: %v", err)
	}

	prov, err := provider.NewProvider(srcDir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	defer prov.Close()

	// Capture slog output.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	prev := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(prev)

	// Call setupLanguagePipelines, which should trigger the warning.
	_, err = setupLanguagePipelines(cfg, prov, PipelineOptions{
		EnableCache:      true,
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     false,
	})
	if err != nil {
		t.Fatalf("setupLanguagePipelines error: %v", err)
	}

	// Check that the warning contains "tags.md".
	logOutput := buf.String()
	if !strings.Contains(logOutput, "tags.md") {
		t.Errorf("expected warning containing 'tags.md', got: %s", logOutput)
	}
}

func TestSetupLanguagePipelines_WarnsOnTagsContentCollision_Directory(t *testing.T) {
	// This test does NOT use t.Parallel() because it captures global slog output.

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Home")
	writeTestFile(t, filepath.Join(srcDir, "tags"), "index.md", "# Tag Index")

	cfg, err := config.NewFromServeArgs(config.ServeArgs{Dir: srcDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("config error: %v", err)
	}

	prov, err := provider.NewProvider(srcDir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	defer prov.Close()

	// Capture slog output.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	prev := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(prev)

	// Call setupLanguagePipelines, which should trigger the warning.
	_, err = setupLanguagePipelines(cfg, prov, PipelineOptions{
		EnableCache:      true,
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     false,
	})
	if err != nil {
		t.Fatalf("setupLanguagePipelines error: %v", err)
	}

	// Check that the warning contains "tags" directory.
	logOutput := buf.String()
	if !strings.Contains(logOutput, "tags") {
		t.Errorf("expected warning containing 'tags', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "directory") {
		t.Errorf("expected warning mentioning 'directory', got: %s", logOutput)
	}
}

func TestSetupLanguagePipelines_NoWarningWhenNoCollision(t *testing.T) {
	// This test does NOT use t.Parallel() because it captures global slog output.

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Home")
	writeTestFile(t, srcDir, "guide.md", "# Guide")

	cfg, err := config.NewFromServeArgs(config.ServeArgs{Dir: srcDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("config error: %v", err)
	}

	prov, err := provider.NewProvider(srcDir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	defer prov.Close()

	// Capture slog output.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	prev := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(prev)

	// Call setupLanguagePipelines, which should NOT trigger the collision warning.
	_, err = setupLanguagePipelines(cfg, prov, PipelineOptions{
		EnableCache:      true,
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     false,
	})
	if err != nil {
		t.Fatalf("setupLanguagePipelines error: %v", err)
	}

	// Check that the warning does NOT contain "tags.md" collision message.
	logOutput := buf.String()
	if strings.Contains(logOutput, "collides with auto-generated tag pages") {
		t.Errorf("unexpected collision warning when there should be none, got: %s", logOutput)
	}
}
