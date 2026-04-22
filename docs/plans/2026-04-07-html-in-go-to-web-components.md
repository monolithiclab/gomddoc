# HTML-in-Go to Web Components Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded HTML in goldmark renderers with `<gmd-*>` web components, move the JSON-LD `<script>` wrapper into its template partial, and rename `<color-chip>` to `<gmd-color-chip>`.

**Architecture:** Goldmark renderers emit semantic custom elements (`<gmd-admonition>`, `<gmd-heading-anchor>`, `<gmd-color-chip>`) instead of hardcoded HTML. Web component JS files in `cmd/gomddoc/assets/shared/` define the client-side behavior, loaded conditionally via `head-shared.html.tmpl`. The JSON-LD `<script>` wrapper moves from Go code into the `jsonld.html.tmpl` partial.

**Tech Stack:** Go 1.26+, goldmark AST renderers, vanilla JS web components (Shadow DOM for heading-anchor, light DOM for admonition)

**Spec:** `docs/specs/2026-04-07-html-in-go-to-web-components.md`

---

### Task 1: Rename `<color-chip>` to `<gmd-color-chip>`

**Files:**
- Modify: `cmd/gomddoc/assets/shared/color-chip.mjs` → rename to `cmd/gomddoc/assets/shared/gmd-color-chip.mjs`
- Modify: `internal/renderer/ext_colorchip.go:123-126`
- Modify: `internal/renderer/colorchip_test.go`
- Modify: `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl:51`
- Modify: `cmd/gomddoc/assets/themes/default/partials/head.html.tmpl:777` (CSS `:not(:defined)` selector)

- [ ] **Step 1: Update the goldmark renderer to emit `<gmd-color-chip>`**

In `internal/renderer/ext_colorchip.go`, change the `renderColorChip` function:

```go
func (r *colorChipRenderer) renderColorChip(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ColorChipNode)
		_, _ = w.WriteString("<gmd-color-chip>")
		_, _ = w.WriteString(n.HexColor)
		_, _ = w.WriteString("</gmd-color-chip>")
	}
	return ast.WalkContinue, nil
}
```

- [ ] **Step 2: Update color chip tests**

In `internal/renderer/colorchip_test.go`, replace all `<color-chip>` / `</color-chip>` assertions with `<gmd-color-chip>` / `</gmd-color-chip>`. Also update the `wantNotContain` check that looks for `"color-chip"` — change it to `"gmd-color-chip"`.

Specifically, update every test case's `wantContains` entries:
- `<color-chip>#FF5733</color-chip>` → `<gmd-color-chip>#FF5733</gmd-color-chip>`
- `<color-chip>#f0f</color-chip>` → `<gmd-color-chip>#f0f</gmd-color-chip>`
- `<color-chip>#aabbcc</color-chip>` → `<gmd-color-chip>#aabbcc</gmd-color-chip>`
- `<color-chip>#FF0000</color-chip>` → `<gmd-color-chip>#FF0000</gmd-color-chip>`
- `<color-chip>#00FF00</color-chip>` → `<gmd-color-chip>#00FF00</gmd-color-chip>`
- `<color-chip>#3366FF</color-chip>` → `<gmd-color-chip>#3366FF</gmd-color-chip>`
- `<color-chip>#FF6633</color-chip>` → `<gmd-color-chip>#FF6633</gmd-color-chip>`

And every `wantNotContain` / string check for `"color-chip"` → `"gmd-color-chip"`.

In `TestColorChips_Disabled`, update:
- `"color-chip"` → `"gmd-color-chip"` in all `strings.Contains` checks.

- [ ] **Step 3: Rename the JS file and update the custom element registration**

Rename `cmd/gomddoc/assets/shared/color-chip.mjs` to `cmd/gomddoc/assets/shared/gmd-color-chip.mjs`.

In the new file, update:
- File header comment: `<color-chip>` → `<gmd-color-chip>`, `Usage: <color-chip>` → `Usage: <gmd-color-chip>`
- Class name: `ColorChip` → `GmdColorChip`
- Registration: `customElements.define("color-chip", ColorChip)` → `customElements.define("gmd-color-chip", GmdColorChip)`

- [ ] **Step 4: Update `head-shared.html.tmpl` asset reference**

In `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl`, line 51:

```
{{- if .Feature "color_chips" }}
<script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
{{- end }}
```

- [ ] **Step 5: Update CSS `:not(:defined)` selector in head.html.tmpl**

