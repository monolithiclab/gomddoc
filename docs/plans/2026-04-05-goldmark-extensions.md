# Post-Processors to Goldmark Extensions — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three regex-based HTML post-processors (heading anchors, admonitions, color chips) with proper goldmark AST extensions.

**Architecture:** Each feature becomes a `goldmark.Extender` registered in the goldmark pipeline. Admonitions and color chips use AST transformers to replace standard nodes with custom AST nodes, plus node renderers. Heading anchors use a renderer override for `ast.KindHeading`. Feature flags are passed via goldmark's parser context so per-page frontmatter overrides still work.

**Tech Stack:** goldmark AST (`github.com/yuin/goldmark/ast`), goldmark parser/renderer interfaces

**Spec:** `docs/specs/2026-04-05-goldmark-extensions-design.md`

---

## File Map

| File                                   | Action  | Responsibility                                                                    |
| -------------------------------------- | ------- | --------------------------------------------------------------------------------- |
| `internal/renderer/ext_features.go`    | Create  | Shared `featuresContextKey` + helper to read features from parser context         |
| `internal/renderer/ext_anchors.go`     | Create  | Heading anchor extension: `Extender` + `NodeRenderer` for `ast.KindHeading`       |
| `internal/renderer/ext_admonition.go`  | Create  | Admonition extension: `AdmonitionNode` + `ASTTransformer` + `NodeRenderer`        |
| `internal/renderer/ext_colorchip.go`   | Create  | Color chip extension: `ColorChipNode` + `ASTTransformer` + `NodeRenderer`         |
| `internal/renderer/markdown.go`        | Modify  | Register extensions, remove post-processing block, set features on parser context |
| `internal/renderer/anchors.go`         | Delete  | Replaced by `ext_anchors.go`                                                      |
| `internal/renderer/admonition.go`      | Delete  | Replaced by `ext_admonition.go`                                                   |
| `internal/renderer/colorchip.go`       | Delete  | Replaced by `ext_colorchip.go`                                                    |
| `internal/renderer/anchors_test.go`    | Rewrite | Test via renderer (markdown input → HTML output)                                  |
| `internal/renderer/admonition_test.go` | Rewrite | Test via renderer (markdown input → HTML output)                                  |
| `internal/renderer/colorchip_test.go`  | Rewrite | Test via renderer (markdown input → HTML output)                                  |

---

### Task 1: Shared Feature Context Key

**Files:**

- Create: `internal/renderer/ext_features.go`

- [ ] **Step 1: Create `ext_features.go`**

```go
package renderer

import "github.com/yuin/goldmark/parser"

// featuresContextKey is the parser context key used to pass merged feature flags
// to goldmark extensions. Set by MarkdownRenderer.Render() before rendering.
var featuresContextKey = parser.NewContextKey()

// getFeatures reads the merged feature map from the parser context.
// Returns nil if not set.
func getFeatures(pc parser.Context) map[string]bool {
	v := pc.Get(featuresContextKey)
	if v == nil {
		return nil
	}
	f, _ := v.(map[string]bool)
	return f
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go build ./internal/renderer/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/renderer/ext_features.go
git commit -m "Add shared feature context key for goldmark extensions"
```

---

### Task 2: Heading Anchors Extension

**Files:**

- Create: `internal/renderer/ext_anchors.go`
- Rewrite: `internal/renderer/anchors_test.go`
- Delete: `internal/renderer/anchors.go`

- [ ] **Step 1: Write the failing tests**

Rewrite `internal/renderer/anchors_test.go`. Tests go through the full renderer with markdown input, since the extension operates at the renderer level. The existing `TestAddHeadingAnchors` tests on raw HTML become renderer-based tests on markdown input.

