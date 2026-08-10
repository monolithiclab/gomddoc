# Tag Components — Chips, Listing & Index Pages: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render tag chips below the H1 of every tagged page, and serve per-tag listing pages (`/tags/{tag}`) plus a tag index page (`/tags/`) — all per-language, all server-rendered, also emitted in build mode.

**Architecture:** Reuses `metadata.Index.AllTags()` / `ByTag()` (already present), the per-language pipeline pattern (mirrors sitemap/feed routing), and the template renderer's partial inheritance system. New code is one validator, two template helpers, three theme partials, two render methods, two HTTP handlers, sitemap inclusion, build emission, and locale strings.

**Tech Stack:** Go 1.25+, `html/template`, `net/http` (Go 1.22+ pattern syntax), `fstest.MapFS` for tests, existing `internal/locale` package for translations.

**Spec:** `docs/specs/2026-04-23-tag-components-chips-and-pages.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/metadata/index.go` | Modify | Add `validTagPattern` + filter at parse time; warn on rejection |
| `internal/metadata/index_test.go` | Modify | Cover new validation rules |
| `internal/template/renderer.go` | Modify | Register `tagURL` and `pageTags` template helpers; add `RenderTagPage` and `RenderTagsIndex` methods |
| `internal/template/renderer_test.go` | Modify | Cover both render methods + helpers |
| `internal/server/tags_html.go` | Create | `TagPageHandler` (GET `/tags/{tag}`), `TagsIndexHandler` (GET `/tags/`) |
| `internal/server/tags_html_test.go` | Create | Handler tests covering 200/404 paths and per-language isolation |
| `internal/server/server.go` | Modify | Register tag routes for default lang + each non-default lang pipeline |
| `internal/server/sitemap.go` | Modify | Append per-language tag URLs to generated sitemap |
| `internal/server/sitemap_test.go` | Modify | Assert tag URLs included |
| `cmd/gomddoc/build.go` | Modify | Emit `tags/{escaped-tag}/index.html` + `tags/index.html` per language |
| `cmd/gomddoc/build_test.go` | Modify | Assert tag files generated for default + non-default languages |
| `cmd/gomddoc/serve.go` | Modify | Startup warning when `tags.md` or `tags/` directory exists in any content root |
| `cmd/gomddoc/assets/locales/en-US.yml` | Modify | Add `tags.title`, `tags.indexTitle`, `tags.taggedAs`, `tags.count`, `tags.empty` |
| `cmd/gomddoc/assets/themes/default/partials/tag-chips.html.tmpl` | Create | Inline chip list, gated by `tag_chips` feature flag |
| `cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl` | Create | Body markup for `/tags/{tag}` (search-result-style cards) |
| `cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl` | Create | Body markup for `/tags/` (alphabetical list with counts) |
| `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl` | Modify | Insert `{{ template "tag-chips" . }}` below H1 |
| `docs/roadmap.md` | Modify | Mark Tag Components sub-items 1, 2, 3 as done |

---

## Task 1: Tag charset validator

**Files:**
- Modify: `internal/metadata/index.go` (around lines 160-167 in `parseFrontmatter` / `extractTags`)
- Modify: `internal/metadata/index_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/metadata/index_test.go`:

```go
func TestBuildIndex_TagValidation(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"valid.md":    {Data: []byte("---\ntags:\n  - go\n  - machine learning\n  - core.runtime\n  - 2026\n---\n# Valid")},
		"with-bad.md": {Data: []byte("---\ntags:\n  - good\n  - bad/slash\n  - \"\"\n  - \"  \"\n---\n# Bad mix")},
	}

	idx, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	tags := idx.AllTags()
	for _, want := range []string{"go", "machine learning", "core.runtime", "2026", "good"} {
		if !slices.Contains(tags, want) {
			t.Errorf("AllTags missing %q (got %v)", want, tags)
		}
	}
	for _, banned := range []string{"bad/slash", "", "  "} {
		if slices.Contains(tags, banned) {
			t.Errorf("AllTags should not contain %q (got %v)", banned, tags)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metadata/ -run TestBuildIndex_TagValidation -v`
Expected: FAIL — `bad/slash` (and possibly empty/whitespace tags) present in `AllTags`.

- [ ] **Step 3: Implement the validator**

Replace the tag-parsing block in `internal/metadata/index.go` (currently `if v, ok := fm["tags"]`) with:

```go
if v, ok := fm["tags"]; ok {
    if tags, ok := v.([]any); ok {
        for _, tag := range tags {
            s, ok := tag.(string)
            if !ok {
                continue
            }
            normalized := strings.ToLower(strings.TrimSpace(s))
            if normalized == "" {
                continue
            }
            if strings.ContainsAny(normalized, "/\\") {
                slog.Warn("Skipping tag with invalid character",
                    slog.String("tag", s),
                    slog.String("path", page.Path),
                    slog.String("reason", "tags may not contain '/' or '\\'"))
                continue
            }
            page.Tags = append(page.Tags, normalized)
        }
    }
}
```

Verify `log/slog` is already imported in `internal/metadata/index.go`. If not, add it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/metadata/ -run TestBuildIndex_TagValidation -v`
Expected: PASS.

- [ ] **Step 5: Run the full metadata package**

Run: `go test ./internal/metadata/ -race`
Expected: all PASS — confirms existing tag tests still work.

- [ ] **Step 6: Commit**

```bash
git add internal/metadata/index.go internal/metadata/index_test.go
git commit -m "Validate tag charset: drop slash/empty/whitespace tags

