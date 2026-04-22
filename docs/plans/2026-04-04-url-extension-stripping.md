# URL Extension Stripping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve content files at extensionless canonical URLs with 301 redirects from extension-based URLs, and generate pretty URL output in build mode.

**Architecture:** A `PathResolver` built at startup maps extensionless paths to real file paths. A redirect middleware enforces canonical URLs. The handler consults the resolver for path resolution. Build mode outputs `guide/index.html` instead of `guide.html`.

**Tech Stack:** Go 1.25+, standard library, existing gomddoc packages.

**Spec:** `docs/specs/2026-04-04-url-extension-stripping.md`

---

## File Structure

| Action | File                                              | Responsibility                                                 |
| ------ | ------------------------------------------------- | -------------------------------------------------------------- |
| Create | `internal/resolve/resolver.go`                    | PathResolver: builds extensionless↔real path mapping           |
| Create | `internal/resolve/resolver_test.go`               | Unit tests for resolver                                        |
| Modify | `internal/config/config.go`                       | Add `StripExtensions` field to `SiteConfig`, default `[".md"]` |
| Modify | `internal/config/config_test.go`                  | Test new config field                                          |
| Create | `internal/server/redirect.go`                     | Extension redirect middleware                                  |
| Create | `internal/server/redirect_test.go`                | Tests for redirect middleware                                  |
| Modify | `internal/server/handler.go`                      | Consult resolver for extensionless paths                       |
| Modify | `internal/server/handler_test.go`                 | Test resolver integration                                      |
| Modify | `internal/server/server.go`                       | Wire resolver into middleware and handler                      |
| Modify | `internal/template/navigation/navigation.go`      | Emit clean paths via resolver                                  |
| Modify | `internal/template/navigation/navigation_test.go` | Test clean path emission                                       |
| Modify | `internal/seo/url.go`                             | Strip extensions from page URLs                                |
| Modify | `internal/seo/url_test.go`                        | Test extension stripping in SEO URLs                           |
| Modify | `cmd/gomddoc/build.go`                            | Pretty URL output (`guide/index.html`)                         |
| Modify | `cmd/gomddoc/build_test.go`                       | Test pretty URL output                                         |
| Modify | `cmd/gomddoc/pipeline.go`                         | Wire resolver into pipeline                                    |

---

### Task 1: Add `StripExtensions` config field

**Files:**

- Modify: `internal/config/config.go:72-83` (SiteConfig struct)
- Modify: `internal/config/config.go:185-201` (NewSiteConfig defaults)
- Modify: `internal/config/config.go:259-296` (Validate)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the test for default value**

Add a test to `internal/config/config_test.go`:

```go
func TestNewSiteConfig_StripExtensionsDefault(t *testing.T) {
	t.Parallel()
	sc := NewSiteConfig(".")
	want := []string{".md"}
	if !slices.Equal(sc.StripExtensions, want) {
		t.Errorf("StripExtensions = %v, want %v", sc.StripExtensions, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestNewSiteConfig_StripExtensionsDefault -v`
Expected: FAIL — `StripExtensions` field does not exist.

- [ ] **Step 3: Add the field and default**

In `internal/config/config.go`, add `StripExtensions` to `SiteConfig` (after `Exclude` on line 82):

```go
StripExtensions []string `yaml:"strip_extensions"`
```

In `NewSiteConfig()` (around line 185), add to the returned struct:

```go
StripExtensions: []string{".md"},
```

- [ ] **Step 4: Write test for YAML loading**

```go
func TestSiteConfig_LoadFromFile_StripExtensions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".gomddoc")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("strip_extensions:\n  - .md\n  - .html\n"), 0o644)

	sc := NewSiteConfig(dir)
	if err := sc.LoadFromFile(dir); err != nil {
		t.Fatal(err)
	}
	want := []string{".md", ".html"}
	if !slices.Equal(sc.StripExtensions, want) {
		t.Errorf("StripExtensions = %v, want %v", sc.StripExtensions, want)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/ -run TestNewSiteConfig_StripExtensionsDefault\|TestSiteConfig_LoadFromFile_StripExtensions -v`
Expected: PASS

- [ ] **Step 6: Write validation test for bad extensions**