```go
package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestHeadingAnchors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "h1 gets anchor",
			input: "# Title",
			wantContains: []string{
				`<h1 id="title">`,
				`Title <a href="#title" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
				`</h1>`,
			},
		},
		{
			name:  "h2 gets anchor",
			input: "## Section",
			wantContains: []string{
				`<h2 id="section">`,
				`Section <a href="#section" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
				`</h2>`,
			},
		},
		{
			name:  "h3 gets anchor",
			input: "### Sub",
			wantContains: []string{
				`<h3 id="sub">`,
				`Sub <a href="#sub" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
			},
		},
		{
			name:  "h4 gets anchor",
			input: "#### Deep",
			wantContains: []string{
				`<h4 id="deep">`,
				`Deep <a href="#deep" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
			},
		},
		{
			name:  "h5 gets anchor",
			input: "##### Deeper",
			wantContains: []string{
				`<h5 id="deeper">`,
				`Deeper <a href="#deeper" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
			},
		},
		{
			name:  "h6 gets anchor",
			input: "###### Deepest",
			wantContains: []string{
				`<h6 id="deepest">`,
				`Deepest <a href="#deepest" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
			},
		},
		{
			name:  "multiple headings",
			input: "# First\n\nSome text.\n\n## Second",
			wantContains: []string{
				`href="#first"`,
				`href="#second"`,
			},
		},
		{
			name:  "heading with special characters in id",
			input: "## Hello World 123",
			wantContains: []string{
				`href="#hello-world-123"`,
			},
		},
		{
			name:  "heading with inline code",
			input: "## Code `example`",
			wantContains: []string{
				`<code>example</code>`,
				`class="heading-anchor"`,
			},
		},
		{
			name:           "paragraph has no anchor",
			input:          "Just a paragraph.",
			wantNotContain: []string{"heading-anchor"},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := r.Render(ctx, []byte(tt.input), &enricher.EnrichmentData{})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			output := string(result.Content)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, output)
				}
			}
		})
	}
}

func TestHeadingAnchors_Disabled(t *testing.T) {
	t.Parallel()

	r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"heading_anchors": false}})
	result, err := r.Render(context.Background(), []byte("# Title"), &enricher.EnrichmentData{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	output := string(result.Content)
	if strings.Contains(output, "heading-anchor") {
		t.Error("heading anchors should not appear when disabled")
	}
	if !strings.Contains(output, "<h1") {
		t.Error("heading should still render")
	}
}

func TestHeadingAnchors_PageOverride(t *testing.T) {
	t.Parallel()

	t.Run("globally enabled, page disables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(context.Background(), []byte("# Title"), &enricher.EnrichmentData{
			Features: map[string]bool{"heading_anchors": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "heading-anchor") {
			t.Error("heading anchors should not appear when page disables them")
		}
	})

	t.Run("globally disabled, page enables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"heading_anchors": false}})
		result, err := r.Render(context.Background(), []byte("# Title"), &enricher.EnrichmentData{
			Features: map[string]bool{"heading_anchors": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "heading-anchor") {
			t.Error("heading anchors should appear when page enables them")
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -run "TestHeadingAnchors" -v`
Expected: Tests fail because `anchors.go` still has the old `addHeadingAnchors` function and the new extension doesn't exist yet. The tests won't compile initially because they depend on the renderer changes in Task 5.

Note: These tests will only compile after Task 5 (which wires up the extensions in `markdown.go`). For now, confirm the test file is saved correctly.

- [ ] **Step 3: Create `ext_anchors.go`**

```go
package renderer

import (
	"fmt"
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// HeadingAnchorExtension adds anchor links to headings that have an ID attribute.
// The anchor uses the "#" symbol and is styled via CSS.
//
// Feature flag: "heading_anchors" (default: enabled).
type HeadingAnchorExtension struct{}

func (e *HeadingAnchorExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(gmRenderer.WithNodeRenderers(
		util.Prioritized(&headingAnchorRenderer{}, 100),
	))
}

type headingAnchorRenderer struct{}

func (r *headingAnchorRenderer) RegisterFuncs(reg gmRenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
}

func (r *headingAnchorRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)

	if entering {
		_, _ = w.WriteString("<h")
		_ = w.WriteByte("0123456"[n.Level])
		if n.Attributes() != nil {
			html.RenderAttributes(w, node, html.HeadingAttributeFilter)
		}
		_ = w.WriteByte('>')
		return ast.WalkContinue, nil
	}

	// Exiting: append anchor if heading has an ID and feature is enabled.
	idAttr, hasID := n.AttributeString("id")
	pc := node.OwnerDocument().Meta()
	features := getFeaturesFromMeta(pc)

	if hasID && config.FeatureEnabled("heading_anchors", features) {
		id := fmt.Sprintf("%s", idAttr)
		_, _ = w.WriteString(` <a href="#`)
		_, _ = w.WriteString(id)
		_, _ = w.WriteString(`" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`)
	}

	_, _ = w.WriteString("</h")
	_ = w.WriteByte("0123456"[n.Level])
	_, _ = w.WriteString(">\n")
	return ast.WalkContinue, nil
}
```

Wait — `OwnerDocument().Meta()` is not the parser context. The parser context is not available at render time via the node. Let me reconsider the design for passing features to the renderer.

The goldmark renderer doesn't have access to the parser context. We need a different approach for the heading anchor renderer. Options:

1. Store features in the document's metadata map via `doc.SetAttributeString`
2. Use an AST transformer for heading anchors too (adds anchor nodes to the AST)
3. Store features in a sync-safe side channel on the extension struct

Option 2 is cleanest — it keeps the heading anchor extension consistent with the other two. Let me revise.

Actually, let me re-examine. The goldmark renderer receives `source []byte` and `node ast.Node`. The node's `OwnerDocument()` returns `*ast.Document`. We can set a custom attribute on the document node before rendering. In `MarkdownRenderer.Render()`, after parsing and before rendering, we can set features on the document:

```go
doc.SetAttributeString("features", merged)
```

Then in the renderer:

```go
featuresAttr, _ := node.OwnerDocument().AttributeString("features")
features, _ := featuresAttr.(map[string]bool)
```

This is clean and doesn't require a side channel. Let me update `ext_features.go` accordingly.

- [ ] **Step 3 (revised): Update `ext_features.go` to use document attribute**

Replace the parser context approach with a document attribute approach:

```go
package renderer