In `cmd/gomddoc/assets/themes/default/partials/head.html.tmpl`, around line 777:

```css
/* ===== Color Chips (pre-definition styling) ===== */
gmd-color-chip:not(:defined) {
  font-family: var(--font-family-mono);
  font-size: 0.875em;
}
```

- [ ] **Step 6: Run tests**

Run: `make ci`
Expected: All tests pass. The colorchip tests now assert `<gmd-color-chip>`, the integration test in `markdown_test.go` does NOT assert color-chip output (it's not tested there), so no changes needed there.

- [ ] **Step 7: Commit**

```bash
git add internal/renderer/ext_colorchip.go internal/renderer/colorchip_test.go \
  cmd/gomddoc/assets/shared/gmd-color-chip.mjs \
  cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl \
  cmd/gomddoc/assets/themes/default/partials/head.html.tmpl
git rm cmd/gomddoc/assets/shared/color-chip.mjs
git commit -m "Rename <color-chip> to <gmd-color-chip> for namespace consistency"
```

---

### Task 2: Admonitions → `<gmd-admonition>` web component (light DOM)

**Files:**
- Modify: `internal/renderer/ext_admonition.go:200-217`
- Modify: `internal/renderer/admonition_test.go`
- Create: `cmd/gomddoc/assets/shared/gmd-admonition.mjs`
- Modify: `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl`

- [ ] **Step 1: Update admonition tests to assert web component output**

In `internal/renderer/admonition_test.go`, update the test cases. The goldmark renderer will now emit `<gmd-admonition type="note" title="Note">` instead of `<div class="admonition admonition-note"><p class="admonition-title">Note</p>`.

Replace all `wantContains` entries across test cases:

For "NOTE admonition":
```go
wantContains: []string{
    `<gmd-admonition type="note" title="Note">`,
    "This is a note.",
    `</gmd-admonition>`,
},
wantNotContain: []string{"<blockquote>", "[!NOTE]"},
```

For "WARNING admonition":
```go
wantContains: []string{
    `<gmd-admonition type="warning" title="Warning">`,
    "Be careful!",
    `</gmd-admonition>`,
},
wantNotContain: []string{"<blockquote>", "[!WARNING]"},
```

For "TIP admonition":
```go
wantContains: []string{
    `<gmd-admonition type="tip" title="Tip">`,
    "Helpful tip here.",
    `</gmd-admonition>`,
},
```

For "IMPORTANT admonition":
```go
wantContains: []string{
    `<gmd-admonition type="important" title="Important">`,
    "Do not ignore this.",
    `</gmd-admonition>`,
},
```

For "CAUTION admonition":
```go
wantContains: []string{
    `<gmd-admonition type="caution" title="Caution">`,
    "Danger ahead.",
    `</gmd-admonition>`,
},
```

For "regular blockquote not affected" — no change (doesn't assert admonition HTML).

For "multi-line admonition content":
```go
wantContains: []string{
    `<gmd-admonition type="note" title="Note">`,
    "First line.",
    "Second line.",
    "Third line.",
    `</gmd-admonition>`,
},
wantNotContain: []string{"<blockquote>"},
```

For "multi-paragraph admonition":
```go
wantContains: []string{
    `<gmd-admonition type="warning" title="Warning">`,
    "First paragraph.",
    "Second paragraph.",
    `</gmd-admonition>`,
},
wantNotContain: []string{"<blockquote>"},
```

For "marker only, no content":
```go
wantContains: []string{
    `<gmd-admonition type="tip" title="Tip">`,
    `</gmd-admonition>`,
},
wantNotContain: []string{"<blockquote>"},
```

For "multiple admonitions in same document":
```go
wantContains: []string{
    `type="note"`,
    `type="warning"`,
    "A note.",
    "A warning.",
    "Some text between.",
},
wantNotContain: []string{"<blockquote>"},
```

In `TestAdmonitions_Disabled`, update:
- `"admonition"` checks remain valid — both old and new output contain this string in the tag name. No changes needed.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -run TestAdmonitions -v`
Expected: FAIL — tests now expect `<gmd-admonition` but renderer still emits `<div class="admonition`.

- [ ] **Step 3: Update the goldmark renderer to emit `<gmd-admonition>`**

In `internal/renderer/ext_admonition.go`, change the `renderAdmonition` function:

```go
func (r *admonitionRenderer) renderAdmonition(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n := node.(*AdmonitionNode)

	if entering {
		_, _ = w.WriteString(`<gmd-admonition type="`)
		_, _ = w.WriteString(n.AdmonitionType)
		_, _ = w.WriteString(`" title="`)
		_, _ = w.WriteString(n.Title)
		_, _ = w.WriteString("\">\n")
	} else {
		_, _ = w.WriteString("</gmd-admonition>\n")
	}

	return ast.WalkContinue, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -run TestAdmonitions -v`
Expected: PASS

- [ ] **Step 5: Create the `gmd-admonition.mjs` web component**

Create `cmd/gomddoc/assets/shared/gmd-admonition.mjs`:

```javascript
/**
 * <gmd-admonition> Web Component
 *
 * Renders styled admonition blocks from type/title attributes.
 * Usage: <gmd-admonition type="note" title="Note">Content</gmd-admonition>
 *
 * Features:
 *   - Light DOM (theme CSS applies to inner content)
 *   - Reads type and title attributes
 *   - Prepends title element and applies CSS classes
 */

class GmdAdmonition extends HTMLElement {
  connectedCallback() {
    const type = this.getAttribute("type") || "note";
    const title = this.getAttribute("title") || type;

    this.classList.add("admonition", `admonition-${type}`);

    const titleEl = document.createElement("p");
    titleEl.className = "admonition-title";
    titleEl.textContent = title;
    this.prepend(titleEl);
  }
}

customElements.define("gmd-admonition", GmdAdmonition);
```

- [ ] **Step 6: Add conditional loading in `head-shared.html.tmpl`**

In `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl`, add the admonition script inside the `scripts-shared` block, before the color_chips entry:

```
{{- if .Feature "admonitions" }}
<script>{{ inlineJSAsset "gmd-admonition.mjs" }}</script>
{{- end }}
```

Note: no `type="module"` needed since this is a simple class definition with no imports. Matches the pattern of `toc-highlight.mjs` and `code-copy.mjs`.

- [ ] **Step 7: Run full CI**

Run: `make ci`
Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/renderer/ext_admonition.go internal/renderer/admonition_test.go \
  cmd/gomddoc/assets/shared/gmd-admonition.mjs \
  cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl
git commit -m "Replace admonition HTML-in-Go with <gmd-admonition> web component"
```

---

### Task 3: Heading anchors → `<gmd-heading-anchor>` web component (Shadow DOM)

**Files:**
- Modify: `internal/renderer/ext_anchors.go:52-68`
- Modify: `internal/renderer/anchors_test.go`
- Modify: `internal/renderer/markdown_test.go` (integration test asserts `heading-anchor`)
- Create: `cmd/gomddoc/assets/shared/gmd-heading-anchor.mjs`
- Modify: `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl`
- Modify: `cmd/gomddoc/assets/themes/default/partials/head.html.tmpl` (CSS cleanup)

- [ ] **Step 1: Update heading anchor tests to assert web component output**

In `internal/renderer/anchors_test.go`, update test cases. The renderer will emit `<gmd-heading-anchor href="#id"></gmd-heading-anchor>` instead of `<a href="#id" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`.

For "h1 with auto id":
```go
wantContains: []string{
    `<h1 id="title">`,
    `Title`,
    `<gmd-heading-anchor href="#title"></gmd-heading-anchor>`,
    `</h1>`,
},
```

For "h2 with auto id":
```go
wantContains: []string{
    `<h2 id="section">`,
    `Section`,
    `<gmd-heading-anchor href="#section"`,
},
```

For "h3 with auto id":
```go
wantContains: []string{
    `<h3 id="sub">`,
    `<gmd-heading-anchor href="#sub"`,
},
```

For "heading with inline markup":
```go
wantContains: []string{
    `<h2 id="code-example">`,
    `Code `,
    `<code>example</code>`,
    `<gmd-heading-anchor href="#code-example"`,
},
```

For "multiple headings in same content":
```go
wantContains: []string{
    `<gmd-heading-anchor href="#first"`,
    `<gmd-heading-anchor href="#second"`,
},
```

For "paragraph only, no headings":
```go
wantContains: []string{"<p>Just a paragraph.</p>"},
wantNotContain: []string{
    "gmd-heading-anchor",
},
```

For "empty content":
```go
wantNotContain: []string{"gmd-heading-anchor"},
```

In `TestHeadingAnchors_Disabled`, update the `strings.Contains` check from `"heading-anchor"` to `"gmd-heading-anchor"`. The "globally disabled" test also has an assertion that `heading-anchor` should not appear — change to `gmd-heading-anchor`. The "globally disabled but page enables" test checks `heading-anchor` should appear — change to `gmd-heading-anchor`.

Also update `internal/renderer/markdown_test.go`, the "heading with auto ID" test case:
```go
wantContains: []string{"<h1", `id="hello"`, "gmd-heading-anchor", `href="#hello"`, "</h1>"},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -run "TestHeadingAnchors|TestMarkdownRenderer" -v`
Expected: FAIL

- [ ] **Step 3: Update the goldmark renderer to emit `<gmd-heading-anchor>`**

In `internal/renderer/ext_anchors.go`, replace the anchor rendering block (lines 55-68):

```go
if config.FeatureEnabled("heading_anchors", features) {
    if idAttr, ok := node.AttributeString("id"); ok {
        var id string
        switch v := idAttr.(type) {
        case []byte:
            id = string(v)
        case string:
            id = v
        }
        if id != "" {
            _, _ = w.WriteString(` <gmd-heading-anchor href="#`)
            _, _ = w.WriteString(id)
            _, _ = w.WriteString(`"></gmd-heading-anchor>`)
        }
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -run "TestHeadingAnchors|TestMarkdownRenderer" -v`
Expected: PASS

- [ ] **Step 5: Create the `gmd-heading-anchor.mjs` web component**

Create `cmd/gomddoc/assets/shared/gmd-heading-anchor.mjs`:

```javascript
/**
 * <gmd-heading-anchor> Web Component
 *
 * Renders a heading anchor link (#) revealed on hover.
 * Usage: <gmd-heading-anchor href="#section-id"></gmd-heading-anchor>
 *
 * Features:
 *   - Shadow DOM encapsulation
 *   - Inherits theme colors via CSS custom properties
 *   - Exposed part: ::part(link)
 *   - Accessible: aria-hidden, tabindex=-1
 */

