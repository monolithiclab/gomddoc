# See Also Section: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render a "See also" section at the bottom of every page that has at least one related page (linked by shared frontmatter tags).

**Architecture:** Template-only feature. The enricher already populates `EnrichmentData.RelatedDocs`. Add a `RelatedDocs` field to `PageContext`, wire it from both render paths (server `serveHTML` + build `buildFile`) with handler-side alphabetical sort, render via a new `see-also` partial gated by a `see_also` feature flag.

**Tech Stack:** Go 1.25+, `html/template`, `slices.SortFunc` for sort, existing `internal/locale` for i18n.

**Spec:** `docs/specs/2026-04-24-see-also-section.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/template/renderer.go` | Modify | Add `RelatedDocs []enricher.RelatedDoc` field to `PageContext` |
| `internal/server/handler.go` | Modify | In `serveHTML`, sort `enrichment.RelatedDocs` alphabetically by title (case-insensitive) and assign to `PageContext.RelatedDocs` |
| `cmd/gomddoc/build.go` | Modify | In the markdown render path (around line 446), do the same sort + assignment |
| `cmd/gomddoc/assets/locales/en-US.yml` | Modify | Add `see_also: "See also"` |
| `cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl` | Create | Renders the section, gated by `see_also` feature flag |
| `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl` | Modify | Insert `{{ template "see-also" . }}` after `{{ .Page.Content }}`, before `{{ template "footer" . }}` |
| `internal/template/renderer_test.go` | Modify | New `TestRender_SeeAlso` covering present/empty/feature-off/sort cases |
| `internal/server/handler_test.go` | Modify | Integration test asserting the section renders end-to-end |
| `cmd/gomddoc/build_test.go` | Modify | Build-mode test asserting the section appears in generated HTML |
| `docs/roadmap.md` | Modify | Mark "Related pages via tags" done |

---

## Task 1: Add `RelatedDocs` field to `PageContext`

**Files:**
- Modify: `internal/template/renderer.go` (`PageContext` struct around line 109-118)

This task is structural — the field has no behavior on its own, so no failing test for *this task*. The wiring tasks (2 and 3) introduce the failing tests that this struct change unblocks.

- [ ] **Step 1: Add the field**

In `internal/template/renderer.go`, locate `type PageContext struct` (around line 109). Add a new field below `NextPage`:

```go
type PageContext struct {
	Content     template.HTML
	Path        string             // Current request path
	Meta        map[string]any     // Extracted metadata (e.g., front matter)
	Features    map[string]bool    // Pre-merged feature toggles (site defaults + page overrides)
	TOC         *enricher.TOCNode  // Table of Contents
	Navigation  *enricher.NavTree  // Navigation tree (populated by enricher)
	PrevPage    *enricher.PageLink // Previous page in navigation order
	NextPage    *enricher.PageLink // Next page in navigation order
	RelatedDocs []enricher.RelatedDoc // Pages sharing frontmatter tags with this page
}
```

(Keep the existing field alignment style — the existing struct uses one-space alignment. The new field follows the same pattern.)

- [ ] **Step 2: Verify build still compiles**

Run: `go build ./...`
Expected: clean build with no errors. (No callers populate the new field yet; it defaults to nil, which is the same as "no related pages" — partials will treat it as empty.)

- [ ] **Step 3: Commit**

```bash
git add internal/template/renderer.go
git commit -m "Add RelatedDocs field to template PageContext

Threads the enricher's already-computed RelatedDocs into the
template context so the upcoming see-also partial can render it.
No behavior change yet -- handler and build wiring follow."
```

---

## Task 2: Locale string

**Files:**
- Modify: `cmd/gomddoc/assets/locales/en-US.yml`
- Modify: `internal/locale/bundle_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/locale/bundle_test.go` (find the existing `TestLoadBundle_TagKeys` test added in the previous tag-components work and add a sibling test):

```go
func TestLoadBundle_SeeAlsoKey(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"locales/en-US.yml": {Data: enUSBundleBytes(t)},
	}
	bundle, err := locale.LoadBundle("en-US", files, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	got := bundle.T("en-US", "see_also")
	if got == "" || got == "see_also" {
		t.Errorf("bundle.T(\"see_also\") returned %q; expected a localized string", got)
	}
}
```