import "github.com/yuin/goldmark/ast"

// featuresDocAttr is the document attribute key for merged feature flags.
// Set on the parsed document by MarkdownRenderer.Render() before rendering.
const featuresDocAttr = "gomddoc:features"

// setDocFeatures stores the merged feature map on the document node.
func setDocFeatures(doc *ast.Document, features map[string]bool) {
	doc.SetAttributeString(featuresDocAttr, features)
}

// getDocFeatures reads the merged feature map from a node's owner document.
// Returns nil if not set.
func getDocFeatures(node ast.Node) map[string]bool {
	doc := node.OwnerDocument()
	if doc == nil {
		return nil
	}
	v, ok := doc.AttributeString(featuresDocAttr)
	if !ok {
		return nil
	}
	f, _ := v.(map[string]bool)
	return f
}
```

- [ ] **Step 4: Create `ext_anchors.go` (revised)**

```go
package renderer

import (
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// HeadingAnchorExtension adds anchor links to headings that have an ID attribute.
// The anchor uses the "#" symbol and is styled via CSS.
//
// Feature flag: "heading_anchors" (default: enabled).
type HeadingAnchorExtension struct{}

func (e *HeadingAnchorExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(gmRenderer.WithNodeRenderers(
		util.Prioritized(&headingAnchorRenderer{}, 100),
	))
}

type headingAnchorRenderer struct{}

func (r *headingAnchorRenderer) RegisterFuncs(reg gmRenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
}

func (r *headingAnchorRenderer) renderHeading(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)

	if entering {
		_, _ = w.WriteString("<h")
		_ = w.WriteByte("0123456"[n.Level])
		if n.Attributes() != nil {
			html.RenderAttributes(w, node, html.HeadingAttributeFilter)
		}
		_ = w.WriteByte('>')
		return ast.WalkContinue, nil
	}

	// Exiting: append anchor if heading has an ID and feature is enabled.
	idAttr, hasID := n.AttributeString("id")
	features := getDocFeatures(node)

	if hasID && config.FeatureEnabled("heading_anchors", features) {
		id := fmt.Sprintf("%s", idAttr)
		_, _ = w.WriteString(` <a href="#`)
		_, _ = w.WriteString(id)
		_, _ = w.WriteString(`" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`)
	}

	_, _ = w.WriteString("</h")
	_ = w.WriteByte("0123456"[n.Level])
	_, _ = w.WriteString(">\n")
	return ast.WalkContinue, nil
}
```

- [ ] **Step 5: Delete `anchors.go`**

```bash
rm internal/renderer/anchors.go
```

Note: This will break compilation until Task 5 removes references from `markdown.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/renderer/ext_features.go internal/renderer/ext_anchors.go internal/renderer/anchors_test.go
git rm internal/renderer/anchors.go
git commit -m "Add heading anchor goldmark extension (replaces regex post-processor)"
```

---

### Task 3: Admonition Extension

**Files:**

- Create: `internal/renderer/ext_admonition.go`
- Rewrite: `internal/renderer/admonition_test.go`
- Delete: `internal/renderer/admonition.go`

- [ ] **Step 1: Write the failing tests**

Rewrite `internal/renderer/admonition_test.go`. All tests use markdown input through the renderer.

```go
package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestAdmonitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "NOTE admonition",
			input: "> [!NOTE]\n> This is a note.",
			wantContains: []string{
				`class="admonition admonition-note"`,
				`class="admonition-title"`,
				"Note",
				"This is a note.",
			},
			wantNotContain: []string{"<blockquote>", "[!NOTE]"},
		},
		{
			name: "WARNING admonition",
			input: "> [!WARNING]\n> Be careful!",
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"Warning",
				"Be careful!",
			},
			wantNotContain: []string{"<blockquote>", "[!WARNING]"},
		},
		{
			name: "TIP admonition",
			input: "> [!TIP]\n> Helpful tip here.",
			wantContains: []string{
				`class="admonition admonition-tip"`,
				"Tip",
				"Helpful tip here.",
			},
		},
		{
			name: "IMPORTANT admonition",
			input: "> [!IMPORTANT]\n> Do not ignore this.",
			wantContains: []string{
				`class="admonition admonition-important"`,
				"Important",
				"Do not ignore this.",
			},
		},
		{
			name: "CAUTION admonition",
			input: "> [!CAUTION]\n> Danger ahead.",
			wantContains: []string{
				`class="admonition admonition-caution"`,
				"Caution",
				"Danger ahead.",
			},
		},
		{
			name:           "regular blockquote not affected",
			input:          "> This is a regular blockquote.",
			wantContains:   []string{"<blockquote>", "This is a regular blockquote."},
			wantNotContain: []string{"admonition"},
		},
		{
			name: "multi-paragraph admonition",
			input: "> [!WARNING]\n> First paragraph.\n>\n> Second paragraph.",
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"First paragraph.",
				"Second paragraph.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name: "admonition with no content after marker",
			input: "> [!TIP]",
			wantContains: []string{
				`class="admonition admonition-tip"`,
				`class="admonition-title"`,
				"Tip",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name: "multiple admonitions in same document",
			input: "> [!NOTE]\n> A note.\n\nSome text between.\n\n> [!WARNING]\n> A warning.",
			wantContains: []string{
				"admonition-note",
				"admonition-warning",
				"A note.",
				"A warning.",
				"Some text between.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name: "case insensitive type marker",
			input: "> [!note]\n> Lowercase marker.",
			wantContains: []string{
				`class="admonition admonition-note"`,
				"Lowercase marker.",
			},
		},
		{
			name: "mixed case type marker",
			input: "> [!Note]\n> Mixed case.",
			wantContains: []string{
				`class="admonition admonition-note"`,
				"Mixed case.",
			},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := r.Render(ctx, []byte(tt.input), &enricher.EnrichmentData{})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			output := string(result.Content)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, output)
				}
			}
		})
	}
}