Skips frontmatter tags containing '/' or '\\\\', plus empty and
whitespace-only entries, with a logged warning. Centralizes the
filter in metadata.Index so the search engine, related-docs
enricher, and tag pages all see the same cleaned set."
```

---

## Task 2: Template helpers `tagURL` and `pageTags`

**Files:**
- Modify: `internal/template/renderer.go` (extend `funcMap()`)
- Modify: `internal/template/renderer_test.go` (or create a focused helper test file)

- [ ] **Step 1: Write the failing test**

Add to `internal/template/renderer_test.go`:

```go
func TestTagURL(t *testing.T) {
	t.Parallel()

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, fstest.MapFS{})
	fn := r.funcMap()["tagURL"].(func(lang, tag string) string)

	tests := []struct {
		name, lang, tag, want string
	}{
		{"default lang", "", "go", "/tags/go"},
		{"default lang space", "", "machine learning", "/tags/machine%20learning"},
		{"non-default lang", "fr", "go", "/fr/tags/go"},
		{"non-default lang space", "de", "machine learning", "/de/tags/machine%20learning"},
		{"unicode tag", "", "café", "/tags/caf%C3%A9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fn(tt.lang, tt.tag); got != tt.want {
				t.Errorf("tagURL(%q, %q) = %q, want %q", tt.lang, tt.tag, got, tt.want)
			}
		})
	}
}

func TestPageTags(t *testing.T) {
	t.Parallel()

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, fstest.MapFS{})
	fn := r.funcMap()["pageTags"].(func(meta map[string]any) []string)

	tests := []struct {
		name string
		meta map[string]any
		want []string
	}{
		{"nil", nil, nil},
		{"missing key", map[string]any{"title": "x"}, nil},
		{"yaml-style []any", map[string]any{"tags": []any{"go", "docs"}}, []string{"go", "docs"}},
		{"already []string", map[string]any{"tags": []string{"go", "docs"}}, []string{"go", "docs"}},
		{"non-string entries skipped", map[string]any{"tags": []any{"go", 42, "docs"}}, []string{"go", "docs"}},
		{"non-list value", map[string]any{"tags": "go"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fn(tt.meta)
			if !slices.Equal(got, tt.want) {
				t.Errorf("pageTags(%v) = %v, want %v", tt.meta, got, tt.want)
			}
		})
	}
}
```

If `slices` is not yet imported in the test file, add it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/ -run "TestTagURL|TestPageTags" -v`
Expected: FAIL — `tagURL`/`pageTags` not in funcMap.

- [ ] **Step 3: Add the helpers**

In `internal/template/renderer.go`, extend `funcMap()`:

```go
func (h *HTMLRenderer) funcMap() template.FuncMap {
	return template.FuncMap{
		// ... existing entries ...
		"tagURL":   tagURL,
		"pageTags": pageTags,
	}
}
```

Add these standalone functions in the same file (place them near the other template funcs, e.g. after `contentURL`):

```go
// tagURL builds the URL for a tag's listing page, scoped to the active
// language. The default language uses /tags/{tag}; other languages use
// /{lang}/tags/{tag}. Tag values are percent-encoded for URL safety.
func tagURL(lang, tag string) string {
	if lang == "" {
		return "/tags/" + url.PathEscape(tag)
	}
	return "/" + lang + "/tags/" + url.PathEscape(tag)
}

// pageTags normalizes the YAML-decoded value of frontmatter "tags" into a
// []string. Returns nil when the key is absent, the value is the wrong type,
// or the list contains no strings.
func pageTags(meta map[string]any) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta["tags"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}
```

Add `"net/url"` and `"slices"` to the import block if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/template/ -run "TestTagURL|TestPageTags" -race -v`
Expected: PASS.

- [ ] **Step 5: Run the full template package**

Run: `go test ./internal/template/ -race`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/template/renderer.go internal/template/renderer_test.go
git commit -m "Add tagURL and pageTags template helpers

tagURL builds /tags/{tag} or /{lang}/tags/{tag} with PathEscape on
the tag segment. pageTags normalizes the frontmatter tags value
(YAML []any or []string) to a flat []string so partials can range
over it without inline type assertions."
```

---

## Task 3: Locale strings

**Files:**
- Modify: `cmd/gomddoc/assets/locales/en-US.yml`

- [ ] **Step 1: Write the failing test**

Add to `internal/locale/bundle_test.go` (or create a new file `cmd/gomddoc/assets/locales/locales_test.go` if no existing test fits):

```go
func TestBundle_TagKeys(t *testing.T) {
	t.Parallel()

	bundle, err := locale.LoadBundle("en-US", embeddedAssets, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	keys := []string{"tags_title", "tags_index_title", "tags_tagged_as", "tags_empty"}
	for _, k := range keys {
		v := bundle.T("en-US", k)
		if v == k || v == "" {
			t.Errorf("bundle.T(%q) returned %q; expected a localized string", k, v)
		}
	}
}
```

If the current locale tests live elsewhere or `embeddedAssets` is unexported, add the test next to existing locale tests (likely `internal/locale/`) and use a `fstest.MapFS` containing the new keys to validate the lookup contract — the integration check below catches the actual file change.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/locale/ -run TestBundle_TagKeys -v`
Expected: FAIL — keys missing from en-US bundle.

- [ ] **Step 3: Add the locale entries**

Append to `cmd/gomddoc/assets/locales/en-US.yml`:

```yaml
tags_title: Tags
tags_index_title: All tags
tags_tagged_as: "Pages tagged %s"
tags_empty: "No pages tagged %s"
```

(The `tags_title` key is used as the chip-list aria-label; `tags_index_title` is the heading
on `/tags/`; `tags_tagged_as` is the heading on `/tags/{tag}`; `tags_empty` is shown when a
tag exists but has no live pages — and as the sole content of `/tags/` when there are no tags.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/locale/ -run TestBundle_TagKeys -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/gomddoc/assets/locales/en-US.yml internal/locale/bundle_test.go
git commit -m "Add en-US locale strings for tag pages and chips"
```

---

## Task 4: tag-chips partial + layout integration

**Files:**
- Create: `cmd/gomddoc/assets/themes/default/partials/tag-chips.html.tmpl`
- Modify: `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl`
- Modify: `internal/template/renderer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/template/renderer_test.go`:

```go
func TestRender_TagChips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tags        []any
		featureOff  bool
		wantContain []string
		wantNot     []string
	}{
		{
			name:        "renders chips for tags",
			tags:        []any{"go", "docs"},
			wantContain: []string{`href="/tags/go"`, `href="/tags/docs"`, `>go<`, `>docs<`, `class="tag-chips"`},
		},
		{
			name:    "no chips when feature off",
			tags:    []any{"go"},
			featureOff:  true,
			wantNot: []string{"tag-chips"},
		},
		{
			name:    "no chips when no tags",
			tags:    nil,
			wantNot: []string{"tag-chips"},
		},
		{
			name:        "non-default language scopes URL",
			tags:        []any{"go"},
			wantContain: []string{`href="/fr/tags/go"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
					`<!doctype html><html><body>{{ template "tag-chips" . }}</body></html>`,
				)},
				"assets/themes/default/partials/tag-chips.html.tmpl": {Data: tagChipsPartialBytes(t)},
			}
			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			page := PageContext{Path: "/x.md", Meta: map[string]any{"tags": tt.tags}}
			if tt.featureOff {
				page.Features = map[string]bool{"tag_chips": false}
			}
			tc := &TemplateContext{Site: &siteConfig, Page: page}
			if tt.name == "non-default language scopes URL" {
				tc.WithI18n("fr", nil, nil)
			}

			out, err := r.Render(context.Background(), "default.html.tmpl", tc)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			s := string(out)
			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("output missing %q\noutput: %s", want, s)
				}
			}
			for _, banned := range tt.wantNot {
				if strings.Contains(s, banned) {
					t.Errorf("output should not contain %q\noutput: %s", banned, s)
				}
			}
		})
	}
}

// tagChipsPartialBytes loads the real partial from the embedded theme.
// Failing if it doesn't exist forces Step 3 of this task to create it.
func tagChipsPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tag-chips.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}
```

If `os` is not imported, add it. Adjust the path relative to the test file's location (`internal/template/`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/ -run TestRender_TagChips -v`
Expected: FAIL — partial file does not exist.

- [ ] **Step 3: Create the partial**

Create `cmd/gomddoc/assets/themes/default/partials/tag-chips.html.tmpl`:

```gotmpl
{{- if .Feature "tag_chips" -}}
{{- $tags := pageTags .Page.Meta -}}
{{- if $tags -}}
<ul class="tag-chips" aria-label="{{ .T "tags_title" }}">
  {{- range $tag := $tags }}
  <li><a class="tag-chip" href="{{ tagURL $.Lang $tag }}">{{ $tag }}</a></li>
  {{- end }}
</ul>
<style>
.tag-chips { list-style: none; padding: 0; margin: 0.5rem 0 1.5rem; display: flex; flex-wrap: wrap; gap: 0.5rem; }
.tag-chip {
  display: inline-block;
  padding: 0.15rem 0.6rem;
  border-radius: 999px;
  font-size: 0.875rem;
  background: var(--color-bg-subtle, #f0f0f0);
  color: var(--color-text, #222);
  text-decoration: none;
  border: 1px solid var(--color-border, transparent);
}
.tag-chip:hover { background: var(--color-primary, #06f); color: var(--color-bg, #fff); }
</style>
{{- end -}}
{{- end -}}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/template/ -run TestRender_TagChips -race -v`
Expected: PASS.

- [ ] **Step 5: Wire the partial into the default layout**

Open `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl`. Locate the spot where `{{ .Page.Content }}` is rendered (the main article body). Immediately above the content block — **after** any title/breadcrumbs and **before** the markdown content — insert:

```gotmpl
{{ template "tag-chips" . }}
```

Verify by running:

Run: `go run ./cmd/gomddoc preview testsite`
Manually visit a tagged page and confirm chips render. Stop the preview server.

- [ ] **Step 6: Run the full template package**

Run: `go test ./internal/template/ -race`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/gomddoc/assets/themes/default/partials/tag-chips.html.tmpl \
        cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl \
        internal/template/renderer_test.go
git commit -m "Render tag chips below H1 (default theme)

New partial tag-chips.html.tmpl renders frontmatter tags as a
horizontal row of pill links scoped to the active language. Gated
by the tag_chips feature flag (default on, per-page overridable).
Inline CSS uses the existing theme variables so all 8 themes will
inherit the styling once they include the partial."
```

---

## Task 5: tags-list partial + RenderTagPage

**Files:**
- Create: `cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl`
- Modify: `internal/template/renderer.go` (add `RenderTagPage`)
- Modify: `internal/template/renderer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/template/renderer_test.go`:

```go
func TestRenderTagPage(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body><main>{{ .Page.Content }}</main></body></html>`,
		)},
		"assets/themes/default/partials/tags-list.html.tmpl": {Data: tagsListPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"
	r := NewHTMLRenderer(&siteConfig, testFS)

	pages := []metadata.PageInfo{
		{Path: "/guide.md", Title: "Guide", Description: "Setup walkthrough"},
		{Path: "/api.md", Title: "API"},
	}
	tFunc := func(k string) string {
		return map[string]string{
			"tags_tagged_as": "Pages tagged %s",
			"tags_empty":     "No pages tagged %s",
		}[k]
	}

	out, err := r.RenderTagPage(context.Background(), "" /* default lang */, tFunc, "go", pages)
	if err != nil {
		t.Fatalf("RenderTagPage: %v", err)
	}

	s := string(out)
	for _, want := range []string{
		`href="/guide.md"`, ">Guide<",
		`href="/api.md"`, ">API<",
		"Setup walkthrough",
		"Pages tagged go", // from tags_tagged_as locale
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\noutput: %s", want, s)
		}
	}
}

func TestRenderTagPage_Empty(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ .Page.Content }}</body></html>`,
		)},
		"assets/themes/default/partials/tags-list.html.tmpl": {Data: tagsListPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tFunc := func(k string) string { return map[string]string{"tags_empty": "No pages tagged %s"}[k] }
	out, err := r.RenderTagPage(context.Background(), "", tFunc, "missing", nil)
	if err != nil {
		t.Fatalf("RenderTagPage empty: %v", err)
	}
	if !strings.Contains(string(out), "No pages tagged missing") {
		t.Errorf("expected empty-state string, got %s", out)
	}
}

func tagsListPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}
```

Add `metadata` import if not present (`"github.com/monolithiclab/gomddoc/internal/metadata"`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/ -run TestRenderTagPage -v`
Expected: FAIL — partial does not exist; method does not exist.