```go
func TestSiteConfig_Validate_StripExtensions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		exts    []string
		wantErr bool
	}{
		{"valid", []string{".md", ".html"}, false},
		{"empty list", []string{}, false},
		{"nil", nil, false},
		{"missing dot", []string{"md"}, true},
		{"empty string", []string{""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sc := NewSiteConfig(".")
			sc.StripExtensions = tt.exts
			err := sc.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 7: Add validation logic**

In `SiteConfig.Validate()` (around line 291, before the return), add:

```go
for _, ext := range sc.StripExtensions {
	if ext == "" || ext[0] != '.' {
		return fmt.Errorf("strip_extensions: %q must start with a dot", ext)
	}
}
```

- [ ] **Step 8: Run all config tests**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "Add StripExtensions config field with .md default"
```

---

### Task 2: Create PathResolver

**Files:**

- Create: `internal/resolve/resolver.go`
- Create: `internal/resolve/resolver_test.go`

- [ ] **Step 1: Write basic resolution test**

Create `internal/resolve/resolver_test.go`:

```go
package resolve

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestResolver_BasicResolution(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"docs/guide.md":    &fstest.MapFile{Data: []byte("# Guide")},
		"README.md":        &fstest.MapFile{Data: []byte("# Root")},
		"about.md":         &fstest.MapFile{Data: []byte("# About")},
		"image.jpg":        &fstest.MapFile{Data: []byte("binary")},
	}

	r := Build(fsys, []string{".md"}, hasHTMLRenderer)

	tests := []struct {
		name      string
		input     string
		wantReal  string
		wantFound bool
	}{
		{"strip md", "docs/guide", "docs/guide.md", true},
		{"strip root file", "about", "about.md", true},
		{"no match", "nonexistent", "", false},
		{"image not stripped", "image", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			real, found := r.Resolve(tt.input)
			if found != tt.wantFound || real != tt.wantReal {
				t.Errorf("Resolve(%q) = (%q, %v), want (%q, %v)",
					tt.input, real, found, tt.wantReal, tt.wantFound)
			}
		})
	}
}

// hasHTMLRenderer simulates renderer check: .md and .html are renderable
func hasHTMLRenderer(mimeType string) bool {
	return mimeType == "text/markdown" || mimeType == "text/html"
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resolve/ -run TestResolver_BasicResolution -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement PathResolver**

Create `internal/resolve/resolver.go`:

```go
package resolve

import (
	"io/fs"
	"log/slog"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// PathResolver maps extensionless paths to real file paths and vice versa.
type PathResolver struct {
	toReal  map[string]string // extensionless -> real
	toClean map[string]string // real -> extensionless
}

// RendererCheck reports whether a MIME type has an HTML renderer.
type RendererCheck func(mimeType string) bool

// Build walks the filesystem and builds the resolution maps.
// Extensions in stripExts are tried in order; first match wins collisions.
// Only files whose MIME type passes hasRenderer are eligible.
func Build(fsys fs.FS, stripExts []string, hasRenderer RendererCheck) *PathResolver {
	r := &PathResolver{
		toReal:  make(map[string]string),
		toClean: make(map[string]string),
	}

	if len(stripExts) == 0 {
		return r
	}

	// Build set for fast lookup
	extSet := make(map[string]int, len(stripExts)) // ext -> priority (lower = higher)
	for i, ext := range stripExts {
		extSet[ext] = i
	}

	// Collect all directories for collision detection
	dirs := make(map[string]bool)
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return nil
		}
		if d.IsDir() {
			dirs[p] = true
		}
		return nil
	})

	// Walk files and build mappings
	// Process in priority order: collect candidates, then resolve collisions
	type candidate struct {
		cleanPath string
		realPath  string
		priority  int
	}
	var candidates []candidate

	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "." {
			return nil
		}

		ext := path.Ext(p)
		priority, ok := extSet[ext]
		if !ok {
			return nil
		}

		// Check if this file type has an HTML renderer
		mimeType := negotiate.NormalizeMimeType(negotiate.DetectMIME(p))
		if !hasRenderer(mimeType) {
			return nil
		}

		clean := strings.TrimSuffix(p, ext)
		candidates = append(candidates, candidate{
			cleanPath: clean,
			realPath:  p,
			priority:  priority,
		})
		return nil
	})

	// Resolve: first by priority (lower index = higher priority)
	for _, c := range candidates {
		if existing, taken := r.toReal[c.cleanPath]; taken {
			// Check if current candidate has higher priority
			existingExt := path.Ext(existing)
			existingPriority := extSet[existingExt]
			if c.priority >= existingPriority {
				slog.Warn("Extension stripping collision: path already claimed",
					slog.String("clean_path", c.cleanPath),
					slog.String("claimed_by", existing),
					slog.String("skipped", c.realPath),
				)
				continue
			}
			// Current candidate has higher priority — replace
			delete(r.toClean, existing)
			slog.Warn("Extension stripping collision: higher priority extension wins",
				slog.String("clean_path", c.cleanPath),
				slog.String("winner", c.realPath),
				slog.String("loser", existing),
			)
		}

		// Check directory collision — file wins, but warn
		if dirs[c.cleanPath] {
			slog.Warn("Extension stripping: file takes precedence over directory",
				slog.String("file", c.realPath),
				slog.String("directory", c.cleanPath+"/"),
			)
		}

		r.toReal[c.cleanPath] = c.realPath
		r.toClean[c.realPath] = c.cleanPath
	}

	return r
}