func TestAdmonitions_Disabled(t *testing.T) {
	t.Parallel()

	r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"admonitions": false}})
	result, err := r.Render(context.Background(), []byte("> [!NOTE]\n> A note."), &enricher.EnrichmentData{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	output := string(result.Content)
	if strings.Contains(output, "admonition") {
		t.Error("admonitions should not appear when disabled")
	}
	if !strings.Contains(output, "<blockquote>") {
		t.Error("blockquote should render normally when admonitions disabled")
	}
}

func TestAdmonitions_PageOverride(t *testing.T) {
	t.Parallel()

	t.Run("globally enabled, page disables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(context.Background(), []byte("> [!NOTE]\n> A note."), &enricher.EnrichmentData{
			Features: map[string]bool{"admonitions": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "admonition") {
			t.Error("admonitions should not appear when page disables them")
		}
	})

	t.Run("globally disabled, page enables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"admonitions": false}})
		result, err := r.Render(context.Background(), []byte("> [!NOTE]\n> A note."), &enricher.EnrichmentData{
			Features: map[string]bool{"admonitions": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "admonition") {
			t.Error("admonitions should appear when page enables them")
		}
	})
}
```

- [ ] **Step 2: Create `ext_admonition.go`**

```go
package renderer

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmParser "github.com/yuin/goldmark/parser"
	gmRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// admonitionType represents a supported GitHub-style admonition type.
type admonitionType struct {
	Name  string // Lowercase name for CSS class.
	Title string // Display title.
}

// supportedAdmonitions maps uppercase marker names to their display properties.
var supportedAdmonitions = map[string]admonitionType{
	"NOTE":      {Name: "note", Title: "Note"},
	"TIP":       {Name: "tip", Title: "Tip"},
	"IMPORTANT": {Name: "important", Title: "Important"},
	"WARNING":   {Name: "warning", Title: "Warning"},
	"CAUTION":   {Name: "caution", Title: "Caution"},
}

var admonitionMarkerPattern = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]`)

// AdmonitionNode is a block node representing a GitHub-style admonition.
type AdmonitionNode struct {
	ast.BaseBlock
	AdmonitionType string // Lowercase type name (e.g., "note").
	Title          string // Display title (e.g., "Note").
}

var KindAdmonition = ast.NewNodeKind("Admonition")

func (n *AdmonitionNode) Kind() ast.NodeKind { return KindAdmonition }
func (n *AdmonitionNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Type":  n.AdmonitionType,
		"Title": n.Title,
	}, nil)
}

// AdmonitionExtension converts GitHub-style admonition blockquotes to
// semantic admonition divs.
//
// Feature flag: "admonitions" (default: enabled).
type AdmonitionExtension struct{}

func (e *AdmonitionExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(gmParser.WithASTTransformers(
		util.Prioritized(&admonitionTransformer{}, 100),
	))
	m.Renderer().AddOptions(gmRenderer.WithNodeRenderers(
		util.Prioritized(&admonitionRenderer{}, 100),
	))
}

// admonitionTransformer walks the AST looking for blockquotes whose first
// paragraph starts with [!TYPE] and replaces them with AdmonitionNode.
type admonitionTransformer struct{}

func (t *admonitionTransformer) Transform(doc *ast.Document, reader text.Reader, pc gmParser.Context) {
	features := getFeatures(pc)
	if !config.FeatureEnabled("admonitions", features) {
		return
	}

	source := reader.Source()
	var toReplace []struct {
		bq  *ast.Blockquote
		adm admonitionType
	}

	// Collect blockquotes to transform (don't modify tree while walking).
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindBlockquote {
			return ast.WalkContinue, nil
		}
		bq := node.(*ast.Blockquote)
		firstChild := bq.FirstChild()
		if firstChild == nil || firstChild.Kind() != ast.KindParagraph {
			return ast.WalkContinue, nil
		}

		// Read the first text node of the first paragraph.
		textNode := firstChild.FirstChild()
		if textNode == nil || textNode.Kind() != ast.KindText {
			return ast.WalkContinue, nil
		}
		tn := textNode.(*ast.Text)
		line := tn.Segment.Value(source)
		lineStr := strings.TrimSpace(string(line))

		match := admonitionMarkerPattern.FindStringSubmatch(lineStr)
		if match == nil {
			return ast.WalkContinue, nil
		}

		typeKey := strings.ToUpper(match[1])
		adm, ok := supportedAdmonitions[typeKey]
		if !ok {
			return ast.WalkContinue, nil
		}

		toReplace = append(toReplace, struct {
			bq  *ast.Blockquote
			adm admonitionType
		}{bq, adm})

		return ast.WalkSkipChildren, nil
	})

	// Replace collected blockquotes with admonition nodes.
	for _, item := range toReplace {
		replaceBlockquoteWithAdmonition(doc, item.bq, item.adm, source)
	}
}

// replaceBlockquoteWithAdmonition replaces a blockquote node with an
// AdmonitionNode, stripping the [!TYPE] marker from the first paragraph.
func replaceBlockquoteWithAdmonition(doc *ast.Document, bq *ast.Blockquote, adm admonitionType, source []byte) {
	admNode := &AdmonitionNode{
		AdmonitionType: adm.Name,
		Title:          adm.Title,
	}

	firstPara := bq.FirstChild()

	// Strip the [!TYPE] marker from the first text node.
	if firstPara != nil && firstPara.Kind() == ast.KindParagraph {
		textNode := firstPara.FirstChild()
		if textNode != nil && textNode.Kind() == ast.KindText {
			tn := textNode.(*ast.Text)
			line := string(tn.Segment.Value(source))
			lineStr := strings.TrimSpace(line)

			match := admonitionMarkerPattern.FindStringIndex(lineStr)
			if match != nil {
				remaining := strings.TrimSpace(lineStr[match[1]:])
				if remaining == "" {
					// Marker is the only content in this text node.
					// Check if there's a next sibling with the actual content.
					next := textNode.NextSibling()
					firstPara.RemoveChild(firstPara, textNode)
					// If removing the marker left the paragraph empty and
					// there are no remaining children, remove the paragraph.
					if !firstPara.HasChildren() && next == nil {
						bq.RemoveChild(bq, firstPara)
					}
				} else {
					// Replace the segment with just the remaining text.
					newSeg := tn.Segment
					newSeg = newSeg.WithStart(newSeg.Start + match[1] + (len(line) - len(lineStr)))
					// Trim leading whitespace from remaining content.
					for newSeg.Start < newSeg.Stop && source[newSeg.Start] == ' ' {
						newSeg = newSeg.WithStart(newSeg.Start + 1)
					}
					tn.Segment = newSeg
				}
			}
		}
	}

	// Move all children from blockquote to admonition.
	for c := bq.FirstChild(); c != nil; {
		next := c.NextSibling()
		bq.RemoveChild(bq, c)
		admNode.AppendChild(admNode, c)
		c = next
	}

	// Replace blockquote with admonition in the parent.
	parent := bq.Parent()
	parent.ReplaceChild(parent, bq, admNode)
}

// admonitionRenderer renders AdmonitionNode to HTML.
type admonitionRenderer struct{}

func (r *admonitionRenderer) RegisterFuncs(reg gmRenderer.NodeRendererFuncRegisterer) {
	reg.Register(KindAdmonition, r.renderAdmonition)
}

func (r *admonitionRenderer) renderAdmonition(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n := node.(*AdmonitionNode)
	if entering {
		_, _ = w.WriteString(`<div class="admonition admonition-`)
		_, _ = w.WriteString(n.AdmonitionType)
		_, _ = w.WriteString(`"><p class="admonition-title">`)
		_, _ = w.WriteString(n.Title)
		_, _ = w.WriteString("</p>\n")
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}
```

**Important note:** The AST manipulation for stripping the `[!TYPE]` marker is the most complex part. The exact byte offsets depend on how goldmark structures the text segments. This implementation uses segment manipulation to strip the marker. During implementation, you may need to adjust the segment arithmetic based on actual goldmark AST output — use `ast.Dump` to inspect the tree structure if the output doesn't match.

The admonition transformer reads features from the **parser context** (not document attribute), because transformers have access to `parser.Context`. This means `ext_features.go` needs both approaches: parser context key for transformers, document attribute for renderers.

- [ ] **Step 3: Update `ext_features.go` to support both parser context and document attribute**

```go
package renderer

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// featuresContextKey is the parser context key for merged feature flags.
// Used by AST transformers which have access to parser.Context.
var featuresContextKey = parser.NewContextKey()

