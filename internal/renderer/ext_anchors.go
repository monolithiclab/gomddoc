package renderer

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// HeadingAnchorExtension is a goldmark extension that appends
// <gmd-heading-anchor> web components to headings that have an id attribute.
// The web component renders a "#" anchor link with Shadow DOM encapsulation.
//
// This replaces the regex-based addHeadingAnchors post-processor with a
// proper AST-level renderer override.
type HeadingAnchorExtension struct{}

// Extend registers the heading anchor renderer with goldmark.
func (e *HeadingAnchorExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&headingAnchorRenderer{}, 100),
	))
}

// headingAnchorRenderer overrides goldmark's default heading rendering
// to append <gmd-heading-anchor> web components when heading_anchors
// feature is enabled.
type headingAnchorRenderer struct{}

// RegisterFuncs registers the heading renderer override.
func (r *headingAnchorRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
}

// renderHeading replicates goldmark's default heading rendering and appends
// a <gmd-heading-anchor> web component when the heading has an ID and the
// heading_anchors feature is enabled.
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
	} else {
		// Append web component if heading has an ID and feature is enabled.
		features := getDocFeatures(node)
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

		_, _ = w.WriteString("</h")
		_ = w.WriteByte("0123456"[n.Level])
		_, _ = w.WriteString(">\n")
	}

	return ast.WalkContinue, nil
}
