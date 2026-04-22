# Theme Feature Toggles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace one-off `ColorChips` and `HasSearch` fields with a generic `Features map[string]bool` on `SiteConfig`, add a `feature` template function with per-page frontmatter override, and gate all conditional theme features behind it.

**Architecture:** `SiteConfig.Features` is a `map[string]bool` loaded from YAML `features:` key. A `featureEnabled(name, features)` function defaults to `true` for unknown keys. Templates use `{{ feature "name" }}` via a closure rebuilt per page render with merged site+frontmatter features. The renderer uses `featureEnabled` to gate post-processors.

**Tech Stack:** Go 1.25+, goldmark, html/template

**Spec:** `docs/specs/2026-03-31-theme-feature-toggles-design.md`

---

### Task 1: Add `featureEnabled` and `mergeFeatures` functions

**Files:**
- Create: `internal/config/features.go`
- Test: `internal/config/features_test.go`

- [ ] **Step 1: Write tests for `featureEnabled`**

```go
// internal/config/features_test.go
package config

import "testing"

func TestFeatureEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		feature  string
		features map[string]bool
		want     bool
	}{
		{
			name:     "nil map defaults to true",
			feature:  "katex",
			features: nil,
			want:     true,
		},
		{
			name:     "empty map defaults to true",
			feature:  "katex",
			features: map[string]bool{},
			want:     true,
		},
		{
			name:     "explicitly enabled",
			feature:  "katex",
			features: map[string]bool{"katex": true},
			want:     true,
		},
		{
			name:     "explicitly disabled",
			feature:  "katex",
			features: map[string]bool{"katex": false},
			want:     false,
		},
		{
			name:     "other features present but not this one",
			feature:  "mermaid",
			features: map[string]bool{"katex": false},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FeatureEnabled(tt.feature, tt.features); got != tt.want {
				t.Errorf("FeatureEnabled(%q, %v) = %v, want %v", tt.feature, tt.features, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/config/ -run TestFeatureEnabled -v`
Expected: FAIL — `FeatureEnabled` undefined

- [ ] **Step 3: Implement `featureEnabled`**

```go
// internal/config/features.go
package config

import "maps"

// FeatureEnabled returns whether a named feature is enabled in the given map.
// Unknown keys default to true (all features enabled by default).
func FeatureEnabled(name string, features map[string]bool) bool {
	if v, ok := features[name]; ok {
		return v
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/config/ -run TestFeatureEnabled -v`
Expected: PASS

- [ ] **Step 5: Write tests for `MergeFeatures`**

Add to `internal/config/features_test.go`:

```go
func TestMergeFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		site     map[string]bool
		pageMeta map[string]any
		feature  string
		want     bool
	}{
		{
			name:     "nil site and nil meta defaults true",
			site:     nil,
			pageMeta: nil,
			feature:  "katex",
			want:     true,
		},
		{
			name:     "site disables, no page override",
			site:     map[string]bool{"katex": false},
			pageMeta: nil,
			feature:  "katex",
			want:     false,
		},
		{
			name:     "site disables, page enables",
			site:     map[string]bool{"katex": false},
			pageMeta: map[string]any{"features": map[string]any{"katex": true}},
			feature:  "katex",
			want:     true,
		},
		{
			name:     "site enables, page disables",
			site:     map[string]bool{"katex": true},
			pageMeta: map[string]any{"features": map[string]any{"katex": false}},
			feature:  "katex",
			want:     false,
		},
		{
			name:     "page features with non-bool value ignored",
			site:     map[string]bool{"katex": false},
			pageMeta: map[string]any{"features": map[string]any{"katex": "yes"}},
			feature:  "katex",
			want:     false,
		},
		{
			name:     "page features key is not a map",
			site:     map[string]bool{"katex": false},
			pageMeta: map[string]any{"features": "invalid"},
			feature:  "katex",
			want:     false,
		},
		{
			name:     "page has unrelated frontmatter only",
			site:     map[string]bool{"mermaid": false},
			pageMeta: map[string]any{"title": "Hello"},
			feature:  "mermaid",
			want:     false,
		},
		{
			name:     "does not mutate site map",
			site:     map[string]bool{"katex": true},
			pageMeta: map[string]any{"features": map[string]any{"katex": false}},
			feature:  "katex",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Clone site to detect mutation
			var siteBefore map[string]bool
			if tt.site != nil {
				siteBefore = maps.Clone(tt.site)
			}

			merged := MergeFeatures(tt.site, tt.pageMeta)
			got := FeatureEnabled(tt.feature, merged)
			if got != tt.want {
				t.Errorf("FeatureEnabled(%q, MergeFeatures(%v, %v)) = %v, want %v",
					tt.feature, tt.site, tt.pageMeta, got, tt.want)
			}

			// Verify site map was not mutated
			if tt.site != nil && !maps.Equal(tt.site, siteBefore) {
				t.Error("MergeFeatures mutated the site map")
			}
		})
	}
}
```