// featuresDocAttr is the document attribute key for merged feature flags.
// Used by node renderers which only have access to ast.Node.
const featuresDocAttr = "gomddoc:features"

// getFeatures reads the merged feature map from the parser context.
func getFeatures(pc parser.Context) map[string]bool {
	v := pc.Get(featuresContextKey)
	if v == nil {
		return nil
	}
	f, _ := v.(map[string]bool)
	return f
}

// setDocFeatures stores the merged feature map on the document node.
func setDocFeatures(doc *ast.Document, features map[string]bool) {
	doc.SetAttributeString(featuresDocAttr, features)
}

// getDocFeatures reads the merged feature map from a node's owner document.
func getDocFeatures(node ast.Node) map[string]bool {
	doc := node.OwnerDocument()
	if doc == nil {
		return nil
	}
	v, ok := doc.AttributeString(featuresDocAttr)
	if !ok {
		return nil
	}
	f, _ := v.(map[string]bool)
	return f
}
```

- [ ] **Step 4: Delete `admonition.go`**

```bash
rm internal/renderer/admonition.go
```

- [ ] **Step 5: Commit**

```bash
git add internal/renderer/ext_admonition.go internal/renderer/ext_features.go internal/renderer/admonition_test.go
git rm internal/renderer/admonition.go
git commit -m "Add admonition goldmark extension (replaces regex post-processor)"
```

---

### Task 4: Color Chip Extension

**Files:**

- Create: `internal/renderer/ext_colorchip.go`
- Rewrite: `internal/renderer/colorchip_test.go`
- Delete: `internal/renderer/colorchip.go`

- [ ] **Step 1: Write the failing tests**

Rewrite `internal/renderer/colorchip_test.go`:

````go
package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestColorChips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "6-digit hex in backticks",
			input: "The brand color is `#FF5733` and it looks great.",
			wantContains: []string{
				`<color-chip>#FF5733</color-chip>`,
			},
			wantNotContain: []string{"<code>#FF5733</code>"},
		},
		{
			name:  "3-digit hex in backticks",
			input: "Short form: `#f00`",
			wantContains: []string{
				`<color-chip>#f00</color-chip>`,
			},
			wantNotContain: []string{"<code>#f00</code>"},
		},
		{
			name:  "lowercase 6-digit hex",
			input: "Color: `#aabbcc`",
			wantContains: []string{
				`<color-chip>#aabbcc</color-chip>`,
			},
		},
		{
			name:  "multiple colors in same paragraph",
			input: "Mix `#FF0000` with `#00FF00`.",
			wantContains: []string{
				`<color-chip>#FF0000</color-chip>`,
				`<color-chip>#00FF00</color-chip>`,
			},
		},
		{
			name:  "colors in list",
			input: "- Primary: `#3366FF`\n- Secondary: `#FF6633`",
			wantContains: []string{
				`<color-chip>#3366FF</color-chip>`,
				`<color-chip>#FF6633</color-chip>`,
			},
		},
		{
			name:           "code with extra text not transformed",
			input:          "Use `color: #FF5733` in your CSS.",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"#FF5733"},
		},
		{
			name:           "bare hex in text not transformed",
			input:          "The color #FF5733 looks good.",
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "hex in fenced code block not transformed",
			input:          "```css\ncolor: #FF5733;\n```",
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "markdown link fragment not transformed",
			input:          "See [section](#overview) for details.",
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "invalid hex too short",
			input:          "`#ab`",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#ab</code>"},
		},
		{
			name:           "invalid hex too long",
			input:          "`#1234567`",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#1234567</code>"},
		},
		{
			name:           "non-hex chars not transformed",
			input:          "`#GGHHII`",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#GGHHII</code>"},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := r.Render(ctx, []byte(tt.input), &enricher.EnrichmentData{})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			output := string(result.Content)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, output)
				}
			}
		})
	}
}

