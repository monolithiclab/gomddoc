package renderer

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// KindAdmonition is the AST node kind for admonition blocks.
var KindAdmonition = ast.NewNodeKind("Admonition")

// AdmonitionNode represents a GitHub-style admonition block (NOTE, TIP, etc.).
// It replaces a blockquote whose first paragraph starts with [!TYPE].
type AdmonitionNode struct {
	ast.BaseBlock
	// AdmonitionType is the lowercase canonical name (e.g., "note", "warning").
	AdmonitionType string
	// Title is the display title (e.g., "Note", "Warning").
	Title string
}

// Kind returns KindAdmonition.
func (n *AdmonitionNode) Kind() ast.NodeKind { return KindAdmonition }

// Dump implements ast.Node.
func (n *AdmonitionNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Type":  n.AdmonitionType,
		"Title": n.Title,
	}, nil)
}

// admonitionTypes maps uppercase marker names to their display properties.
var admonitionTypes = map[string]struct {
	Name  string
	Title string
}{
	"NOTE":      {Name: "note", Title: "Note"},
	"TIP":       {Name: "tip", Title: "Tip"},
	"IMPORTANT": {Name: "important", Title: "Important"},
	"WARNING":   {Name: "warning", Title: "Warning"},
	"CAUTION":   {Name: "caution", Title: "Caution"},
}

// AdmonitionExtension is a goldmark extension that transforms GitHub-style
// admonition blockquotes into custom HTML elements.
//
// It transforms blockquotes like:
//
//	> [!NOTE]
//	> Content here
//
// Into:
//
//	<gmd-admonition type="note" title="Note">
//	<p>Content here</p></gmd-admonition>
type AdmonitionExtension struct{}

// Extend registers the admonition AST transformer and node renderer.
func (e *AdmonitionExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&admonitionTransformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&admonitionRenderer{}, 100),
	))
}

// admonitionTransformer walks the AST looking for blockquotes with [!TYPE]
// markers and replaces them with AdmonitionNode.
type admonitionTransformer struct{}

// Transform implements parser.ASTTransformer.
func (t *admonitionTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	features := getFeatures(pc)
	if !config.FeatureEnabled("admonitions", features) {
		return
	}

	source := reader.Source()

	// Collect blockquotes to transform (don't mutate during iteration).
	var toTransform []ast.Node
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		t.collectBlockquotes(c, &toTransform)
	}

	for _, bq := range toTransform {
		t.transformBlockquote(bq, source)
	}
}

// collectBlockquotes recursively collects all blockquote nodes in the tree.
func (t *admonitionTransformer) collectBlockquotes(n ast.Node, result *[]ast.Node) {
	if n.Kind() == ast.KindBlockquote {
		*result = append(*result, n)
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		t.collectBlockquotes(c, result)
	}
}

// transformBlockquote checks if a blockquote has the [!TYPE] marker pattern
// and replaces it with an AdmonitionNode if so.
//
// The AST for `> [!NOTE]\n> Content` looks like:
//
//	Blockquote
//	  Paragraph
//	    Text "[" (softBreak=false)
//	    Text "!NOTE" (softBreak=false)
//	    Text "]" (softBreak=true)
//	    Text "Content" (softBreak=false)
func (t *admonitionTransformer) transformBlockquote(bq ast.Node, source []byte) {
	firstPara := bq.FirstChild()
	if firstPara == nil || firstPara.Kind() != ast.KindParagraph {
		return
	}

	// Check for [!TYPE] pattern: exactly "[", "!TYPE", "]" as the first three text nodes.
	first := firstPara.FirstChild()
	if first == nil || first.Kind() != ast.KindText {
		return
	}
	firstText := first.(*ast.Text)
	if string(firstText.Segment.Value(source)) != "[" {
		return
	}

	second := first.NextSibling()
	if second == nil || second.Kind() != ast.KindText {
		return
	}
	secondText := second.(*ast.Text)
	markerValue := string(secondText.Segment.Value(source))
	if !strings.HasPrefix(markerValue, "!") {
		return
	}
	typeKey := strings.ToUpper(markerValue[1:])

	third := second.NextSibling()
	if third == nil || third.Kind() != ast.KindText {
		return
	}
	thirdText := third.(*ast.Text)
	if string(thirdText.Segment.Value(source)) != "]" {
		return
	}

	adm, ok := admonitionTypes[typeKey]
	if !ok {
		return
	}

	// Build the AdmonitionNode.
	node := &AdmonitionNode{
		AdmonitionType: adm.Name,
		Title:          adm.Title,
	}

	// Remove the [!TYPE] marker text nodes from the first paragraph.
	firstPara.RemoveChild(firstPara, first)
	firstPara.RemoveChild(firstPara, second)
	firstPara.RemoveChild(firstPara, third)

	// Move all children from blockquote to admonition node.
	var children []ast.Node
	for c := bq.FirstChild(); c != nil; c = c.NextSibling() {
		children = append(children, c)
	}
	for _, c := range children {
		bq.RemoveChild(bq, c)
		// Skip empty paragraphs (marker-only case).
		if c.Kind() == ast.KindParagraph && !c.HasChildren() {
			continue
		}
		node.AppendChild(node, c)
	}

	// Replace blockquote with admonition in the parent.
	parent := bq.Parent()
	parent.ReplaceChild(parent, bq, node)
}

// admonitionRenderer renders AdmonitionNode to HTML.
type admonitionRenderer struct{}

// RegisterFuncs registers the admonition node renderer.
func (r *admonitionRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindAdmonition, r.renderAdmonition)
}

// renderAdmonition renders an AdmonitionNode as a custom HTML element.
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
