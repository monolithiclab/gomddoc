package renderer

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// KindMath is the AST node kind for inline math, KindMathBlock for display math.
var (
	KindMath      = ast.NewNodeKind("Math")
	KindMathBlock = ast.NewNodeKind("MathBlock")
)

// MathNode is inline math written $`…`$; it replaces the code span.
type MathNode struct {
	ast.BaseInline
	// TeX is the expression between the backticks, unescaped.
	TeX string
}

// Kind returns KindMath.
func (n *MathNode) Kind() ast.NodeKind { return KindMath }

// Dump implements ast.Node.
func (n *MathNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"TeX": n.TeX}, nil)
}

// MathBlockNode is display math between $$ markers; its lines hold the TeX.
type MathBlockNode struct {
	ast.BaseBlock
	// closed is set when the closing $$ has been read, so Continue ends the block.
	closed bool
}

// Kind returns KindMathBlock.
func (n *MathBlockNode) Kind() ast.NodeKind { return KindMathBlock }

// IsRaw keeps the TeX out of inline parsing: `_` and `\\` are TeX, not Markdown.
func (n *MathBlockNode) IsRaw() bool { return true }

// Dump implements ast.Node.
func (n *MathBlockNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// MathExtension recognises math in the Markdown source, so only unambiguous
// syntax becomes math and a price like "$1,200" stays text (D52):
//
//   - inline: $`x^2`$, the code span syntax GitHub and GitLab use;
//   - display: a block opened by a line starting with $$ and closed by a line
//     ending with $$ (both markers may share one line).
//
// Both render as <gmd-math> (display math with the display attribute), which
// gmd-math.mjs typesets with KaTeX. With the katex feature off, the syntax is
// left to plain Markdown.
type MathExtension struct{}

// Extend registers the math block parser, the inline transformer and the renderer.
func (e *MathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(&mathBlockParser{}, 90)),
		parser.WithASTTransformers(util.Prioritized(&mathInlineTransformer{}, 100)),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&mathRenderer{}, 100),
	))
}

var mathMarker = []byte("$$")

// mathBlockParser parses $$ display math blocks.
type mathBlockParser struct{}

// Trigger implements parser.BlockParser.
func (b *mathBlockParser) Trigger() []byte { return []byte{'$'} }

// Open implements parser.BlockParser.
func (b *mathBlockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	if !config.FeatureEnabled("katex", getFeatures(pc)) {
		return nil, parser.NoChildren
	}
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || !bytes.HasPrefix(line[pos:], mathMarker) {
		return nil, parser.NoChildren
	}
	node := &MathBlockNode{}
	start := pos + len(mathMarker)
	var rest []byte
	rest, node.closed = cutClosingMarker(line[start:])
	if !util.IsBlank(rest) {
		offset := segment.Start - segment.Padding + start
		node.Lines().Append(text.NewSegment(offset, offset+len(rest)))
	}
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

// Continue implements parser.BlockParser.
func (b *mathBlockParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	n := node.(*MathBlockNode)
	if n.closed {
		return parser.Close
	}
	line, segment := reader.PeekLine()
	if line == nil {
		return parser.Close
	}
	content, closed := cutClosingMarker(line)
	if closed {
		n.closed = true
		segment = segment.WithStop(segment.Stop - (len(line) - len(content)))
	}
	if !util.IsBlank(content) {
		n.Lines().Append(segment)
	}
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

// cutClosingMarker reports whether line ends with the closing $$ (trailing
// whitespace allowed) and returns it without the marker; an open line is
// returned whole.
func cutClosingMarker(line []byte) ([]byte, bool) {
	content := bytes.TrimRight(line, " \t\r\n")
	if !bytes.HasSuffix(content, mathMarker) {
		return line, false
	}
	return content[:len(content)-len(mathMarker)], true
}

// Close implements parser.BlockParser.
func (b *mathBlockParser) Close(ast.Node, text.Reader, parser.Context) {}

// CanInterruptParagraph implements parser.BlockParser: like a code fence,
// a $$ line right after text opens a block.
func (b *mathBlockParser) CanInterruptParagraph() bool { return true }

// CanAcceptIndentedLine implements parser.BlockParser.
func (b *mathBlockParser) CanAcceptIndentedLine() bool { return false }

// mathInlineTransformer turns a code span wrapped in $ signs ($`x`$) into a
// MathNode, trimming the $ from the neighbouring text. Escaping the opening
// sign (\$`x`$) keeps a literal dollar followed by code.
type mathInlineTransformer struct{}

// Transform implements parser.ASTTransformer.
func (t *mathInlineTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	if !config.FeatureEnabled("katex", getFeatures(pc)) {
		return
	}
	source := reader.Source()
	if !bytes.Contains(source, []byte("$`")) {
		return
	}

	var spans []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindCodeSpan {
			spans = append(spans, n)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	// Checked one at a time against the current tree: in $`a`$`b`$ the middle
	// sign closes the first span and can't also open the second.
	for _, cs := range spans {
		prev, okPrev := cs.PreviousSibling().(*ast.Text)
		next, okNext := cs.NextSibling().(*ast.Text)
		if !okPrev || !okNext {
			continue
		}
		before, after := prev.Segment.Value(source), next.Segment.Value(source)
		if !bytes.HasSuffix(before, []byte("$")) || escaped(before[:len(before)-1]) ||
			!bytes.HasPrefix(after, []byte("$")) {
			continue
		}
		prev.Segment = prev.Segment.WithStop(prev.Segment.Stop - 1)
		next.Segment = next.Segment.WithStart(next.Segment.Start + 1)
		parent := cs.Parent()
		parent.ReplaceChild(parent, cs, &MathNode{TeX: codeSpanText(cs, source)})
	}
}

// escaped reports whether the character after prefix is backslash-escaped: an
// odd run of trailing backslashes (\$ is escaped, \\$ is a backslash and a $).
func escaped(prefix []byte) bool {
	n := len(prefix) - len(bytes.TrimRight(prefix, `\`))
	return n%2 == 1
}

// mathRenderer renders math nodes as <gmd-math> elements holding escaped TeX.
type mathRenderer struct{}

// RegisterFuncs implements renderer.NodeRenderer.
func (r *mathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindMath, r.renderMath)
	reg.Register(KindMathBlock, r.renderMathBlock)
}

func (r *mathRenderer) renderMath(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		writeMath(w, "<gmd-math>", []byte(node.(*MathNode).TeX))
	}
	return ast.WalkSkipChildren, nil
}

func (r *mathRenderer) renderMathBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		var tex []byte
		lines := node.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			tex = append(tex, bytes.TrimRight(seg.Value(source), "\r\n")...)
			tex = append(tex, '\n')
		}
		writeMath(w, "<gmd-math display>", bytes.TrimSpace(tex))
		_ = w.WriteByte('\n')
	}
	return ast.WalkSkipChildren, nil
}

// writeMath writes a <gmd-math> element, opened by tag, holding the escaped TeX.
func writeMath(w util.BufWriter, tag string, tex []byte) {
	_, _ = w.WriteString(tag)
	_, _ = w.Write(util.EscapeHTML(tex))
	_, _ = w.WriteString("</gmd-math>")
}