func TestColorChips_Disabled(t *testing.T) {
	t.Parallel()

	r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": false}})
	result, err := r.Render(context.Background(), []byte("Color: `#FF5733`"), &enricher.EnrichmentData{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(result.Content), "color-chip") {
		t.Error("color chips should not appear when globally disabled")
	}
}

func TestColorChips_PageOverride(t *testing.T) {
	t.Parallel()

	t.Run("globally enabled, page disables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(context.Background(), []byte("Color: `#FF5733`"), &enricher.EnrichmentData{
			Features: map[string]bool{"color_chips": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when page disables them")
		}
	})

	t.Run("globally disabled, page enables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": false}})
		result, err := r.Render(context.Background(), []byte("Color: `#FF5733`"), &enricher.EnrichmentData{
			Features: map[string]bool{"color_chips": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should appear when page enables them")
		}
	})
}
````

- [ ] **Step 2: Create `ext_colorchip.go`**

```go
package renderer

import (
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmParser "github.com/yuin/goldmark/parser"
	gmRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

var hexColorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`)

// ColorChipNode is an inline node representing a hex color chip.
type ColorChipNode struct {
	ast.BaseInline
	HexColor string // e.g., "#FF5733"
}

var KindColorChip = ast.NewNodeKind("ColorChip")