const template = document.createElement("template");
template.innerHTML = `
  <style>
    :host {
      display: inline;
    }
    a {
      color: var(--color-text-muted, #64748b);
      text-decoration: none;
      opacity: 0;
      margin-left: 0.5rem;
      font-weight: 400;
      transition: opacity 0.2s, color 0.2s;
    }
    :host-context(h1:hover) a,
    :host-context(h2:hover) a,
    :host-context(h3:hover) a,
    :host-context(h4:hover) a,
    :host-context(h5:hover) a,
    :host-context(h6:hover) a {
      opacity: 1;
    }
    a:hover {
      color: var(--color-primary, #2563eb);
    }
    @media (hover: none) {
      a { opacity: 1; }
    }
  </style>
  <a part="link" aria-hidden="true" tabindex="-1">#</a>
`;

class GmdHeadingAnchor extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: "open" });
    this.shadowRoot.appendChild(template.content.cloneNode(true));
  }

  connectedCallback() {
    const href = this.getAttribute("href");
    if (href) {
      this.shadowRoot.querySelector("a").href = href;
    }
  }
}

customElements.define("gmd-heading-anchor", GmdHeadingAnchor);
```

- [ ] **Step 6: Add conditional loading in `head-shared.html.tmpl`**

In `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl`, add inside the `scripts-shared` block:

```
{{- if .Feature "heading_anchors" }}
<script>{{ inlineJSAsset "gmd-heading-anchor.mjs" }}</script>
{{- end }}
```

- [ ] **Step 7: Remove heading anchor CSS from `head.html.tmpl`**

In `cmd/gomddoc/assets/themes/default/partials/head.html.tmpl`, remove the heading anchor CSS block (lines 782-809). This includes:
- `.heading-anchor { ... }`
- `h1:hover .heading-anchor, h2:hover .heading-anchor, ...` block
- `.heading-anchor:hover { ... }`
- The `.heading-anchor { opacity: 1; }` line inside the `@media (hover: none)` block

The `@media (hover: none)` block should keep the `.copy-btn { opacity: 1; }` rule but remove the `.heading-anchor { opacity: 1; }` rule.

- [ ] **Step 8: Run full CI**

Run: `make ci`
Expected: All tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/renderer/ext_anchors.go internal/renderer/anchors_test.go \
  internal/renderer/markdown_test.go \
  cmd/gomddoc/assets/shared/gmd-heading-anchor.mjs \
  cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl \
  cmd/gomddoc/assets/themes/default/partials/head.html.tmpl
git commit -m "Replace heading anchor HTML-in-Go with <gmd-heading-anchor> web component"
```