- [ ] **Step 3: Create the partial**

Create `cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl`:

```gotmpl
{{- /* Renders the body of /tags/{tag} given .Tag (string) and .Pages ([]metadata.PageInfo) */ -}}
<header class="tag-page-header">
  <h1>{{ printf (.T "tags_tagged_as") .Tag }}</h1>
</header>
{{- if .Pages }}
<ul class="tag-page-results">
  {{- range $page := .Pages }}
  <li class="tag-result">
    <a class="tag-result-title" href="{{ $page.Path }}">{{ $page.Title }}</a>
    {{- if $page.Description }}
    <p class="tag-result-description">{{ $page.Description }}</p>
    {{- end }}
  </li>
  {{- end }}
</ul>
{{- else }}
<p class="tag-page-empty">{{ printf (.T "tags_empty") .Tag }}</p>
{{- end }}
<style>
.tag-page-results { list-style: none; padding: 0; }
.tag-result { padding: 0.75rem 0; border-bottom: 1px solid var(--color-border, #eee); }
.tag-result-title { font-weight: 600; font-size: 1.1rem; }
.tag-result-description { color: var(--color-text-muted, #666); margin: 0.25rem 0 0; }
</style>
```

- [ ] **Step 4: Add the render method and partial-execution helper**

In `internal/template/renderer.go`, add (place after `Render`):

```go
// RenderTagPage renders the body of a /tags/{tag} page through the standard
// theme layout. lang is the active BCP-47 code ("" for default language).
// tFunc is the translator scoped to lang (callers typically pass
// bundle.TFunc(lang)). Pages should already be sorted by the caller.
func (h *HTMLRenderer) RenderTagPage(ctx context.Context, lang string, tFunc func(string) string, tag string, pages []metadata.PageInfo) ([]byte, error) {
	if tFunc == nil {
		tFunc = func(k string) string { return k }
	}
	body, err := h.executePartial("tags-list", tagPageData{
		Tag:   tag,
		Pages: pages,
		Lang:  lang,
		T:     tFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("render tags-list: %w", err)
	}

	page := PageContext{
		Path:    tagURL(lang, tag),
		Title:   tag,
		Content: template.HTML(body), // #nosec G203 -- partial output is trusted
		Meta:    map[string]any{"title": tag},
	}
	tc := &TemplateContext{Site: h.siteConfig, Page: page}
	tc.WithI18n(lang, tFunc, nil)
	return h.Render(ctx, "default.html.tmpl", tc)
}

// tagPageData is the data passed to the tags-list partial.
type tagPageData struct {
	Tag   string
	Pages []metadata.PageInfo
	Lang  string
	T     func(string) string
}
```

If `executePartial` does not already exist on `HTMLRenderer`, add it next to the new method (it's reused by `RenderTagsIndex` in the next task):

```go
// executePartial runs a single named partial against data and returns the
// rendered bytes. Used for server-side composition of synthetic pages
// (tag listings, tag index) where the body is pre-built then passed
// through the standard layout via Page.Content.
func (h *HTMLRenderer) executePartial(name string, data any) ([]byte, error) {
	partialPath := path.Join("assets", "themes", h.siteConfig.Theme.Name, "partials", name+".html.tmpl")
	tmpl, err := template.New(name+".html.tmpl").Funcs(h.funcMap()).ParseFS(h.assetsFS, partialPath)
	if err != nil {
		// Fall back to default theme partial.
		fallback := path.Join("assets", "themes", config.DefaultThemeName, "partials", name+".html.tmpl")
		tmpl, err = template.New(name+".html.tmpl").Funcs(h.funcMap()).ParseFS(h.assetsFS, fallback)
		if err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

Add imports: `"bytes"`, `"path"`, `"github.com/monolithiclab/gomddoc/internal/metadata"` (none of these need to be added if already imported).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/template/ -run TestRenderTagPage -race -v`
Expected: PASS.

- [ ] **Step 6: Run the full template package**

Run: `go test ./internal/template/ -race`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl \
        internal/template/renderer.go internal/template/renderer_test.go
git commit -m "Add RenderTagPage and tags-list partial

RenderTagPage executes the new tags-list partial against {Tag,
Pages, Lang} data, then runs the result through the standard
default.html.tmpl layout via Page.Content. Empty list renders the
localized empty state. New executePartial helper supports
single-partial rendering for synthetic pages (reused by
RenderTagsIndex in the next task)."
```

---

## Task 6: tags-index partial + RenderTagsIndex

**Files:**
- Create: `cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl`
- Modify: `internal/template/renderer.go`
- Modify: `internal/template/renderer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/template/renderer_test.go`:

```go
func TestRenderTagsIndex(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ .Page.Content }}</body></html>`,
		)},
		"assets/themes/default/partials/tags-index.html.tmpl": {Data: tagsIndexPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tags := []TagCount{
		{Tag: "go", Count: 4},
		{Tag: "kubernetes", Count: 1},
		{Tag: "tutorial", Count: 7},
	}

	tFunc := func(k string) string { return map[string]string{"tags_index_title": "All tags"}[k] }
	out, err := r.RenderTagsIndex(context.Background(), "", tFunc, tags)
	if err != nil {
		t.Fatalf("RenderTagsIndex: %v", err)
	}

	s := string(out)
	for _, want := range []string{
		"All tags",
		`href="/tags/go"`, ">go<", "(4)",
		`href="/tags/kubernetes"`, "(1)",
		`href="/tags/tutorial"`, "(7)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\noutput: %s", want, s)
		}
	}
}

func TestRenderTagsIndex_Empty(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ .Page.Content }}</body></html>`,
		)},
		"assets/themes/default/partials/tags-index.html.tmpl": {Data: tagsIndexPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tFunc := func(k string) string { return map[string]string{"tags_index_title": "All tags"}[k] }
	out, err := r.RenderTagsIndex(context.Background(), "", tFunc, nil)
	if err != nil {
		t.Fatalf("RenderTagsIndex empty: %v", err)
	}
	if !strings.Contains(string(out), "All tags") {
		t.Errorf("expected header in empty state, got %s", out)
	}
}

func tagsIndexPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/ -run TestRenderTagsIndex -v`
Expected: FAIL — partial missing, method missing, `TagCount` type missing.

- [ ] **Step 3: Add the TagCount type and method**

In `internal/template/renderer.go` add:

```go
// TagCount is one entry in the tag index.
type TagCount struct {
	Tag   string
	Count int
}

// RenderTagsIndex renders the body of /tags/ (the index of all tags) through
// the standard theme layout. tags should already be sorted alphabetically.
func (h *HTMLRenderer) RenderTagsIndex(ctx context.Context, lang string, tFunc func(string) string, tags []TagCount) ([]byte, error) {
	if tFunc == nil {
		tFunc = func(k string) string { return k }
	}
	body, err := h.executePartial("tags-index", tagsIndexData{
		Tags: tags,
		Lang: lang,
		T:    tFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("render tags-index: %w", err)
	}

	pagePath := "/tags/"
	if lang != "" {
		pagePath = "/" + lang + "/tags/"
	}
	page := PageContext{
		Path:    pagePath,
		Title:   tFunc("tags_index_title"),
		Content: template.HTML(body), // #nosec G203 -- partial output is trusted
	}
	tc := &TemplateContext{Site: h.siteConfig, Page: page}
	tc.WithI18n(lang, tFunc, nil)
	return h.Render(ctx, "default.html.tmpl", tc)
}

type tagsIndexData struct {
	Tags []TagCount
	Lang string
	T    func(string) string
}
```

- [ ] **Step 4: Create the partial**

Create `cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl`:

```gotmpl
{{- /* Renders the body of /tags/ given .Tags ([]TagCount) */ -}}
<header class="tag-index-header">
  <h1>{{ .T "tags_index_title" }}</h1>
</header>
<ul class="tag-index">
  {{- range $entry := .Tags }}
  <li class="tag-index-row">
    <a class="tag-index-link" href="{{ tagURL $.Lang $entry.Tag }}">{{ $entry.Tag }}</a>
    <span class="tag-index-count">({{ $entry.Count }})</span>
  </li>
  {{- end }}
</ul>
<style>
.tag-index { list-style: none; padding: 0; columns: 2; }
.tag-index-row { padding: 0.25rem 0; break-inside: avoid; }
.tag-index-count { color: var(--color-text-muted, #666); margin-left: 0.25rem; }
@media (max-width: 600px) { .tag-index { columns: 1; } }
</style>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/template/ -run TestRenderTagsIndex -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl \
        internal/template/renderer.go internal/template/renderer_test.go
git commit -m "Add RenderTagsIndex and tags-index partial

Renders /tags/ as a two-column alphabetical list of tags with
counts, each linking to its per-tag page. Reuses executePartial
from the previous task and follows the same Page.Content
composition pattern."
```

---

## Task 7: TagPageHandler

**Files:**
- Create: `internal/server/tags_html.go`
- Create: `internal/server/tags_html_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/tags_html_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
)

func newTagPageHandler(t *testing.T) (*TagPageHandler, *metadata.Index) {
	t.Helper()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A page\ntags: [go, docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B page\ntags: [docs]\n---\n# B")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":      {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-list.html.tmpl":   {Data: []byte(`<h1>Pages tagged {{ .Tag }}</h1><ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>`)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k } // identity for tests

	return NewTagPageHandler(idx, r, tFunc, "" /* default lang */), idx
}

func TestTagPageHandler_Found(t *testing.T) {
	t.Parallel()

	h, _ := newTagPageHandler(t)

	req := httptest.NewRequest("GET", "/tags/docs", nil)
	req.SetPathValue("tag", "docs")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "A page") || !strings.Contains(body, "B page") {
		t.Errorf("body missing both pages\n%s", body)
	}
	if !strings.Contains(body, "Pages tagged docs") {
		t.Errorf("body missing title\n%s", body)
	}
}

func TestTagPageHandler_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newTagPageHandler(t)

	req := httptest.NewRequest("GET", "/tags/missing", nil)
	req.SetPathValue("tag", "missing")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTagPageHandler_DecodesPathValue(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: ML Page\ntags: [\"machine learning\"]\n---\n# A")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":      {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-list.html.tmpl":   {Data: []byte(`<ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>`)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }
	h := NewTagPageHandler(idx, r, tFunc, "")

	req := httptest.NewRequest("GET", "/tags/machine%20learning", nil)
	req.SetPathValue("tag", "machine learning") // ServeMux URL-decodes path values
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ML Page") {
		t.Errorf("expected ML Page, got %s", w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestTagPageHandler -v`
Expected: FAIL — `TagPageHandler` not defined.

- [ ] **Step 3: Implement the handler**

Create `internal/server/tags_html.go`:

```go
package server

import (
	"log/slog"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
)

// TagPageHandler serves the HTML page for /tags/{tag} (or /{lang}/tags/{tag}).
type TagPageHandler struct {
	index    *metadata.Index
	renderer *template.HTMLRenderer
	tFunc    func(string) string
	lang     string // "" for default language
}

// NewTagPageHandler binds an index, renderer, and translator to a language scope.
func NewTagPageHandler(index *metadata.Index, renderer *template.HTMLRenderer, tFunc func(string) string, lang string) *TagPageHandler {
	return &TagPageHandler{index: index, renderer: renderer, tFunc: tFunc, lang: lang}
}

func (h *TagPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" {
		http.NotFound(w, r)
		return
	}

	pages := h.index.ByTag(tag)
	if len(pages) == 0 {
		http.NotFound(w, r)
		return
	}

	// Sort alphabetically by title for deterministic output.
	slices.SortFunc(pages, func(a, b metadata.PageInfo) int {
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	body, err := h.renderer.RenderTagPage(r.Context(), h.lang, h.tFunc, tag, pages)
	if err != nil {
		slog.Error("render tag page", slog.String("tag", tag), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	serveWithETag(w, r, body, "text/html; charset=utf-8", "public, max-age=300")
}
```

Add imports `"slices"` and `"strings"` if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestTagPageHandler -race -v`
Expected: PASS (all three subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/server/tags_html.go internal/server/tags_html_test.go
git commit -m "Add TagPageHandler for /tags/{tag}

Looks up pages by tag in the language-scoped MetaIndex, 404s when
the tag has no matches, otherwise renders through the template
renderer's RenderTagPage. ETag + Cache-Control: max-age=300 match
content pages."
```

---

## Task 8: TagsIndexHandler

**Files:**
- Modify: `internal/server/tags_html.go`
- Modify: `internal/server/tags_html_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/server/tags_html_test.go`:

```go
func TestTagsIndexHandler(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ntags: [go, docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B\ntags: [docs]\n---\n# B")},
		"c.md": {Data: []byte("---\ntitle: C\ntags: [tutorial]\n---\n# C")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":       {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-index.html.tmpl":   {Data: []byte(`<ul>{{ range .Tags }}<li>{{ .Tag }}({{ .Count }})</li>{{ end }}</ul>`)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }
	h := NewTagsIndexHandler(idx, r, tFunc, "")

	req := httptest.NewRequest("GET", "/tags/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"docs(2)", "go(1)", "tutorial(1)"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in %s", want, body)
		}
	}
	// Confirm alphabetical: docs < go < tutorial.
	if i, j, k := strings.Index(body, "docs"), strings.Index(body, "go"), strings.Index(body, "tutorial"); !(i < j && j < k) {
		t.Errorf("not alphabetical: docs=%d go=%d tutorial=%d", i, j, k)
	}
}

func TestTagsIndexHandler_Empty(t *testing.T) {
	t.Parallel()

	idx, err := metadata.BuildIndex(t.Context(), fstest.MapFS{}, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":       {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-index.html.tmpl":   {Data: []byte(`<p>tags index ({{ len .Tags }} tags)</p>`)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }
	h := NewTagsIndexHandler(idx, r, tFunc, "")

	req := httptest.NewRequest("GET", "/tags/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty index should still 200, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestTagsIndexHandler -v`
Expected: FAIL — `TagsIndexHandler` not defined.

- [ ] **Step 3: Implement the handler**

Append to `internal/server/tags_html.go`:

```go
// TagsIndexHandler serves the HTML page for /tags/ (or /{lang}/tags/).
type TagsIndexHandler struct {
	index    *metadata.Index
	renderer *template.HTMLRenderer
	tFunc    func(string) string
	lang     string
}

// NewTagsIndexHandler binds an index, renderer, and translator to a language scope.
func NewTagsIndexHandler(index *metadata.Index, renderer *template.HTMLRenderer, tFunc func(string) string, lang string) *TagsIndexHandler {
	return &TagsIndexHandler{index: index, renderer: renderer, tFunc: tFunc, lang: lang}
}

func (h *TagsIndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tags := h.index.AllTags() // already alphabetical
	entries := make([]template.TagCount, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, template.TagCount{Tag: tag, Count: len(h.index.ByTag(tag))})
	}
	body, err := h.renderer.RenderTagsIndex(r.Context(), h.lang, h.tFunc, entries)
	if err != nil {
		slog.Error("render tags index", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	serveWithETag(w, r, body, "text/html; charset=utf-8", "public, max-age=300")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestTagsIndexHandler -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/tags_html.go internal/server/tags_html_test.go
git commit -m "Add TagsIndexHandler for /tags/

Builds []TagCount from MetaIndex.AllTags() (already alphabetical)
and renders through RenderTagsIndex. Always returns 200 -- empty
state is handled by the partial."
```

---

## Task 9: Per-language route registration

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go` (or relevant existing test)

- [ ] **Step 1: Locate the existing per-language registration block**

Open `internal/server/server.go` and find where `/sitemap.xml` is registered for default and per-lang pipelines. The new tag routes go in the same block(s).

- [ ] **Step 2: Write the failing test**

Add to `internal/server/server_test.go`:

```go
func TestServer_TagRoutes_DefaultLang(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ntags: [docs]\n---\n# A")},
	}
	srv := newTestServerWithContent(t, files) // existing test helper or assemble inline

	tests := []struct {
		path string
		want int
	}{
		{"/tags/docs", http.StatusOK},
		{"/tags/missing", http.StatusNotFound},
		{"/tags/", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("%s: status = %d, want %d", tt.path, w.Code, tt.want)
			}
		})
	}
}
```

If `newTestServerWithContent` does not exist, mirror the construction pattern used in existing handler tests (`internal/server/handler_test.go`), wiring a real `HTTPServer` against `fstest.MapFS`.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestServer_TagRoutes -v`
Expected: FAIL — routes not registered (404 on /tags/docs, etc.).

- [ ] **Step 4: Register the routes**

In `internal/server/server.go`, find the loop that registers per-language sitemap/feed handlers (look for `for lang, langCfg := range opts.LangPipelines`). Inside that same loop add:

```go
langTFunc := opts.LocaleBundle.TFunc(lang)
tagPage := NewTagPageHandler(langCfg.MetaIndex, opts.TemplateRenderer, langTFunc, lang)
tagsIndex := NewTagsIndexHandler(langCfg.MetaIndex, opts.TemplateRenderer, langTFunc, lang)
mux.Handle("GET /"+lang+"/tags/{tag}", tagPage)
mux.Handle("GET /"+lang+"/tags/", tagsIndex)
```

For the default-language registration block (immediately above or below), add:

```go
defaultTFunc := opts.LocaleBundle.TFunc(opts.DefaultLang)
defaultTagPage := NewTagPageHandler(opts.MetaIndex, opts.TemplateRenderer, defaultTFunc, "")
defaultTagsIndex := NewTagsIndexHandler(opts.MetaIndex, opts.TemplateRenderer, defaultTFunc, "")
mux.Handle("GET /tags/{tag}", defaultTagPage)
mux.Handle("GET /tags/", defaultTagsIndex)
```

`opts.LocaleBundle` and `opts.DefaultLang` already exist on `HTTPServerConfig` (they back the
existing per-language sitemap/feed handlers). `opts.MetaIndex` and `opts.TemplateRenderer` likewise.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestServer_TagRoutes -race -v`
Expected: PASS.

- [ ] **Step 6: Run full server package**

Run: `go test ./internal/server/ -race`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "Register tag-page routes per language

Mounts /tags/{tag} and /tags/ on the default language pipeline,
plus /{lang}/tags/{tag} and /{lang}/tags/ on each non-default
language pipeline. Mirrors the existing per-lang sitemap/feed
registration pattern."
```

---

## Task 10: Sitemap inclusion of tag URLs

**Files:**
- Modify: `internal/server/sitemap.go`
- Modify: `internal/server/sitemap_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/sitemap_test.go`:

```go
func TestGenerateSitemap_IncludesTagPages(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ntags: [go, docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B\ntags: [docs]\n---\n# B")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	prov := &fakeProvider{root: files}
	resolver := resolve.Build(files, []string{".md"}, func(string) bool { return true })

	xml, err := GenerateSitemap(t.Context(), idx, "https://example.com", "README.md", prov, resolver, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(xml)
	for _, want := range []string{
		"https://example.com/tags/",
		"https://example.com/tags/go",
		"https://example.com/tags/docs",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q\n%s", want, body)
		}
	}
}
```

If `fakeProvider` doesn't already exist in this package's tests, locate the existing sitemap-test fixture pattern and reuse it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestGenerateSitemap_IncludesTagPages -v`
Expected: FAIL — tag URLs not in generated sitemap.

- [ ] **Step 3: Append tag URLs to the sitemap**

In `internal/server/sitemap.go`, find where page URLs are emitted (look for the loop iterating `idx.AllPages()`). After that loop and before the closing `</urlset>` write, add:

```go
// Append the tag index plus one URL per tag.
indexURL := seo.PageURL(domain, pathPrefix+"/tags/", "")
fmt.Fprintf(&buf, `  <url><loc>%s</loc></url>`+"\n", html.EscapeString(indexURL))
for _, tag := range idx.AllTags() {
	tagURL := seo.PageURL(domain, pathPrefix+"/tags/"+url.PathEscape(tag), "")
	fmt.Fprintf(&buf, `  <url><loc>%s</loc></url>`+"\n", html.EscapeString(tagURL))
}
```

Inspect existing sitemap-emission code to match its exact formatting (indentation, lastmod usage, xml.escape vs html.EscapeString). If the file uses an `xml.Encoder` instead of string formatting, switch to that style. Add `"net/url"` import if missing.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestGenerateSitemap -race -v`
Expected: PASS (existing sitemap tests still PASS, new test PASS).

- [ ] **Step 5: Commit**

```bash
git add internal/server/sitemap.go internal/server/sitemap_test.go
git commit -m "Include tag pages in generated sitemap

Appends the tag index plus one URL per unique tag (per language)
to the sitemap so search engines can crawl the new aggregation
pages. Tags are PathEscape'd for URL safety."
```

---

## Task 11: Build mode emission

**Files:**
- Modify: `cmd/gomddoc/build.go`
- Modify: `cmd/gomddoc/build_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/gomddoc/build_test.go`:

```go
func TestBuildCmd_EmitsTagPages(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contentDir, "a.md"),
		[]byte("---\ntitle: A\ntags: [go, machine learning]\n---\n# A"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "b.md"),
		[]byte("---\ntitle: B\ntags: [go]\n---\n# B"), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	b := &BuildCmd{Dir: contentDir, Output: outDir}
	if err := b.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, want := range []string{
		"tags/index.html",
		"tags/go/index.html",
		"tags/machine%20learning/index.html",
	} {
		if _, err := os.Stat(filepath.Join(outDir, want)); err != nil {
			t.Errorf("expected output %s: %v", want, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/gomddoc/ -run TestBuildCmd_EmitsTagPages -v`
Expected: FAIL — files not generated.

- [ ] **Step 3: Add the build-mode emission**

In `cmd/gomddoc/build.go`, find where `walkAndBuildToDir` finishes for the default language (around the section that calls `generateSEOFiles`). Add a new method invocation:

```go
if err := b.emitTagPages(ctx, pipeline, "", bc.tFunc); err != nil {
	return fmt.Errorf("emit tag pages: %w", err)
}
```

In the per-language loop (look for `for lang, langPipe := range lp.ByLang`), add:

```go
if err := b.emitTagPages(ctx, langPipe, lang, bundle.TFunc(lang)); err != nil {
	return fmt.Errorf("emit tag pages for %s: %w", lang, err)
}
```

Add the helper method on `BuildCmd`:

```go
// emitTagPages writes /tags/index.html plus one /tags/{escaped}/index.html per
// unique tag for the given pipeline. Empty lang produces unprefixed paths;
// non-empty lang prefixes with /{lang}.
func (b *BuildCmd) emitTagPages(ctx context.Context, p *Pipeline, lang string, tFunc func(string) string) error {
	if p.MetaIndex == nil {
		return nil
	}
	prefix := ""
	if lang != "" {
		prefix = lang + "/"
	}

	// Index page.
	tags := p.MetaIndex.AllTags()
	entries := make([]tmpl.TagCount, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, tmpl.TagCount{Tag: tag, Count: len(p.MetaIndex.ByTag(tag))})
	}
	body, err := p.TemplateRenderer.RenderTagsIndex(ctx, lang, tFunc, entries)
	if err != nil {
		return err
	}
	if err := b.writeOutputFile(prefix+"tags/index.html", body); err != nil {
		return err
	}

	// Per-tag pages.
	for _, tag := range tags {
		pages := p.MetaIndex.ByTag(tag)
		slices.SortFunc(pages, func(a, b metadata.PageInfo) int {
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		})
		body, err := p.TemplateRenderer.RenderTagPage(ctx, lang, tFunc, tag, pages)
		if err != nil {
			return fmt.Errorf("render tag %q: %w", tag, err)
		}
		out := prefix + "tags/" + url.PathEscape(tag) + "/index.html"
		if err := b.writeOutputFile(out, body); err != nil {
			return err
		}
	}
	return nil
}
```

Add `"net/url"`, `"slices"`, `"strings"`, `"github.com/monolithiclab/gomddoc/internal/metadata"` to the import block if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/gomddoc/ -run TestBuildCmd_EmitsTagPages -race -v`
Expected: PASS.

- [ ] **Step 5: Run the full cmd/gomddoc package**

Run: `go test ./cmd/gomddoc/ -race`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/build.go cmd/gomddoc/build_test.go
git commit -m "Emit /tags/ index and per-tag pages in build mode

For each language pipeline, write tags/index.html plus one
tags/{escaped-tag}/index.html per unique tag. Reuses RenderTagsIndex
and RenderTagPage so serve and build produce identical output."
```

---

## Task 12: Content-collision startup warning

**Files:**
- Modify: `cmd/gomddoc/serve.go` (or wherever the content provider is constructed)
- Modify: `cmd/gomddoc/serve_test.go` (or build_test.go for the build-mode equivalent)

- [ ] **Step 1: Write the failing test**

Add to `cmd/gomddoc/serve_test.go`:

```go
func TestSetupServer_WarnsOnTagsContentCollision(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contentDir, "tags.md"), []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}

	logs := captureLogs(t) // existing test helper or implement inline

	cfg := &config.Config{Server: config.ServerConfig{Dir: contentDir, Port: "0"}, Site: config.NewSiteConfig(contentDir)}
	_, err := setupServer(cfg)
	if err != nil {
		t.Fatalf("setupServer: %v", err)
	}

	if !strings.Contains(logs.String(), "tags.md") {
		t.Errorf("expected warning about tags.md, got: %s", logs.String())
	}
}
```

If `captureLogs` is not available, write a minimal inline helper that swaps `slog.Default()` for a buffered handler.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/gomddoc/ -run TestSetupServer_WarnsOnTagsContentCollision -v`
Expected: FAIL — no warning emitted.

- [ ] **Step 3: Implement the warning**

In `cmd/gomddoc/serve.go` (or wherever the language pipelines are assembled), after the metadata index is built but before the server starts, add:

```go
warnTagsContentCollision(contentRoot, "" /* lang */)
for lang, langCfg := range langPipelines {
	warnTagsContentCollision(langCfg.contentRoot, lang)
}
```

And the helper (in the same file, or a new `internal/server/collision.go` if exporting makes more sense):

```go
// warnTagsContentCollision logs a warning when the user has authored content
// at paths that collide with the auto-generated tag pages (/tags/, /tags/{tag}).
func warnTagsContentCollision(contentRoot fs.FS, lang string) {
	for _, candidate := range []string{"tags.md", "tags"} {
		info, err := fs.Stat(contentRoot, candidate)
		if err != nil {
			continue
		}
		kind := "file"
		if info.IsDir() {
			kind = "directory"
		}
		slog.Warn("Content path collides with auto-generated tag pages",
			slog.String("path", candidate),
			slog.String("kind", kind),
			slog.String("lang", lang),
			slog.String("hint", "rename to avoid being shadowed by /tags routes"))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/gomddoc/ -run TestSetupServer_WarnsOnTagsContentCollision -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/gomddoc/serve.go cmd/gomddoc/serve_test.go
git commit -m "Warn at startup when content collides with /tags routes

Logs a non-fatal warning if the content root contains tags.md or
a tags/ directory (per language). The auto-generated tag routes
shadow user content at those paths -- the warning advises a
rename."
```

---

## Task 13: Update roadmap and decisions

**Files:**
- Modify: `docs/roadmap.md`
- Modify: `docs/decisions.md`

- [ ] **Step 1: Mark roadmap items done**

Open `docs/roadmap.md` → "Tag Components" section. Change each `- [ ]` to `- [x]` for the three items shipped here:

- Clickable tag chips in page rendering
- Tag listing page (`/tags/{tag}`)
- Tag index page (`/tags/`)

Leave the remaining two items (`tag:` search syntax and Related pages via tags) as `- [ ]`.

- [ ] **Step 2: Add a decisions entry**

Append to `docs/decisions.md`:

```markdown
## Tag URL Format & Pages

**Chosen**: percent-encoded tag values in URLs (`/tags/machine%20learning`), centralized validator skipping invalid tags at index-build time

**Alternatives considered**:
- **Slugified URLs (kebab-case)**: shorter and prettier, but introduces a slug→tag reverse map and invents collisions where none exist in the source data. Rejected.
- **Single global `/tags/` (no per-lang)**: simpler routing but mixes languages and breaks the per-lang navigation model. Rejected — per-lang routing matches the existing sitemap/feed pattern.

**Why percent-encoding**: tag values already exist in frontmatter as plain strings; encoding rather than slugifying preserves them exactly. Pathological tag values containing `/` or whitespace-only entries are skipped at `metadata.Index` build time with a logged warning, so the encoding stays simple and the bad-data surface is closed at the source.

**Why no related-pages or `tag:` search**: deferred to follow-up sub-specs to keep this PR focused on the user-visible discovery features (chips + landing pages).
```

- [ ] **Step 3: Verify the build still passes**

Run: `make ci`
Expected: all PASS, coverage ≥ 86%.

- [ ] **Step 4: Commit**

```bash
git add docs/roadmap.md docs/decisions.md
git commit -m "Mark tag-component sub-items done; log decision

Roadmap: chips, listing pages, and tag index marked complete.
Decisions log captures the percent-encoding vs slug choice and
the rationale for per-language pages."
```

---

## Coordinated change (separate repo, out of this plan)

After landing all 13 tasks above, open a coordinated PR in `gomddoc-themes` adding the same `{{ template "tag-chips" . }}` line to each of the 7 themes' main layouts at the same position (below H1). The shared partial uses CSS custom properties that all themes already define, so no per-theme styling is needed.

A separate `gomddoc-website` PR should follow, documenting:
- The new `tag_chips` feature flag in the configuration reference.
- The tag-chips, tags-list, and tags-index partials in the theming guide.
- A short "Tags" section in the user guide explaining how chips and the tag pages work.