// Resolve maps an extensionless path to the real file path.
func (r *PathResolver) Resolve(cleanPath string) (realPath string, found bool) {
	real, ok := r.toReal[cleanPath]
	return real, ok
}

// CleanPath maps a real file path to its extensionless canonical path.
func (r *PathResolver) CleanPath(realPath string) (cleanPath string, found bool) {
	clean, ok := r.toClean[realPath]
	return clean, ok
}

// IsEmpty reports whether the resolver has no mappings (feature disabled).
func (r *PathResolver) IsEmpty() bool {
	return len(r.toReal) == 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/resolve/ -run TestResolver_BasicResolution -v`
Expected: PASS

- [ ] **Step 5: Write collision tests**

Add to `internal/resolve/resolver_test.go`:

```go
func TestResolver_MultiExtensionCollision(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md":   &fstest.MapFile{Data: []byte("md")},
		"guide.html": &fstest.MapFile{Data: []byte("html")},
	}

	r := Build(fsys, []string{".md", ".html"}, hasHTMLRenderer)

	// .md has higher priority (first in list)
	real, found := r.Resolve("guide")
	if !found || real != "guide.md" {
		t.Errorf("Resolve(guide) = (%q, %v), want (guide.md, true)", real, found)
	}

	// guide.html should NOT have a clean path mapping
	_, found = r.CleanPath("guide.html")
	if found {
		t.Error("guide.html should not have a clean path (lost collision)")
	}
}

func TestResolver_DirectoryCollision(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md":          &fstest.MapFile{Data: []byte("file")},
		"guide/intro.md":    &fstest.MapFile{Data: []byte("intro")},
	}

	r := Build(fsys, []string{".md"}, hasHTMLRenderer)

	// File wins — /guide resolves to guide.md
	real, found := r.Resolve("guide")
	if !found || real != "guide.md" {
		t.Errorf("Resolve(guide) = (%q, %v), want (guide.md, true)", real, found)
	}

	// Directory content still has its own mappings
	real, found = r.Resolve("guide/intro")
	if !found || real != "guide/intro.md" {
		t.Errorf("Resolve(guide/intro) = (%q, %v), want (guide/intro.md, true)", real, found)
	}
}

func TestResolver_MultipleDots(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"my.config.md": &fstest.MapFile{Data: []byte("config")},
	}

	r := Build(fsys, []string{".md"}, hasHTMLRenderer)

	real, found := r.Resolve("my.config")
	if !found || real != "my.config.md" {
		t.Errorf("Resolve(my.config) = (%q, %v), want (my.config.md, true)", real, found)
	}
}

func TestResolver_EmptyConfig(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": &fstest.MapFile{Data: []byte("guide")},
	}

	r := Build(fsys, []string{}, hasHTMLRenderer)

	if !r.IsEmpty() {
		t.Error("resolver should be empty with no strip extensions")
	}
	_, found := r.Resolve("guide")
	if found {
		t.Error("should not resolve anything when disabled")
	}
}

func TestResolver_CleanPath(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"docs/guide.md": &fstest.MapFile{Data: []byte("guide")},
	}

	r := Build(fsys, []string{".md"}, hasHTMLRenderer)

	clean, found := r.CleanPath("docs/guide.md")
	if !found || clean != "docs/guide" {
		t.Errorf("CleanPath(docs/guide.md) = (%q, %v), want (docs/guide, true)", clean, found)
	}
}