func (n *ColorChipNode) Kind() ast.NodeKind { return KindColorChip }
func (n *ColorChipNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"HexColor": n.HexColor,
	}, nil)
}

// ColorChipExtension replaces inline code spans containing only a hex color
// code with <color-chip> web components.
//
// Feature flag: "color_chips" (default: enabled).
type ColorChipExtension struct{}

func (e *ColorChipExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(gmParser.WithASTTransformers(
		util.Prioritized(&colorChipTransformer{}, 100),
	))
	m.Renderer().AddOptions(gmRenderer.WithNodeRenderers(
		util.Prioritized(&colorChipRenderer{}, 100),
	))
}

// colorChipTransformer walks the AST looking for CodeSpan nodes whose content
// is exactly a hex color code, and replaces them with ColorChipNode.
type colorChipTransformer struct{}

func (t *colorChipTransformer) Transform(doc *ast.Document, reader text.Reader, pc gmParser.Context) {
	features := getFeatures(pc)
	if !config.FeatureEnabled("color_chips", features) {
		return
	}

	source := reader.Source()
	var toReplace []struct {
		codeSpan ast.Node
		hex      string
	}

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindCodeSpan {
			return ast.WalkContinue, nil
		}

		// Read the code span text from its children (ast.Text nodes).
		var content string
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if tn, ok := c.(*ast.Text); ok {
				content += string(tn.Segment.Value(source))
			}
		}

		if hexColorPattern.MatchString(content) {
			toReplace = append(toReplace, struct {
				codeSpan ast.Node
				hex      string
			}{node, content})
		}

		return ast.WalkSkipChildren, nil
	})

	for _, item := range toReplace {
		chip := &ColorChipNode{HexColor: item.hex}
		parent := item.codeSpan.Parent()
		parent.ReplaceChild(parent, item.codeSpan, chip)
	}
}

// colorChipRenderer renders ColorChipNode to HTML.
type colorChipRenderer struct{}

func (r *colorChipRenderer) RegisterFuncs(reg gmRenderer.NodeRendererFuncRegisterer) {
	reg.Register(KindColorChip, r.renderColorChip)
}

func (r *colorChipRenderer) renderColorChip(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ColorChipNode)
		_, _ = w.WriteString(`<color-chip>`)
		_, _ = w.WriteString(n.HexColor)
		_, _ = w.WriteString(`</color-chip>`)
	}
	return ast.WalkContinue, nil
}
```

- [ ] **Step 3: Delete `colorchip.go`**

```bash
rm internal/renderer/colorchip.go
```

- [ ] **Step 4: Commit**

```bash
git add internal/renderer/ext_colorchip.go internal/renderer/colorchip_test.go
git rm internal/renderer/colorchip.go
git commit -m "Add color chip goldmark extension (replaces regex post-processor)"
```

---

### Task 5: Wire Extensions into MarkdownRenderer

**Files:**

- Modify: `internal/renderer/markdown.go`

- [ ] **Step 1: Update `markdown.go`**

Replace the contents of `internal/renderer/markdown.go`. Key changes:

1. Register the three extensions in `goldmark.New()`
2. Set features on both parser context and document attribute before rendering
3. Remove the post-processing block entirely

```go
package renderer