---

### Task 4: Move JSON-LD `<script>` wrapper into template partial

**Files:**
- Modify: `internal/template/renderer.go:348-352`
- Modify: `internal/template/renderer_test.go:1341-1474`
- Modify: `cmd/gomddoc/assets/themes/default/partials/jsonld.html.tmpl`

- [ ] **Step 1: Update JSON-LD tests**

In `internal/template/renderer_test.go`, the `TestJSONLDFunction` test renders a template that calls `{{- jsonLD .Page -}}`. Since the `jsonLD` function will now return only the raw JSON (no `<script>` wrapper), the test template needs updating.

Update the test to render the jsonld partial instead, which will include the `<script>` wrapper. Change the template and test FS:

```go
func TestJSONLDFunction(t *testing.T) {
	t.Parallel()

	// Use a layout that includes the jsonld partial
	layoutContent := `{{ template "jsonld" . }}`
	partialContent := `{{ define "jsonld" }}{{ $json := jsonLD .Page }}{{ if $json }}<script type="application/ld+json">{{ $json }}</script>{{ end }}{{ end }}`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/jsonld.html.tmpl": {
			Data: []byte(layoutContent),
		},
		"assets/themes/default/partials/jsonld.html.tmpl": {
			Data: []byte(partialContent),
		},
	}
```