func TestResolver_NonRenderedFileSkipped(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"data.csv": &fstest.MapFile{Data: []byte("a,b,c")},
	}

	// .csv in strip list but no renderer
	r := Build(fsys, []string{".csv"}, hasHTMLRenderer)

	_, found := r.Resolve("data")
	if found {
		t.Error("non-rendered file should not be resolved")
	}
}
```

- [ ] **Step 6: Run all resolver tests**

Run: `go test ./internal/resolve/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/resolve/
git commit -m "Add PathResolver for extensionless URL mapping"
```

---

### Task 3: Extension redirect middleware

**Files:**

- Create: `internal/server/redirect.go`
- Create: `internal/server/redirect_test.go`

- [ ] **Step 1: Write the test**

Create `internal/server/redirect_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/resolve"
)

func TestExtensionRedirect(t *testing.T) {
	t.Parallel()

	// Build a resolver that maps "guide" -> "guide.md"
	resolver := &resolve.PathResolver{} // Will need a test helper or mock

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantLoc    string
	}{
		{"redirect .md", "/docs/guide.md", http.StatusMovedPermanently, "/docs/guide"},
		{"pass extensionless", "/docs/guide", http.StatusOK, ""},
		{"pass non-stripped ext", "/image.jpg", http.StatusOK, ""},
		{"pass directory", "/docs/", http.StatusOK, ""},
		{"pass root", "/", http.StatusOK, ""},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := ExtensionRedirect(resolver, []string{".md"})(inner)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantLoc != "" {
				got := rec.Header().Get("Location")
				if got != tt.wantLoc {
					t.Errorf("Location = %q, want %q", got, tt.wantLoc)
				}
			}
		})
	}
}
```

Note: The test will need adjustment based on the exact `PathResolver` API — the resolver needs to be built with a test filesystem. Update accordingly:

```go
func TestExtensionRedirect(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"docs/guide.md": &fstest.MapFile{Data: []byte("# Guide")},
	}
	resolver := resolve.Build(fsys, []string{".md"}, func(mime string) bool {
		return mime == "text/markdown"
	})

	// ... rest of test as above ...
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestExtensionRedirect -v`
Expected: FAIL — `ExtensionRedirect` not defined.

- [ ] **Step 3: Implement the middleware**

Create `internal/server/redirect.go`:

```go
package server

import (
	"net/http"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/resolve"
)