- [ ] **Step 6: Implement `MergeFeatures`**

Add to `internal/config/features.go`:

```go
// MergeFeatures creates a merged features map from site config and page frontmatter.
// Page frontmatter overrides site defaults. Returns a new map (never mutates inputs).
func MergeFeatures(site map[string]bool, pageMeta map[string]any) map[string]bool {
	merged := maps.Clone(site)
	if merged == nil {
		merged = make(map[string]bool)
	}
	if pageMeta == nil {
		return merged
	}
	features, ok := pageMeta["features"]
	if !ok {
		return merged
	}
	fm, ok := features.(map[string]any)
	if !ok {
		return merged
	}
	for k, v := range fm {
		if b, ok := v.(bool); ok {
			merged[k] = b
		}
	}
	return merged
}
```

- [ ] **Step 7: Run all tests**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/config/ -run "TestFeatureEnabled|TestMergeFeatures" -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/config/features.go internal/config/features_test.go
git commit -m "feat: add FeatureEnabled and MergeFeatures for generic feature toggles"
```

---

### Task 2: Migrate `SiteConfig` — replace `ColorChips` and `HasSearch` with `Features`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/features.go` (add validation)
- Modify: `internal/config/features_test.go` (add validation tests)
- Modify: `cmd/gomddoc/init.go`

- [ ] **Step 1: Write validation test**

Add to `internal/config/features_test.go`:

```go
func TestValidateFeatureKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		keys    map[string]bool
		wantErr bool
	}{
		{name: "nil map", keys: nil, wantErr: false},
		{name: "empty map", keys: map[string]bool{}, wantErr: false},
		{name: "valid keys", keys: map[string]bool{"katex": true, "dark_mode": false}, wantErr: false},
		{name: "starts with digit", keys: map[string]bool{"1abc": true}, wantErr: true},
		{name: "uppercase", keys: map[string]bool{"KaTeX": true}, wantErr: true},
		{name: "hyphen", keys: map[string]bool{"dark-mode": true}, wantErr: true},
		{name: "empty key", keys: map[string]bool{"": true}, wantErr: true},
		{name: "starts with underscore", keys: map[string]bool{"_private": true}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFeatureKeys(tt.keys)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFeatureKeys(%v) error = %v, wantErr %v", tt.keys, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Implement `ValidateFeatureKeys`**

Add to `internal/config/features.go`:

```go
import (
	"fmt"
	"maps"
	"regexp"
)

var featureKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ValidateFeatureKeys validates that all feature keys match the allowed pattern.
func ValidateFeatureKeys(features map[string]bool) error {
	for k := range features {
		if !featureKeyPattern.MatchString(k) {
			return fmt.Errorf("invalid feature key %q: must match %s", k, featureKeyPattern.String())
		}
	}
	return nil
}
```

- [ ] **Step 3: Run validation test**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/config/ -run TestValidateFeatureKeys -v`
Expected: PASS

- [ ] **Step 4: Update `SiteConfig` struct**

In `internal/config/config.go`, replace:

```go
type SiteConfig struct {
	DefaultIndex string          `env:"DEFAULT_INDEX" yaml:"default_index"`
	DirIndex     bool            `env:"DIR_INDEX" yaml:"dir_index"`
	EditURL      string          `env:"EDIT_URL" yaml:"edit_url"`
	ColorChips   bool            `env:"COLOR_CHIPS" yaml:"color_chips"`
	Language     string          `env:"LANGUAGE" yaml:"language"`
	Meta         MetaConfig      `env:"META" yaml:"meta"`
	Theme        ThemeConfig     `env:"THEME" yaml:"theme"`
	Highlighting HighlightConfig `env:"HIGHLIGHTING" yaml:"highlighting"`
	HasSearch    bool            `yaml:"-"` // Set at runtime, not from config file
}
```

With:

```go
type SiteConfig struct {
	DefaultIndex string          `env:"DEFAULT_INDEX" yaml:"default_index"`
	DirIndex     bool            `env:"DIR_INDEX" yaml:"dir_index"`
	EditURL      string          `env:"EDIT_URL" yaml:"edit_url"`
	Language     string          `env:"LANGUAGE" yaml:"language"`
	Meta         MetaConfig      `env:"META" yaml:"meta"`
	Theme        ThemeConfig     `env:"THEME" yaml:"theme"`
	Highlighting HighlightConfig `env:"HIGHLIGHTING" yaml:"highlighting"`
	Features     map[string]bool `yaml:"features"`
}
```

Notes:
- `ColorChips` removed — migrated to `Features["color_chips"]`
- `HasSearch` removed — migrated to `Features["search"]`
- `Features` has no `env` tag — env override for map types requires custom handling (see Task 3)

- [ ] **Step 5: Update `NewSiteConfig`**

In `internal/config/config.go`, remove `ColorChips: true` from `NewSiteConfig`. The Features map starts nil — `FeatureEnabled()` defaults everything to `true`.

```go
func NewSiteConfig(dir string) SiteConfig {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	basename := filepath.Base(absDir)
	if basename == "." || basename == string(filepath.Separator) {
		basename = "Documentation"
	}
	title := text.TitleCase(basename)

	return SiteConfig{
		DefaultIndex: DefaultIndex,
		DirIndex:     false,
		Language:     "en",
		Meta: MetaConfig{
			Title: title,
		},
		Theme: ThemeConfig{
			Name: DefaultThemeName,
		},
		Highlighting: HighlightConfig{
			Theme: DefaultHighlightTheme,
		},
	}
}
```

- [ ] **Step 6: Add feature key validation to `SiteConfig.Validate()`**

In `internal/config/config.go`, add at the end of `SiteConfig.Validate()`:

```go
if err := ValidateFeatureKeys(sc.Features); err != nil {
	return err
}
```

- [ ] **Step 7: Update `initConfig` in `cmd/gomddoc/init.go`**

Replace the `initConfig` struct and `generateConfigYAML`:

```go
type initConfig struct {
	DefaultIndex string          `yaml:"default_index"`
	DirIndex     bool            `yaml:"dir_index"`
	Meta         initMetaConfig  `yaml:"meta"`
	Theme        initThemeConfig `yaml:"theme"`
	Highlighting initHLConfig    `yaml:"highlighting"`
}
```