import (
	"bytes"
	"context"
	"mime"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func init() {
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
	_ = mime.AddExtensionType(".markdown", "text/markdown; charset=utf-8")
}

// MarkdownOptions configures the MarkdownRenderer.
type MarkdownOptions struct {
	// HighlightTheme controls the Chroma syntax highlighting style for fenced
	// code blocks. Defaults to "github" when empty.
	HighlightTheme string

	// Features controls which markdown rendering features are enabled.
	// If nil, all features default to enabled.
	// Per-page frontmatter can override site-level settings.
	Features map[string]bool
}

// MarkdownRenderer transforms markdown content to HTML.
// It uses the goldmark library with GitHub Flavored Markdown (GFM) and Front Matter support.
//
// The renderer is stateless and thread-safe. The underlying goldmark instance
// is shared across renders.
type MarkdownRenderer struct {
	md       goldmark.Markdown
	features map[string]bool
}

// NewMarkdownRenderer creates a new markdown renderer with the given options.
func NewMarkdownRenderer(opts MarkdownOptions) *MarkdownRenderer {
	highlightTheme := opts.HighlightTheme
	if highlightTheme == "" {
		highlightTheme = "github"
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle(highlightTheme),
			),
			&HeadingAnchorExtension{},
			&AdmonitionExtension{},
			&ColorChipExtension{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	return &MarkdownRenderer{
		md:       md,
		features: opts.Features,
	}
}

// InputMimeTypes returns the MIME types this renderer accepts as input.
func (m *MarkdownRenderer) InputMimeTypes() []string {
	return []string{"text/markdown"}
}

// OutputMimeTypes returns the MIME types this renderer produces.
func (m *MarkdownRenderer) OutputMimeTypes() []string {
	return []string{"text/html"}
}

// Render converts markdown content to HTML.
func (m *MarkdownRenderer) Render(ctx context.Context, content []byte, enrichment *enricher.EnrichmentData) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Merge site-level and page-level feature flags.
	var pageFeatures map[string]bool
	if enrichment != nil {
		pageFeatures = enrichment.Features
	}
	merged := config.MergeFeatures(m.features, pageFeatures)

	// Set features on parser context for AST transformers.
	pCtx := parser.NewContext()
	pCtx.Set(featuresContextKey, merged)

	reader := text.NewReader(content)
	doc := m.md.Parser().Parse(reader, parser.WithContext(pCtx))

	// Set features on document for node renderers.
	setDocFeatures(doc, merged)

	var buf bytes.Buffer
	if err := m.md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, err
	}

	return &RenderResult{
		Content:  buf.Bytes(),
		MimeType: "text/html; charset=utf-8",
	}, nil
}
```

- [ ] **Step 2: Run all tests**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/renderer/ -v -count=1`
Expected: All tests pass. If any fail, debug by comparing actual vs expected HTML output.

- [ ] **Step 3: Run `make ci`**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: Full pipeline passes (lint + tests + coverage).

- [ ] **Step 4: Commit**

```bash
git add internal/renderer/markdown.go
git commit -m "Wire goldmark extensions into renderer, remove post-processing block"
```

---

### Task 6: Verify Byte-Identical Output

This task validates that the new extensions produce the same HTML as the old post-processors for representative inputs.

**Files:** None (verification only)

- [ ] **Step 1: Run the full test suite**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: All tests pass, including the existing `markdown_test.go` integration tests that validate end-to-end rendering.

- [ ] **Step 2: Manual spot-check with testsite**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go run ./cmd/gomddoc serve --root testsite`

Open in browser and verify:

- Headings have anchor links on hover
- Admonition blockquotes render as styled divs
- Hex color codes in backticks render as color chips

- [ ] **Step 3: Run benchmarks to check for performance regression**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make bench`
Expected: No significant regression. AST-based transforms may be slightly faster than regex since they avoid re-parsing HTML.

---

### Task 7: Cleanup and Final Commit

- [ ] **Step 1: Verify deleted files are gone**

Run: `ls internal/renderer/anchors.go internal/renderer/admonition.go internal/renderer/colorchip.go 2>&1`
Expected: All three files report "No such file or directory"

- [ ] **Step 2: Check for stale references**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && grep -r "addHeadingAnchors\|transformAdmonitions\|transformColorChips" internal/`
Expected: No matches

- [ ] **Step 3: Final `make ci`**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: Clean pass

- [ ] **Step 4: Final commit if any cleanup was needed**

Only if steps 1-3 revealed issues that needed fixing.