If the helper `enUSBundleBytes(t)` does not exist, find the equivalent helper used by `TestLoadBundle_TagKeys` and reuse it. If that test loads the YAML inline rather than via a helper, mirror the same inline pattern — read `cmd/gomddoc/assets/locales/en-US.yml` once via `os.ReadFile` and pass the bytes to a `fstest.MapFS`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/locale/ -run TestLoadBundle_SeeAlsoKey -v`
Expected: FAIL — `see_also` key absent from en-US bundle.

- [ ] **Step 3: Add the locale entry**

Append to `cmd/gomddoc/assets/locales/en-US.yml`:

```yaml
see_also: "See also"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/locale/ -run TestLoadBundle_SeeAlsoKey -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/gomddoc/assets/locales/en-US.yml internal/locale/bundle_test.go
git commit -m "Add en-US locale string for see-also section heading"
```

---

## Task 3: `see-also` partial

**Files:**
- Create: `cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl`
- Modify: `internal/template/renderer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/template/renderer_test.go`:

```go
func TestRender_SeeAlso(t *testing.T) {
	t.Parallel()

	related := []enricher.RelatedDoc{
		{Path: "/api.md", Title: "API"},
		{Path: "/guide.md", Title: "Guide"},
	}

	tests := []struct {
		name        string
		related     []enricher.RelatedDoc
		featureOff  bool
		wantContain []string
		wantNot     []string
	}{
		{
			name:        "renders section for non-empty related list",
			related:     related,
			wantContain: []string{`class="see-also"`, `>See also<`, `href="/api.md"`, `href="/guide.md"`, `>API<`, `>Guide<`},
		},
		{
			name:    "no section when feature off",
			related: related,
			featureOff: true,
			wantNot: []string{"see-also"},
		},
		{
			name:    "no section when no related",
			related: nil,
			wantNot: []string{"see-also"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
					`<!doctype html><html><body>{{ template "see-also" . }}</body></html>`,
				)},
				"assets/themes/default/partials/see-also.html.tmpl": {Data: seeAlsoPartialBytes(t)},
			}
			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			page := PageContext{Path: "/x.md", RelatedDocs: tt.related}
			if tt.featureOff {
				page.Features = map[string]bool{"see_also": false}
			}
			tc := &TemplateContext{Site: &siteConfig, Page: page}

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

func seeAlsoPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}
```

If `enricher` is not yet imported in this test file, add `"github.com/monolithiclab/gomddoc/internal/enricher"` to imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/ -run TestRender_SeeAlso -v`
Expected: FAIL — partial file does not exist.

- [ ] **Step 3: Create the partial**

Create `cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl`:

```gotmpl
{{ define "see-also" }}
{{- if .Feature "see_also" -}}
{{- if .Page.RelatedDocs -}}
<section class="see-also" aria-labelledby="see-also-heading">
  <h2 id="see-also-heading">{{ .T "see_also" }}</h2>
  <ul>
  {{- range $doc := .Page.RelatedDocs }}
    <li><a href="{{ $doc.Path }}">{{ $doc.Title }}</a></li>
  {{- end }}
  </ul>
</section>
<style>
.see-also { margin-top: 2.5rem; padding-top: 1.5rem; border-top: 1px solid var(--color-border, #eee); }
.see-also h2 { font-size: 1.1rem; margin: 0 0 0.75rem; }
.see-also ul { list-style: none; padding: 0; margin: 0; }
.see-also li { padding: 0.25rem 0; }
.see-also a { color: var(--color-primary, #06f); }
</style>
{{- end -}}
{{- end -}}
{{ end }}
```

`.Feature`, `.T`, and `.Page.RelatedDocs` resolve through the `TemplateContext` (top-level `.`). The `{{ define "see-also" }}…{{ end }}` wrapper matches the existing pattern used by `tag-chips.html.tmpl`, `tags-list.html.tmpl`, and `tags-index.html.tmpl`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/template/ -run TestRender_SeeAlso -race -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Run the full template package**

Run: `go test ./internal/template/ -race`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl \
        internal/template/renderer_test.go
git commit -m "Add see-also partial for related pages section

Renders <section class=\"see-also\"> with an <h2> and <ul> of links
when the page has related pages. Gated by the see_also feature
flag (default on, per-page overridable). Inline CSS uses the
existing theme variables so all 8 themes inherit consistent
styling once they include the partial."
```

---

## Task 4: Layout integration

**Files:**
- Modify: `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl` (around line 32)

- [ ] **Step 1: Insert the partial call**

Open `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl`. The `<article>` block currently looks like:

```gotmpl
<article>
  {{ template "tag-chips" . }}
  {{ .Page.Content }}

  {{ template "footer" . }}
