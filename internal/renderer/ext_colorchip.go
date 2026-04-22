package renderer

import (
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// KindColorChip is the AST node kind for color chip inline elements.
var KindColorChip = ast.NewNodeKind("ColorChip")

// ColorChipNode represents a hex color code rendered as a <color-chip> element.
// It replaces a code span whose sole content is a hex color code.
type ColorChipNode struct {
	ast.BaseInline
	// HexColor is the color value including the # prefix (e.g., "#FF5733").
	HexColor string
}

// Kind returns KindColorChip.
func (n *ColorChipNode) Kind() ast.NodeKind { return KindColorChip }

// Dump implements ast.Node.
func (n *ColorChipNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"HexColor": n.HexColor,
	}, nil)
}

// hexColorPattern matches a hex color code (3-digit or 6-digit).
var hexColorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`)

// ColorChipExtension is a goldmark extension that transforms backtick-wrapped
// hex color codes into <color-chip> custom elements.
//
// It transforms code spans like `#FF5733` into <color-chip>#FF5733</color-chip>.
// Only code spans containing exactly a hex color code are transformed.
type ColorChipExtension struct{}

// Extend registers the color chip AST transformer and node renderer.
func (e *ColorChipExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&colorChipTransformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&colorChipRenderer{}, 100),
	))
}

// colorChipTransformer walks the AST looking for code spans with hex color
// content and replaces them with ColorChipNode.
type colorChipTransformer struct{}

// Transform implements parser.ASTTransformer.
func (t *colorChipTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	features := getFeatures(pc)
	if !config.FeatureEnabled("color_chips", features) {
		return
	}

	source := reader.Source()

	// Collect code spans to transform (don't mutate during walk).
	var toTransform []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() != ast.KindCodeSpan {
			return ast.WalkContinue, nil
		}

		// Extract text content from code span children.
		content := codeSpanText(n, source)
		if hexColorPattern.MatchString(content) {
			toTransform = append(toTransform, n)
		}

		return ast.WalkSkipChildren, nil
	})

	for _, cs := range toTransform {
		content := codeSpanText(cs, source)
		chip := &ColorChipNode{HexColor: content}
		parent := cs.Parent()
		parent.ReplaceChild(parent, cs, chip)
	}
}

// codeSpanText extracts the text content from a code span's child text nodes.
func codeSpanText(n ast.Node, source []byte) string {
	var buf []byte
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == ast.KindText {
			seg := c.(*ast.Text).Segment
			buf = append(buf, seg.Value(source)...)
		}
	}
	return string(buf)
}

// colorChipRenderer renders ColorChipNode to HTML.
type colorChipRenderer struct{}

// RegisterFuncs registers the color chip node renderer.
func (r *colorChipRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindColorChip, r.renderColorChip)
}

// renderColorChip renders a ColorChipNode as a <color-chip> custom element.
func (r *colorChipRenderer) renderColorChip(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ColorChipNode)
		_, _ = w.WriteString("<color-chip>")
		_, _ = w.WriteString(n.HexColor)
		_, _ = w.WriteString("</color-chip>")
	}
	return ast.WalkContinue, nil
}