Remove `ColorChips: true` from `generateConfigYAML`. The `features` key is omitted from init config since all features default to true.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/config/features.go internal/config/features_test.go cmd/gomddoc/init.go
git commit -m "feat: replace ColorChips and HasSearch with Features map on SiteConfig"
```

---

### Task 3: Add env var override for Features map

**Files:**
- Modify: `internal/config/config.go` (extend `walkStruct` for map[string]bool)
- Modify: `internal/config/config_test.go` (add env override test)

The existing `walkStruct` handles `string`, `bool`, `int`, `duration` but not maps. For `Features`, env vars follow the pattern `GOMDDOC_SITE_FEATURES_KATEX=false`.

- [ ] **Step 1: Write env override test for features**

Find the existing env override tests in `internal/config/config_test.go` and add a test case. The test should:

```go
func TestSiteConfig_ApplyEnvOverrides_Features(t *testing.T) {
	t.Setenv("GOMDDOC_SITE_FEATURES_KATEX", "false")
	t.Setenv("GOMDDOC_SITE_FEATURES_MERMAID", "false")

	sc := NewSiteConfig(".")
	sc.ApplyEnvOverrides()

	if FeatureEnabled("katex", sc.Features) {
		t.Error("katex should be disabled via env var")
	}
	if FeatureEnabled("mermaid", sc.Features) {
		t.Error("mermaid should be disabled via env var")
	}
	if !FeatureEnabled("dark_mode", sc.Features) {
		t.Error("dark_mode should still default to true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/config/ -run TestSiteConfig_ApplyEnvOverrides_Features -v`
Expected: FAIL

- [ ] **Step 3: Add map[string]bool handling to `walkStruct`**

In `internal/config/config.go`, in the `walkStruct` function, add a case for `reflect.Map` after the `reflect.Pointer` case:

```go
if kind == reflect.Map && field.Type() == reflect.TypeFor[map[string]bool]() {
	if envTag == "" {
		continue
	}
	mapPrefix := prefix + "_" + envTag + "_"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, mapPrefix) {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(parts[0], mapPrefix))
		if boolValue, err := strconv.ParseBool(parts[1]); err == nil {
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}
			field.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(boolValue))
			slog.Debug("Applied env override",
				slog.String("var", parts[0]),
				slog.String("kind", "map[string]bool"))
		}
	}
	continue
}
```

Also add the `env:"FEATURES"` tag to the `Features` field in `SiteConfig`:

```go
Features     map[string]bool `env:"FEATURES" yaml:"features"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/config/ -run TestSiteConfig_ApplyEnvOverrides_Features -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add env var override support for Features map (GOMDDOC_SITE_FEATURES_*)"
```

---

### Task 4: Migrate renderer to use `Features`

**Files:**
- Modify: `internal/renderer/markdown.go`
- Modify: `internal/renderer/colorchip_test.go`
- Modify: `internal/renderer/markdown_test.go`
- Modify: `internal/renderer/highlighting_test.go`
- Modify: `internal/renderer/admonition_test.go`
- Modify: `internal/renderer/markdown_bench_test.go`

- [ ] **Step 1: Update `MarkdownOptions` and `MarkdownRenderer`**

In `internal/renderer/markdown.go`:

Replace `MarkdownOptions`:
```go
type MarkdownOptions struct {
	// HighlightTheme controls the Chroma syntax highlighting style for fenced
	// code blocks. Defaults to "github" when empty.
	HighlightTheme string

	// Features maps feature names to enabled/disabled state.
	// Used to gate post-processors (color_chips, heading_anchors, admonitions).
	// Nil or missing keys default to enabled.
	Features map[string]bool
}
```

Replace `MarkdownRenderer`:
```go
type MarkdownRenderer struct {
	md       goldmark.Markdown
	features map[string]bool
}
```

Update `NewMarkdownRenderer` to store `features`:
```go
return &MarkdownRenderer{
	md:       md,
	features: opts.Features,
}
```

- [ ] **Step 2: Update `Render` method**

Replace the post-processing section in `Render()`:

```go
// Post-process: conditionally apply post-processors based on features.
// Per-page frontmatter can override site-level feature settings.
var metadata map[string]any
if enrichment != nil {
	metadata = enrichment.Metadata
}
merged := config.MergeFeatures(m.features, metadata)

rendered := buf.Bytes()
if config.FeatureEnabled("heading_anchors", merged) {
	rendered = addHeadingAnchors(rendered)
}
if config.FeatureEnabled("admonitions", merged) {
	rendered = transformAdmonitions(rendered)
}
if config.FeatureEnabled("color_chips", merged) {
	rendered = transformColorChips(rendered)
}
```

Add `"github.com/monolithiclab/gomddoc/internal/config"` to imports.

Remove the `colorChipsEnabled` function entirely.

- [ ] **Step 3: Update all test files**

Replace `ColorChips: true` with `Features: map[string]bool{"color_chips": true}` in:

- `internal/renderer/markdown_test.go`: all `NewMarkdownRenderer(MarkdownOptions{ColorChips: true})` calls
- `internal/renderer/highlighting_test.go`: lines 75, 98
- `internal/renderer/admonition_test.go`: line 239
- `internal/renderer/markdown_bench_test.go`: line 798

Update `internal/renderer/colorchip_test.go`:
- `TestColorChipsEnabled` — delete this test (function removed)
- `TestMarkdownRenderer_ColorChips_Disabled` — update to use `Features` instead of `ColorChips`:
  - `NewMarkdownRenderer(MarkdownOptions{})` stays (nil features = all enabled, but color_chips not explicitly set means enabled — need to pass `Features: map[string]bool{"color_chips": false}` for "globally disabled")
  - Update enrichment metadata from `map[string]any{"color_chips": false}` to `map[string]any{"features": map[string]any{"color_chips": false}}`
  - Similarly for the "enables" case

- [ ] **Step 4: Run all renderer tests**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/renderer/markdown.go internal/renderer/colorchip_test.go internal/renderer/markdown_test.go internal/renderer/highlighting_test.go internal/renderer/admonition_test.go internal/renderer/markdown_bench_test.go
git commit -m "feat: migrate renderer from ColorChips bool to Features map"
```

---

### Task 5: Update pipeline and server to pass `Features`

**Files:**
- Modify: `cmd/gomddoc/pipeline.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/server/testhelpers_test.go`
- Modify: `cmd/gomddoc/build_test.go`

- [ ] **Step 1: Update pipeline to pass Features**

In `cmd/gomddoc/pipeline.go`, change `setupPipeline`:

```go
registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{
	HighlightTheme: cfg.Site.Highlighting.Theme,
	Features:       cfg.Site.Features,
}))
```

- [ ] **Step 2: Update HasSearch migration in pipeline**

Replace `cfg.Site.HasSearch = true` with:

```go
if cfg.Site.Features == nil {
	cfg.Site.Features = make(map[string]bool)
}
cfg.Site.Features["search"] = true
```

- [ ] **Step 3: Update JSON-LD in template renderer**

In `internal/template/renderer.go`, change `generateJSONLD`:

```go
cfg := seo.JSONLDConfig{
	Domain:       h.siteConfig.Meta.Domain,
	SiteName:     h.siteConfig.Meta.Title,
	DefaultIndex: h.siteConfig.DefaultIndex,
	HasSearch:    config.FeatureEnabled("search", h.siteConfig.Features),
}
```

Add `"github.com/monolithiclab/gomddoc/internal/config"` to imports.

- [ ] **Step 4: Update test files**

In `internal/server/server_test.go` and `internal/server/testhelpers_test.go`:
Replace `renderer.MarkdownOptions{ColorChips: true}` with `renderer.MarkdownOptions{}` (nil features = all enabled).

In `internal/template/renderer_test.go`:
Replace `siteConfig.HasSearch = true` with:
```go
siteConfig.Features = map[string]bool{"search": true}
```

In `cmd/gomddoc/init_test.go`:
- Remove the `ColorChips` assertion (`sc.ColorChips != true`)
- Remove the `color_chips` YAML assertion
- Optionally add assertion that `Features` is nil (all default true)

- [ ] **Step 5: Run all tests**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./... 2>&1 | tail -30`
Expected: PASS (all packages)

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/pipeline.go internal/template/renderer.go internal/server/server_test.go internal/server/testhelpers_test.go internal/template/renderer_test.go cmd/gomddoc/init_test.go cmd/gomddoc/build_test.go
git commit -m "feat: wire Features through pipeline, server, and JSON-LD"
```

---

### Task 6: Add `feature` template function

**Files:**
- Modify: `internal/template/renderer.go`
- Modify: `internal/template/renderer_test.go`

The `feature` function needs access to per-page merged features. Since templates are cached but executed per-request, the closure must be injected per render via template execution data, not via `funcMap()` (which is fixed at parse time).

The approach: add `feature` to `funcMap()` as a no-op placeholder (so templates parse), then override it per render via `template.Execute` data that includes a helper method.

Actually, a simpler approach: add a `Feature` method to `TemplateContext` that templates call as `{{ .Feature "katex" }}`. This avoids the closure/funcMap complexity.

Wait — the spec says `{{ feature "katex" }}` (lowercase function), not a method. To make this work with cached templates, we need to inject the function at parse time but have it read from the execution data. Go templates support this via `template.Funcs` — but functions are fixed at parse time.

The correct approach per the spec: rebuild the `feature` closure per page render. Since templates are cached, we can't change `funcMap` per render. Instead, we put the merged features in `TemplateContext` and define `feature` in funcMap to panic (it should never be called directly). Then we use `template.Clone()` + `Funcs()` per render to override it.

Actually, the simplest Go template approach: put a `Feature` method on `TemplateContext` and use `{{ .Feature "katex" }}` in templates. This is cleaner and avoids clone overhead.

Let me re-read the spec... It says:
> Templates access features via the `feature` template function — a closure

But then says:
> The `feature` closure is rebuilt per page render with the merged map. Cached templates are unaffected — the closure is injected via the template execution data, not the template definition.

The cleanest way to honor the spec with cached templates: add a `Feature` method to `TemplateContext`. Templates call `{{ .Feature "katex" }}`. This is functionally identical to the spec's intent — per-page resolution — just uses Go method dispatch instead of a closure in funcMap.

- [ ] **Step 1: Write test for feature template function**

Add to `internal/template/renderer_test.go`:

```go
func TestTemplateContext_Feature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		features map[string]bool
		feature  string
		want     bool
	}{
		{
			name:     "nil features defaults true",
			features: nil,
			feature:  "katex",
			want:     true,
		},
		{
			name:     "explicitly disabled",
			features: map[string]bool{"katex": false},
			feature:  "katex",
			want:     false,
		},
		{
			name:     "explicitly enabled",
			features: map[string]bool{"katex": true},
			feature:  "katex",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := &TemplateContext{
				Site: &config.SiteConfig{Features: tt.features},
				Page: PageContext{Meta: map[string]any{}},
			}
			if got := ctx.Feature(tt.feature); got != tt.want {
				t.Errorf("Feature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Implement `Feature` method on `TemplateContext`**

In `internal/template/renderer.go`:

```go
// Feature returns whether a named feature is enabled for this page.
// It checks page frontmatter first, then site config, defaulting to true.
func (tc *TemplateContext) Feature(name string) bool {
	merged := config.MergeFeatures(tc.Site.Features, tc.Page.Meta)
	return config.FeatureEnabled(name, merged)
}
```

- [ ] **Step 3: Run test**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/template/ -run TestTemplateContext_Feature -v`
Expected: PASS

- [ ] **Step 4: Write integration test with per-page override**

```go
func TestTemplateContext_Feature_PageOverride(t *testing.T) {
	t.Parallel()

	ctx := &TemplateContext{
		Site: &config.SiteConfig{
			Features: map[string]bool{"katex": true, "mermaid": false},
		},
		Page: PageContext{
			Meta: map[string]any{
				"features": map[string]any{"katex": false},
			},
		},
	}

	if ctx.Feature("katex") {
		t.Error("katex should be disabled by page override")
	}
	if ctx.Feature("mermaid") {
		t.Error("mermaid should be disabled by site config")
	}
	if !ctx.Feature("dark_mode") {
		t.Error("dark_mode should default to true")
	}
}
```

- [ ] **Step 5: Run test**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/template/ -run TestTemplateContext_Feature -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/template/renderer.go internal/template/renderer_test.go
git commit -m "feat: add Feature method to TemplateContext for per-page feature toggles"
```

---

### Task 7: Update default theme templates

**Files:**
- Modify: `cmd/gomddoc/assets/themes/default/partials/scripts.html.tmpl`
- Modify: `cmd/gomddoc/assets/themes/default/partials/header.html.tmpl`
- Modify: `cmd/gomddoc/assets/themes/default/partials/toc.html.tmpl`

- [ ] **Step 1: Update `scripts.html.tmpl`**

Wrap each feature in `{{ if .Feature "name" }}`:

```html
{{ define "scripts" }}
  {{ if .Feature "dark_mode" }}
  <!-- Theme Toggle -->
  <script>{{ inlineJSAsset "theme-toggle.mjs" }}</script>
  {{ end }}

  <script>
    // Mobile Navigation Toggle
    (function() {
      'use strict';

      const navToggle = document.getElementById('nav-toggle');
      const navSidebar = document.getElementById('nav-sidebar');
      const navOverlay = document.getElementById('nav-overlay');

      if (navToggle && navSidebar) {
        const closeNav = () => {
          navSidebar.classList.remove('show');
          if (navOverlay) navOverlay.classList.remove('show');
        };

        navToggle.addEventListener('click', () => {
          const isShown = navSidebar.classList.contains('show');
          navSidebar.classList.toggle('show', !isShown);
          if (navOverlay) navOverlay.classList.toggle('show', !isShown);
        });
        if (navOverlay) navOverlay.addEventListener('click', closeNav);

        // Close on link click
        navSidebar.addEventListener('click', (e) => {
          if (e.target.closest('a')) {
            closeNav();
          }
        });
      }
    })();

    {{ if .Feature "toc" }}
    // Mobile TOC Toggle
    (function() {
      'use strict';

      const tocToggle = document.getElementById('toc-toggle');
      const tocSidebar = document.getElementById('toc-sidebar');
      const tocOverlay = document.getElementById('toc-overlay');

      if (tocToggle && tocSidebar) {
        const closeToc = () => {
          tocSidebar.classList.remove('show');
          if (tocOverlay) tocOverlay.classList.remove('show');
        };

        tocToggle.addEventListener('click', () => {
          const isShown = tocSidebar.classList.contains('show');
          tocSidebar.classList.toggle('show', !isShown);
          if (tocOverlay) tocOverlay.classList.toggle('show', !isShown);
        });
        if (tocOverlay) tocOverlay.addEventListener('click', closeToc);

        // Close on link click
        tocSidebar.addEventListener('click', (e) => {
          if (e.target.closest('a')) {
            closeToc();
          }
        });
      }
    })();
    {{ end }}
  </script>

  {{ if .Feature "toc" }}
  <!-- TOC Scroll Highlighting -->
  <script>{{ inlineJSAsset "toc-highlight.mjs" }}</script>
  {{ end }}

  {{ if .Feature "code_copy" }}
  <!-- Code Copy Button -->
  <script>{{ inlineJSAsset "code-copy.mjs" }}</script>
  {{ end }}

  {{ if .Feature "color_chips" }}
  <!-- Color Chip Web Component -->
  <script type="module">{{ inlineJSAsset "color-chip.mjs" }}</script>
  {{ end }}

  {{ if .Feature "search" }}
  <!-- Search Modal -->
  <script type="module">{{ inlineJSAsset "search.mjs" }}</script>
  {{ end }}

  {{ if .Feature "katex" }}
  <!-- KaTeX Auto-render -->
  <script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.js"></script>
  <script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/contrib/auto-render.min.js"
    onload="renderMathInElement(document.body, {
      delimiters: [
        {left: '$$', right: '$$', display: true},
        {left: '$', right: '$', display: false}
      ]
    });"></script>
  {{ end }}

  {{ if .Feature "mermaid" }}
  <!-- Mermaid -->
  <script type="module">
    import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
    mermaid.initialize({
      startOnLoad: false,
      theme: document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'default'
    });
    await mermaid.run({ querySelector: '.language-mermaid' });
  </script>
  {{ end }}
{{ end }}
```

- [ ] **Step 2: Update `header.html.tmpl`**

Wrap theme toggle and search button:

```html
{{ define "header" }}
  <header class="site-header">
    <div class="site-title">
      <button id="nav-toggle" aria-label="Toggle navigation">☰</button>
      <a href="/">{{ .Site.Meta.Title }}</a>
    </div>
    <div class="header-controls">
      {{ if .Feature "search" }}
      <button id="search-toggle" aria-label="Search documentation" title="Search (Ctrl+K)">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
      </button>
      {{ end }}
      {{ if .Feature "dark_mode" }}
      <button id="theme-toggle" aria-label="Toggle dark mode">🌙</button>
      {{ end }}
    </div>
  </header>
{{ end }}
```

- [ ] **Step 3: Update `toc.html.tmpl`**

Wrap entire TOC in feature check:

```html
{{ define "toc-item" -}}
<li>
  <a href="#{{ .ID }}">{{ .Text }}</a>
  {{- if .Children }}
  <ul>{{ range .Children }}{{ template "toc-item" . }}{{ end }}</ul>
  {{- end }}
</li>
{{- end }}

{{ define "toc" }}
{{- if .Feature "toc" }}
{{- $items := toc .Page.TOC }}
{{- if $items }}
          <aside id="toc-sidebar" aria-label="Table of Contents">
            <nav>
              <header>On this page</header>
              <ul>{{ range $items }}{{ template "toc-item" . }}{{ end }}</ul>
            </nav>
          </aside>
          <div id="toc-overlay"></div>
          <button id="toc-toggle" aria-label="Toggle table of contents">
            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="3" y1="12" x2="21" y2="12"></line>
              <line x1="3" y1="6" x2="21" y2="6"></line>
              <line x1="3" y1="18" x2="21" y2="18"></line>
            </svg>
          </button>
{{- end }}
{{- end }}
{{ end }}
```

- [ ] **Step 4: Run `make ci`**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/gomddoc/assets/themes/default/partials/scripts.html.tmpl cmd/gomddoc/assets/themes/default/partials/header.html.tmpl cmd/gomddoc/assets/themes/default/partials/toc.html.tmpl
git commit -m "feat: gate default theme features behind feature toggle checks"
```

---

### Task 8: Update all 7 material themes

**Files:**
- Modify: `material/themes/*/partials/scripts.html.tmpl` (7 files)
- Modify: `material/themes/*/partials/header.html.tmpl` (7 files)
- Modify: `material/themes/*/partials/toc.html.tmpl` (7 files)

Each theme's templates need the same `{{ .Feature "name" }}` guards applied. The exact wrapping points vary per theme due to different HTML/JS structure, but the pattern is identical:

- `scripts.html.tmpl`: wrap `theme-toggle.mjs` in `{{ if .Feature "dark_mode" }}`, `toc-highlight.mjs` in `{{ if .Feature "toc" }}`, `code-copy.mjs` in `{{ if .Feature "code_copy" }}`, `color-chip.mjs` in `{{ if .Feature "color_chips" }}`, `search.mjs` in `{{ if .Feature "search" }}`, KaTeX in `{{ if .Feature "katex" }}`, Mermaid in `{{ if .Feature "mermaid" }}`, TOC toggle JS in `{{ if .Feature "toc" }}`
- `header.html.tmpl`: wrap search button in `{{ if .Feature "search" }}`, theme toggle in `{{ if .Feature "dark_mode" }}`
- `toc.html.tmpl`: wrap entire TOC output in `{{ if .Feature "toc" }}`

**This task should be dispatched to parallel subagents — one per theme** since each theme's templates differ in structure but the transformation is mechanical.

- [ ] **Step 1: Update academic theme** (scripts, header, toc)
- [ ] **Step 2: Update gitbook theme** (scripts, header, toc)
- [ ] **Step 3: Update material theme** (scripts, header, toc)
- [ ] **Step 4: Update midnight theme** (scripts, header, toc)
- [ ] **Step 5: Update minimal theme** (scripts, header, toc)
- [ ] **Step 6: Update nord theme** (scripts, header, toc)
- [ ] **Step 7: Update ocean theme** (scripts, header, toc)

For each theme, read the existing template, identify the asset/component blocks, wrap them with the appropriate `{{ if .Feature "name" }}...{{ end }}` guard. Preserve the theme's existing formatting and any theme-specific JS logic.

- [ ] **Step 8: Run `make ci`**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add material/themes/
git commit -m "feat: gate all 7 material theme features behind feature toggle checks"
```

---

### Task 9: Final verification and cleanup

**Files:**
- Verify: all tests pass
- Verify: no remaining references to `ColorChips` or `HasSearch` in non-test code

- [ ] **Step 1: Search for stale references**

```bash
cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc
grep -r "ColorChips\|HasSearch\|colorChips\|color_chips" --include="*.go" | grep -v _test.go | grep -v features.go
```

Expected: no matches (all migrated)

- [ ] **Step 2: Run full CI**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: PASS

- [ ] **Step 3: Verify with a quick manual check**

Start the server with the testsite to verify feature toggles work:

```bash
cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc
go run ./cmd/gomddoc serve material/testsite
```

Open in browser, verify all features still load (default: all enabled).

- [ ] **Step 4: Commit any remaining fixes**

If any stale references or test failures were found, fix and commit.