</article>
```

Insert `{{ template "see-also" . }}` between `{{ .Page.Content }}` and `{{ template "footer" . }}`:

```gotmpl
<article>
  {{ template "tag-chips" . }}
  {{ .Page.Content }}

  {{ template "see-also" . }}

  {{ template "footer" . }}
</article>
```

- [ ] **Step 2: Verify the build still works**

Run: `go build ./...`
Expected: clean build.

Run: `go test ./internal/template/ -race`
Expected: all PASS — confirms the layout edit didn't break any existing test.

- [ ] **Step 3: Commit**

```bash
git add cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl
git commit -m "Wire see-also partial into default layout

Renders the section between content body and footer in the
<article> block, after tag chips."
```

---

## Task 5: Wire `RelatedDocs` from the server handler

**Files:**
- Modify: `internal/server/handler.go` (`serveHTML` around line 175-188)
- Modify: `internal/server/handler_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/handler_test.go`:

```go
func TestServeHTML_IncludesSeeAlsoSection(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md":     {Data: []byte("---\ntitle: A\ntags: [shared]\n---\n# A\n\nContent.")},
		"b.md":     {Data: []byte("---\ntitle: B\ntags: [shared]\n---\n# B\n\nMore.")},
		"unrel.md": {Data: []byte("---\ntitle: Unrelated\ntags: [other]\n---\n# Unrelated")},
	}

	srv := newTestServer(t, files) // existing helper or assemble inline matching this package's pattern
	req := httptest.NewRequest("GET", "/a.md", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "see-also") {
		t.Errorf("response missing see-also section\n%s", body)
	}
	if !strings.Contains(body, ">B<") {
		t.Errorf("see-also missing related page B\n%s", body)
	}
	if strings.Contains(body, ">Unrelated<") {
		t.Errorf("see-also should not include unrelated page\n%s", body)
	}
}
```

`newTestServer(t, files)` may not exist with that exact signature — find the existing setup pattern in `internal/server/handler_test.go` (other tests construct an `HTTPServer` from `fstest.MapFS`) and adapt. The test must exercise the full request flow (provider → enricher → renderer → response) so the new wiring is covered end-to-end.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestServeHTML_IncludesSeeAlsoSection -v`
Expected: FAIL — section not in output (because `RelatedDocs` is not threaded into the template context yet).

- [ ] **Step 3: Wire the field in `serveHTML`**

In `internal/server/handler.go`, locate `serveHTML` (around line 164). Find the `PageContext` literal (around line 175-188). Add the sort call before constructing the context, and add `RelatedDocs` to the literal:

```go
// Sort related docs deterministically. The enricher returns them in
// tag-iteration order, which is non-deterministic from the user's
// perspective; sorting here keeps the rendered see-also section stable.
slices.SortFunc(enrichment.RelatedDocs, func(a, b enricher.RelatedDoc) int {
	return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
})

context := &tmpl.TemplateContext{
	Site: h.siteConfig,
	Page: tmpl.PageContext{
		Content:     template.HTML(htmlContent), // #nosec G203
		Path:        r.URL.Path,
		Meta:        metadata,
		Features:    config.MergeFeatures(h.siteConfig.Theme.Features, enrichment.Features),
		TOC:         enrichment.TOC,
		Navigation:  enrichment.Navigation,
		PrevPage:    enrichment.PrevPage,
		NextPage:    enrichment.NextPage,
		RelatedDocs: enrichment.RelatedDocs,
	},
}
```

