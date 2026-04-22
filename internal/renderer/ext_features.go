package renderer

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// featuresContextKey is used to pass feature flags through the parser context
// to AST transformers. Transformers have access to parser.Context but not to
// the renderer's configuration directly.
var featuresContextKey = parser.NewContextKey()

// featuresDocAttr is the attribute key used to store feature flags on the
// document node. Node renderers only have access to ast.Node, so they read
// features from the document's attributes via OwnerDocument().
const featuresDocAttr = "gomddoc:features"

// getFeatures reads feature flags from the parser context.
// Returns nil if no features have been set.
func getFeatures(pc parser.Context) map[string]bool {
	v := pc.Get(featuresContextKey)
	if v == nil {
		return nil
	}
	features, ok := v.(map[string]bool)
	if !ok {
		return nil
	}
	return features
}

// setDocFeatures stores feature flags on the document node as an attribute,
// making them accessible to node renderers via getDocFeatures.
func setDocFeatures(doc *ast.Document, features map[string]bool) {
	doc.SetAttributeString(featuresDocAttr, features)
}

// getDocFeatures reads feature flags from the document node that owns the
// given AST node. Returns nil if the node has no owner document or no
// features have been set.
func getDocFeatures(node ast.Node) map[string]bool {
	doc := node.OwnerDocument()
	if doc == nil {
		return nil
	}
	v, ok := doc.AttributeString(featuresDocAttr)
	if !ok {
		return nil
	}
	features, ok := v.(map[string]bool)
	if !ok {
		return nil
	}
	return features
}