The rest of the test cases remain the same — they still assert `<script type="application/ld+json">`, `"TechArticle"`, etc. The only difference is the `<script>` tag now comes from the partial instead of the Go function.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/template/ -run TestJSONLDFunction -v`
Expected: FAIL — the `jsonLD` function still returns `<script>` wrapper, so output will have double `<script>` tags.

- [ ] **Step 3: Update `generateJSONLD` to return raw JSON only**

In `internal/template/renderer.go`, change line 352:

```go
raw := seo.GenerateJSONLD(cfg, p)
if raw == "" {
    return ""
}
return template.HTML(raw) // #nosec G203 -- trusted JSON-LD output
```

Remove the `<script type="application/ld+json">` wrapper and the `</script>` suffix.

- [ ] **Step 4: Update the `jsonld.html.tmpl` partial**

In `cmd/gomddoc/assets/themes/default/partials/jsonld.html.tmpl`:

```
{{ define "jsonld" }}{{ $json := jsonLD .Page }}{{ if $json }}<script type="application/ld+json">{{ $json }}</script>{{ end }}{{ end }}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/template/ -run TestJSONLDFunction -v`
Expected: PASS

- [ ] **Step 6: Run full CI**

Run: `make ci`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/template/renderer.go internal/template/renderer_test.go \
  cmd/gomddoc/assets/themes/default/partials/jsonld.html.tmpl
git commit -m "Move JSON-LD <script> wrapper from Go into template partial"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `REVIEW.md`
- Modify: `docs/architecture.md`
- Modify: `docs/decisions.md`

- [ ] **Step 1: Update REVIEW.md**

In `REVIEW.md`, update the HIGH architecture issue. Change the title and content of "HIGH: Hardcoded HTML in Go (Theme Inflexibility)" to mark it as resolved. Replace the section content with a summary of what was done:

```markdown
### ~~HIGH: Hardcoded HTML in Go (Theme Inflexibility)~~ FIXED