// ExtensionRedirect returns middleware that 301-redirects requests with
// strippable extensions to their canonical extensionless URL.
// For example, /docs/guide.md -> /docs/guide when .md is stripped.
func ExtensionRedirect(resolver *resolve.PathResolver, stripExts []string) func(http.Handler) http.Handler {
	extSet := make(map[string]bool, len(stripExts))
	for _, ext := range stripExts {
		extSet[ext] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ext := path.Ext(r.URL.Path)
			if ext == "" || !extSet[ext] {
				next.ServeHTTP(w, r)
				return
			}

			// Check if this file has a clean path mapping
			// Strip leading "/" for resolver lookup
			realPath := strings.TrimPrefix(r.URL.Path, "/")
			if cleanPath, found := resolver.CleanPath(realPath); found {
				http.Redirect(w, r, "/"+cleanPath, http.StatusMovedPermanently)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestExtensionRedirect -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/redirect.go internal/server/redirect_test.go
git commit -m "Add extension redirect middleware for canonical URLs"
```

---

### Task 4: Handler path resolution with resolver

**Files:**

- Modify: `internal/server/handler.go:26-57` (HandlerConfig, Handler, NewHandler)
- Modify: `internal/server/handler.go:67-135` (ServeContent)
- Test: `internal/server/handler_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/server/handler_test.go` a test that requests an extensionless path and verifies the handler resolves it through the resolver. Look at existing handler tests for the pattern (they likely use a mock provider). The test should:

```go
func TestServeContent_ExtensionlessPath(t *testing.T) {
	// Setup: mock provider that has "guide.md"
	// Build resolver: "guide" -> "guide.md"
	// Request GET /guide
	// Expect: 200 with rendered content
	// Verify: template context uses "/guide" as path (not "/guide.md")
}
```

Adapt to the existing test patterns in `handler_test.go`. The key assertion is that the handler resolves `/guide` by consulting the resolver and reading `guide.md` from the provider.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestServeContent_ExtensionlessPath -v`
Expected: FAIL — handler doesn't have resolver.

- [ ] **Step 3: Add resolver to handler**

In `internal/server/handler.go`, add to `HandlerConfig` and `Handler`:

```go
// In HandlerConfig (after URLRedirects):
Resolver *resolve.PathResolver

// In Handler (after urlRedirects):
resolver *resolve.PathResolver
```

In `NewHandler`, add:

```go
resolver: cfg.Resolver,
```

- [ ] **Step 4: Add resolution logic in ServeContent**

In `ServeContent`, after the provider read fails with a not-found error (around line 78), add resolver fallback before the error handling:

```go
// 1. Read file + get MIME type
content, mimeType, err := h.provider.ReadFile(r.Context(), r.URL.Path)
if err != nil {
	// Try resolver for extensionless paths
	if errors.Is(err, provider.ErrNotFound) && h.resolver != nil {
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if realPath, found := h.resolver.Resolve(cleanPath); found {
			content, mimeType, err = h.provider.ReadFile(r.Context(), "/"+realPath)
		}
	}
}
if err != nil {
	// existing error handling (directory redirect, etc.)
	...
}
```

Note: Check what error type `ReadFile` returns for not-found. It likely wraps `fs.ErrNotExist`. Adjust the error check accordingly (e.g., `errors.Is(err, fs.ErrNotExist)`).

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestServeContent_ExtensionlessPath -v`
Expected: PASS

- [ ] **Step 6: Run all handler tests**

Run: `go test ./internal/server/ -v`
Expected: PASS — no regressions.

- [ ] **Step 7: Commit**

```bash
git add internal/server/handler.go internal/server/handler_test.go
git commit -m "Resolve extensionless paths via PathResolver in handler"
```

---

### Task 5: Wire resolver into server startup

**Files:**

- Modify: `cmd/gomddoc/pipeline.go:26-44` (Pipeline struct)
- Modify: `cmd/gomddoc/pipeline.go:46-126` (setupPipeline)
- Modify: `internal/server/server.go:30-51` (HTTPServerConfig)
- Modify: `internal/server/server.go:54-155` (NewHTTPServer)

- [ ] **Step 1: Add resolver to Pipeline struct**

In `cmd/gomddoc/pipeline.go`, add to the `Pipeline` struct:

```go
Resolver *resolve.PathResolver
```

- [ ] **Step 2: Build resolver in setupPipeline**

In `setupPipeline()`, after the registry is built (around line 54), add:

```go
// Build path resolver for extensionless URLs
resolver := resolve.Build(contentRoot, cfg.Site.StripExtensions, func(mimeType string) bool {
	_, _, err := registry.Get(mimeType, []negotiate.MediaType{{Type: "text", Subtype: "html", Q: 1.0}})
	return err == nil
})
p.Resolver = resolver
```

This uses the registry itself to check if a MIME type has an HTML renderer.

- [ ] **Step 3: Pass resolver to HTTPServerConfig**

In `internal/server/server.go`, add to `HTTPServerConfig`:

```go
Resolver *resolve.PathResolver
```

In `cmd/gomddoc/pipeline.go`'s `setupServer()`, pass it in the `serverConfig` (around line 238):

```go
Resolver: pipeline.Resolver,
```

- [ ] **Step 4: Wire resolver in NewHTTPServer**

In `NewHTTPServer()`, pass resolver to `HandlerConfig`:

```go
Resolver: opts.Resolver,
```

Add the redirect middleware to the content middleware chain (between `ContentExclusion` and `Metrics`):

```go
content := auth.Subgroup("",
	Compression,
	NewMethodFilterMiddleware(http.MethodGet, http.MethodHead),
	ContentExclusion(cfg.Site.Exclude),
	ExtensionRedirect(opts.Resolver, cfg.Site.StripExtensions), // NEW
	Metrics,
)
```

- [ ] **Step 5: Run full test suite**

Run: `make ci`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/pipeline.go internal/server/server.go
git commit -m "Wire PathResolver into server startup and middleware"
```

---

### Task 6: Navigation emits clean paths

**Files:**

- Modify: `internal/template/navigation/navigation.go:24-38` (Generator struct)
- Modify: `internal/template/navigation/navigation.go:86-120` (buildTree file handling)
- Test: `internal/template/navigation/navigation_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/template/navigation/navigation_test.go`:

```go
func TestGenerate_CleanPaths(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md":   &fstest.MapFile{Data: []byte("# Guide")},
		"about.md":   &fstest.MapFile{Data: []byte("# About")},
		"image.jpg":  &fstest.MapFile{Data: []byte("binary")},
	}

	resolver := resolve.Build(fsys, []string{".md"}, func(mime string) bool {
		return mime == "text/markdown"
	})
	gen := NewGenerator(fsys, "README.md", nil, resolver)
	root := gen.Generate("guide.md")

	// Find the guide node
	for _, child := range root.Children {
		if child.Label == "Guide" {
			if child.Path != "/guide" {
				t.Errorf("Path = %q, want /guide", child.Path)
			}
			return
		}
	}
	t.Error("guide node not found")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/navigation/ -run TestGenerate_CleanPaths -v`
Expected: FAIL — `NewGenerator` doesn't accept resolver.

- [ ] **Step 3: Add resolver to Generator**

In `navigation.go`, add `resolver` field to `Generator`:

```go
type Generator struct {
	rootFS          fs.FS
	defaultIndex    string
	excludePatterns []string
	resolver        *resolve.PathResolver
}
```

Update `NewGenerator` to accept it:

```go
func NewGenerator(rootFS fs.FS, defaultIndex string, excludePatterns []string, resolver *resolve.PathResolver) *Generator {
	return &Generator{
		rootFS:          rootFS,
		defaultIndex:    defaultIndex,
		excludePatterns: excludePatterns,
		resolver:        resolver,
	}
}
```

In `buildTree()`, when creating file nodes (around line 103), use clean path if available:

```go
} else {
	urlPath := "/" + entryPath
	if g.resolver != nil {
		if clean, found := g.resolver.CleanPath(entryPath); found {
			urlPath = "/" + clean
		}
	}
	// ... rest of node creation
}
```

- [ ] **Step 4: Fix all existing callers of NewGenerator**

Update `setupPipeline()` in `pipeline.go` (line 92):

```go
navGen := navigation.NewGenerator(contentRoot, cfg.Site.DefaultIndex, cfg.Site.Exclude, p.Resolver)
```

Update build command if it creates a `NewGenerator` separately.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/template/navigation/ -v && make ci`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/template/navigation/navigation.go internal/template/navigation/navigation_test.go cmd/gomddoc/pipeline.go
git commit -m "Emit extensionless paths in navigation tree"
```

---

### Task 7: SEO URLs use clean paths

**Files:**

- Modify: `internal/seo/url.go`
- Modify: `internal/seo/url_test.go`
- Modify: `internal/server/sitemap.go`
- Modify: `internal/server/feed.go`

- [ ] **Step 1: Write the test**

Add to `internal/seo/url_test.go`:

```go
func TestPageURL_StripExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		domain   string
		pagePath string
		want     string
	}{
		{"strip .md", "example.com", "/docs/guide", "https://example.com/docs/guide"},
		{"directory path", "example.com", "/docs/", "https://example.com/docs/"},
		{"root", "example.com", "/", "https://example.com/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PageURL(tt.domain, tt.pagePath, "README.md")
			if got != tt.want {
				t.Errorf("PageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

The SEO `PageURL` function already handles path normalization. The change is at the call sites — sitemap and feed should pass clean paths instead of raw file paths.

- [ ] **Step 2: Update sitemap to use clean paths**

In `internal/server/sitemap.go`, `GenerateSitemap` needs access to the resolver. Update the signature:

```go
func GenerateSitemap(ctx context.Context, index *metadata.Index, domain, defaultIndex string, prov provider.Provider, resolver *resolve.PathResolver) ([]byte, error) {
```

In the loop (around line 88), use clean path:

```go
pagePath := "/" + page.Path
if resolver != nil {
	if clean, found := resolver.CleanPath(page.Path); found {
		pagePath = "/" + clean
	}
}
loc := seo.PageURL(domain, pagePath, defaultIndex)
```

- [ ] **Step 3: Update feed similarly**

In `internal/server/feed.go`, update `GenerateFeed` signature to accept resolver. Apply the same clean path logic when building entry URLs.

- [ ] **Step 4: Update all callers**

Update `NewSitemapHandler`, `NewFeedHandler`, and the build command's `generateSEOFiles` to pass the resolver.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/seo/ ./internal/server/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/seo/ internal/server/sitemap.go internal/server/feed.go cmd/gomddoc/build.go
git commit -m "Use extensionless paths in sitemap and feed URLs"
```

---

### Task 8: Build command — pretty URL output

**Files:**

- Modify: `cmd/gomddoc/build.go:231-301` (buildFile output path logic)
- Modify: `cmd/gomddoc/build.go:162-228` (walkAndBuild)
- Test: `cmd/gomddoc/build_test.go`

- [ ] **Step 1: Write the test**

Add to `cmd/gomddoc/build_test.go` (or create if needed):

```go
func TestBuildFile_PrettyURLOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{"regular md", "docs/guide.md", "docs/guide/index.html"},
		{"index.md stays", "docs/index.md", "docs/index.html"},
		{"root file", "about.md", "about/index.html"},
		{"nested", "a/b/c.md", "a/b/c/index.html"},
	}
	// Test the output path computation logic
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := prettyOutputPath(tt.filePath, "README.md", map[string]bool{})
			if got != tt.want {
				t.Errorf("prettyOutputPath(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/gomddoc/ -run TestBuildFile_PrettyURLOutput -v`
Expected: FAIL — `prettyOutputPath` not defined.

- [ ] **Step 3: Extract output path logic**

In `cmd/gomddoc/build.go`, extract the output path computation into a function:

```go
// prettyOutputPath computes the output path for pretty URLs.
// Regular files: guide.md -> guide/index.html
// Index files: index.md -> index.html (no double nesting)
// Default index: README.md -> index.html (when no index.md in dir)
func prettyOutputPath(filePath, defaultIndex string, dirsWithIndexMD map[string]bool) string {
	base := path.Base(filePath)

	// index.md stays as index.html — no double nesting
	if strings.EqualFold(base, "index.md") {
		return strings.TrimSuffix(filePath, path.Ext(filePath)) + ".html"
	}

	// Default index (README.md) becomes index.html when no index.md exists
	if strings.EqualFold(base, defaultIndex) && !dirsWithIndexMD[path.Dir(filePath)] {
		return path.Join(path.Dir(filePath), "index.html")
	}

	// Regular file: guide.md -> guide/index.html
	stem := strings.TrimSuffix(filePath, path.Ext(filePath))
	return path.Join(stem, "index.html")
}
```

- [ ] **Step 4: Update buildFile to use prettyOutputPath**

In `buildFile()`, replace the output path logic (lines 286-291):

```go
var htmlPath string
if len(siteConfig.StripExtensions) > 0 {
	htmlPath = prettyOutputPath(filePath, siteConfig.DefaultIndex, dirsWithIndexMD)
} else {
	htmlPath = strings.TrimSuffix(filePath, path.Ext(filePath)) + ".html"
	if b.isDefaultIndex(filePath, siteConfig.DefaultIndex) && !dirsWithIndexMD[path.Dir(filePath)] {
		htmlPath = path.Join(path.Dir(filePath), "index.html")
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./cmd/gomddoc/ -run TestBuildFile_PrettyURLOutput -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/build.go cmd/gomddoc/build_test.go
git commit -m "Generate pretty URL output in build mode"
```

---

### Task 9: Build — generate extension redirect pages

**Files:**

- Modify: `cmd/gomddoc/build.go:393-427` (generateRedirectFiles)

- [ ] **Step 1: Write the test**

Add to `cmd/gomddoc/build_test.go`:

```go
func TestGenerateExtensionRedirects(t *testing.T) {
	// Test that when strip_extensions is active, the build generates
	// redirect files from guide.md -> guide (no trailing slash)
	// The redirect file at guide.md should be an HTML meta-refresh
	// pointing to /guide
}
```

- [ ] **Step 2: Add extension redirect generation**

In `cmd/gomddoc/build.go`, after `generateRedirectFiles`, add a call to generate extension redirects. This can be integrated into the existing `generateRedirectFiles` or be a new function:

```go
func (b *BuildCmd) generateExtensionRedirects(resolver *resolve.PathResolver, siteConfig *config.SiteConfig) error {
	if resolver == nil || resolver.IsEmpty() {
		return nil
	}

	// For each mapped file, generate a redirect from the extension URL to the clean URL
	// e.g., guide.md redirects to /guide
	for realPath, cleanPath := range resolver.AllMappings() {
		html := server.GenerateRedirectHTML("/" + cleanPath)
		// Write at the original extension path (e.g., guide.md becomes a redirect file)
		if err := b.writeOutputFile(realPath, html); err != nil {
			return fmt.Errorf("write extension redirect %s: %w", realPath, err)
		}
		slog.Debug("Generated extension redirect",
			slog.String("from", "/"+realPath),
			slog.String("to", "/"+cleanPath),
		)
	}

	return nil
}
```

Add an `AllMappings` method to `PathResolver`:

```go
// AllMappings returns all real -> clean path mappings.
func (r *PathResolver) AllMappings() map[string]string {
	return r.toClean
}
```

- [ ] **Step 3: Call it from Build.Run()**

In `Build.Run()`, after `generateRedirectFiles` (around line 113):

```go
if err := b.generateExtensionRedirects(pipeline.Resolver, &cfg.Site); err != nil {
	return fmt.Errorf("generate extension redirects: %w", err)
}
```

- [ ] **Step 4: Run tests**

Run: `make ci`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/gomddoc/build.go internal/resolve/resolver.go
git commit -m "Generate extension redirect pages in build output"
```

---

### Task 10: Build — wire resolver into build pipeline

**Files:**

- Modify: `cmd/gomddoc/build.go:50-136` (Run)
- Modify: `cmd/gomddoc/build.go:162-228` (walkAndBuild)

- [ ] **Step 1: Pass resolver through build pipeline**

The build command creates its own pipeline via `setupPipeline()`, which already builds the resolver (from Task 5). Pass `pipeline.Resolver` through the build chain:

Update `walkAndBuild` signature to accept the resolver:

```go
func (b *BuildCmd) walkAndBuild(
	contentRoot fs.FS,
	registry renderer.RendererRegistry,
	enricherRegistry enricher.EnricherRegistry,
	templateRenderer *tmpl.HTMLRenderer,
	siteConfig *config.SiteConfig,
	resolver *resolve.PathResolver, // NEW
) (*buildStats, error) {
```

Pass resolver to `buildFile` and through to `prettyOutputPath`.

Update the call in `Run()`:

```go
stats, err := b.walkAndBuild(contentRoot, pipeline.Registry, pipeline.EnricherRegistry, pipeline.TemplateRenderer, &cfg.Site, pipeline.Resolver)
```

- [ ] **Step 2: Pass resolver to SEO generation**

Update `generateSEOFiles` to accept and pass the resolver to `GenerateSitemap` and `GenerateFeed`.

- [ ] **Step 3: Run full CI**

Run: `make ci`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/gomddoc/build.go
git commit -m "Wire resolver into build pipeline for SEO and pretty URLs"
```

---

### Task 11: Integration test — end-to-end serve mode

**Files:**

- Modify or create: an integration test file

- [ ] **Step 1: Write integration test**

Create a test that starts a full server with test content and verifies:

1. `GET /guide` → 200 with rendered content
2. `GET /guide.md` → 301 redirect to `/guide`
3. `GET /image.jpg` → 200 (not stripped)
4. `GET /nonexistent` → 404
5. Navigation links use extensionless paths

Use `httptest.NewServer` with the full handler stack or test against the handler directly.

- [ ] **Step 2: Run integration test**

Run: `go test ./... -run TestIntegration_ExtensionStripping -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add <test files>
git commit -m "Add integration tests for URL extension stripping"
```

---

### Task 12: Update documentation

**Files:**

- Modify: `docs/guide/02-configuration.md` (add strip_extensions docs)
- Modify: `docs/architecture.md` (mention resolver in pipeline)
- Modify: `docs/roadmap.md` (mark feature as done)

- [ ] **Step 1: Update configuration docs**

Add a section for `strip_extensions` in `docs/guide/02-configuration.md` explaining the config option, defaults, and behavior.

- [ ] **Step 2: Update architecture docs**

Add the `PathResolver` to the pipeline diagram in `docs/architecture.md`.

- [ ] **Step 3: Update roadmap**

Mark URL extension stripping as completed in `docs/roadmap.md`.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "Document URL extension stripping feature"
```

---

### Task 13: Final validation

- [ ] **Step 1: Run full CI**

Run: `make ci`
Expected: PASS with no warnings.

- [ ] **Step 2: Manual test with testsite**

Run: `make run` and verify with Chrome DevTools MCP against `testsite/`:

- Extensionless URLs work
- Extension URLs redirect
- Navigation shows clean URLs
- Build output uses pretty URLs

- [ ] **Step 3: Run build and inspect output**

```bash
go run ./cmd/gomddoc build --dir testsite --output /tmp/testbuild --force
find /tmp/testbuild -name "*.html" | head -20
```

Verify directory structure shows `guide/index.html` pattern.