Add `"slices"`, `"strings"`, and `"github.com/monolithiclab/gomddoc/internal/enricher"` to the import block if not already present. (`enricher` is likely already imported; `slices` and `strings` may already be too — `gofmt` will tell you.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestServeHTML_IncludesSeeAlsoSection -race -v`
Expected: PASS.

- [ ] **Step 5: Run the full server package**

Run: `go test ./internal/server/ -race`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/handler.go internal/server/handler_test.go
git commit -m "Render see-also section in server response

serveHTML now sorts enrichment.RelatedDocs alphabetically by title
(case-insensitive) and threads it into PageContext so the see-also
partial in the layout can render the section."
```

---

## Task 6: Wire `RelatedDocs` in build mode

**Files:**
- Modify: `cmd/gomddoc/build.go` (around line 444-456)
- Modify: `cmd/gomddoc/build_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/gomddoc/build_test.go`:

```go
func TestBuildCmd_EmitsSeeAlsoSection(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	files := map[string][]byte{
		"a.md": []byte("---\ntitle: A\ntags: [shared]\n---\n# A\n\nContent."),
		"b.md": []byte("---\ntitle: B\ntags: [shared]\n---\n# B\n\nMore."),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(contentDir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	outDir := t.TempDir()
	b := &BuildCmd{Dir: contentDir, Output: outDir}
	if err := b.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(outDir, "a", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "see-also") {
		t.Errorf("a/index.html missing see-also section\n%s", body)
	}
	if !strings.Contains(body, ">B<") {
		t.Errorf("see-also missing related page B\n%s", body)
	}
}
```

If the build's URL-extension stripping produces a different output path (e.g. `a.html` instead of `a/index.html`), adjust the `os.ReadFile` path. Inspect another existing build test (e.g. `TestBuildCmd_EmitsTagPages`) to confirm the convention.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/gomddoc/ -run TestBuildCmd_EmitsSeeAlsoSection -v`
Expected: FAIL — section absent because build path doesn't thread `RelatedDocs` yet.

- [ ] **Step 3: Wire the field in `build.go`**

In `cmd/gomddoc/build.go`, locate the `PageContext` literal in the markdown render path (around line 444-456). It currently looks like:

```go
templateCtx := &tmpl.TemplateContext{
	Site: bc.siteConfig,
	Page: tmpl.PageContext{
		Content:    template.HTML(renderResult.Content), // #nosec G203
		Path:       "/" + filePath,
		Meta:       metadata,
		Features:   config.MergeFeatures(bc.siteConfig.Theme.Features, enrichment.Features),
		TOC:        enrichment.TOC,
		Navigation: enrichment.Navigation,
		PrevPage:   enrichment.PrevPage,
		NextPage:   enrichment.NextPage,
	},
}
```

Add the sort call before constructing the context, and add `RelatedDocs` to the literal:

```go
slices.SortFunc(enrichment.RelatedDocs, func(a, b enricher.RelatedDoc) int {
	return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
})

templateCtx := &tmpl.TemplateContext{
	Site: bc.siteConfig,
	Page: tmpl.PageContext{
		Content:     template.HTML(renderResult.Content), // #nosec G203
		Path:        "/" + filePath,
		Meta:        metadata,
		Features:    config.MergeFeatures(bc.siteConfig.Theme.Features, enrichment.Features),
		TOC:         enrichment.TOC,
		Navigation:  enrichment.Navigation,
		PrevPage:    enrichment.PrevPage,
		NextPage:    enrichment.NextPage,
		RelatedDocs: enrichment.RelatedDocs,
	},
}
```

Add `"slices"`, `"strings"`, and `"github.com/monolithiclab/gomddoc/internal/enricher"` to imports if missing.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/gomddoc/ -run TestBuildCmd_EmitsSeeAlsoSection -race -v`
Expected: PASS.

- [ ] **Step 5: Run the full cmd package**

Run: `go test ./cmd/gomddoc/ -race`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/build.go cmd/gomddoc/build_test.go
git commit -m "Render see-also section in build mode

The build-time markdown render path now sorts and threads
RelatedDocs into the template context, matching the server-side
serveHTML behavior. Build and serve produce identical output."
```

---

## Task 7: Update roadmap

**Files:**
- Modify: `docs/roadmap.md`

- [ ] **Step 1: Mark the item done**

Open `docs/roadmap.md`. Find the "Tag Components" section's "Related pages via tags" line:

```markdown
- [ ] **Related pages via tags**: Display a "Related pages" section at the bottom of each page,
```

Change `- [ ]` to `- [x]`. Leave the description text intact.

- [ ] **Step 2: Verify the full test suite**

Run: `go test ./... -race`
Expected: all packages PASS.

- [ ] **Step 3: Commit**

```bash
git add docs/roadmap.md
git commit -m "Mark Related pages via tags roadmap item done"
```

---

## Coordinated change (separate repo, out of this plan)

After landing all 7 tasks above, open a coordinated PR in `gomddoc-themes` adding the same `{{ template "see-also" . }}` line to each of the 7 themes' main layouts in the same position (between content body and footer). The shared partial uses CSS custom properties that all themes already define, so no per-theme styling is needed.

A separate `gomddoc-website` PR should follow, documenting the new `see_also` feature flag and the `see-also` partial in the theming guide.