Resolved by replacing hardcoded HTML with web components and template partials:
1. **Admonitions** → `<gmd-admonition>` web component (light DOM)
2. **Heading anchors** → `<gmd-heading-anchor>` web component (Shadow DOM)
3. **JSON-LD script wrapper** → moved into `jsonld.html.tmpl` partial
4. **Search `<mark>` tags** → postponed (operates outside template system)
5. **Color chips** → renamed `<color-chip>` to `<gmd-color-chip>` for namespace consistency
```

Update the Executive Summary to reflect no remaining HIGH issues. Update the Architecture score from 8.5/10 to 9.0/10. Update the Themes score from 8.5/10 to 9.0/10. Update Overall from 8.5/10 to 8.5/10 (unchanged — other MEDIUM items remain).

In the Recommendations section, mark item 1 as done:
```markdown
1. ~~**Move HTML-in-Go to templates (HIGH arch)**~~ DONE: Web components + template partial.
```

- [ ] **Step 2: Update `docs/architecture.md` Goldmark Extensions section**

In `docs/architecture.md`, update lines 132-134:

```markdown
1. **Heading Anchors** (`HeadingAnchorExtension`) — Custom `NodeRenderer` for `ast.KindHeading` that appends `<gmd-heading-anchor href="#id">` web component to headings with auto-generated IDs. The web component (Shadow DOM) renders the anchor link with `#` text. Gated by `heading_anchors` feature toggle.
2. **Admonitions** (`AdmonitionExtension`) — AST transformer that detects `[!NOTE]`/`[!TIP]`/`[!IMPORTANT]`/`[!WARNING]`/`[!CAUTION]` patterns in blockquotes and replaces them with `AdmonitionNode` custom AST nodes, rendered as `<gmd-admonition type="..." title="...">` web component elements. The web component (light DOM) adds CSS classes and title element. Gated by `admonitions` feature toggle.
3. **Color Chips** (`ColorChipExtension`) — AST transformer that detects hex color codes in `ast.CodeSpan` nodes and replaces them with `ColorChipNode` custom AST nodes, rendered as `<gmd-color-chip>#HEX</gmd-color-chip>` web component elements. Gated by `color_chips` feature toggle.
```

Update the Color Chip Web Component section (line 449-465):

```markdown
### 14. Web Components

**Responsibility:** Client-side rendering of interactive elements produced by goldmark extensions

**Location:** `cmd/gomddoc/assets/shared/gmd-*.mjs` (shared across all themes via `inlineJSAsset`)

**Components:**

1. **`<gmd-color-chip>`** — Shadow DOM. Renders inline hex color swatches with click-to-copy. Controlled by `color_chips` feature toggle.
2. **`<gmd-admonition>`** — Light DOM. Renders styled admonition blocks (note, tip, warning, etc.) from type/title attributes. Adds CSS classes and title element. Controlled by `admonitions` feature toggle.
3. **`<gmd-heading-anchor>`** — Shadow DOM. Renders heading anchor links (`#`) revealed on hover. Controlled by `heading_anchors` feature toggle.

**Convention:** All custom elements use the `gmd-` prefix. Themes override behavior by providing their own `.mjs` file in theme assets.
```

Update the glossary entry (line 732):
```markdown
- **Web Components**: `<gmd-*>` custom elements that render interactive content produced by goldmark extensions (color chips, admonitions, heading anchors). Shadow DOM or light DOM depending on the component.
```

Update the Goldmark Extensions glossary entry (line 734):
```markdown
- **Goldmark Extensions**: Custom goldmark `Extender` implementations (heading anchors, admonitions, color chips) that operate at the AST level during parsing and rendering, emitting `<gmd-*>` web component elements, gated by feature toggles
```

- [ ] **Step 3: Add entry to `docs/decisions.md`**

Add a new section before "Deferred / Discarded Ideas":

```markdown
## HTML-in-Go to Web Components

**Chosen**: Replace hardcoded HTML in goldmark renderers with `<gmd-*>` web components

**Previous approach**: Goldmark node renderers directly emitted HTML markup (`<div class="admonition">`, `<a class="heading-anchor">`), making it impossible for themes to customize the rendered output.

**Alternatives considered**:
- **Template partials for goldmark output**: Pre-render Go `html/template` partials and inject the HTML strings into goldmark renderers. Would couple the renderer package to the template package and require passing a template executor through the goldmark pipeline.
- **CSS-only customization**: Keep HTML structure but expose CSS custom properties. Limits customization to styling — themes can't change structure, add icons, or alter behavior.

**Why web components**: Goldmark stays concerned with structure (emitting semantic custom elements), while themes control presentation via overridable JS files. Web components are a web standard, require no build tools, and can be overridden per-theme by providing a replacement `.mjs` file.

**Key design decisions**:
- **Light DOM for admonitions**: Theme CSS must apply to inner content (paragraphs, code blocks, lists). Shadow DOM would require extensive `::part()` exposure.
- **Shadow DOM for heading anchors and color chips**: Self-contained elements with no inner content from markdown. Encapsulation prevents style leakage.
- **`gmd-` prefix**: Namespaces all custom elements to avoid collisions. Applied retroactively to `<color-chip>` → `<gmd-color-chip>`.
```

- [ ] **Step 4: Run full CI one final time**

Run: `make ci`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add REVIEW.md docs/architecture.md docs/decisions.md
git commit -m "Update docs for HTML-in-Go to web components migration"
```
