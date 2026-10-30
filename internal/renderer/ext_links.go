package renderer

import (
	"context"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// PageURLs is a language scope's page URL rule (D35), as its template
// renderer (template.Renderer) owns it.
type PageURLs interface {
	// ContentURL is the site URL file is published at, prefix included.
	ContentURL(file string) string
	// MarkdownURL is the site URL of file's Markdown, "" for a non-page.
	MarkdownURL(file string) string
	// LangPrefix is "/{lang}" in a language pipeline, "" otherwise.
	LangPrefix() string
}

// PageLinks is what the HTML render needs to point a body link at a page's
// clean URL (D54): a link written to a page's Markdown file ("other.md",
// "../guide/README.md#setup") would otherwise reach the Markdown URL, which
// serves text/markdown, not the page.
type PageLinks struct {
	// URLs answers which files are pages and their URLs.
	URLs PageURLs
	// SourceDir is the content-root-relative directory relative links
	// resolve against: the rendered file's own directory ("" or "." at root).
	SourceDir string
}

type pageLinksKey struct{}

// WithPageLinks returns ctx carrying links for MarkdownRenderer.Render. It is
// per render, not per renderer: one registry serves every language scope, and
// each render resolves against its own file.
func WithPageLinks(ctx context.Context, links PageLinks) context.Context {
	return context.WithValue(ctx, pageLinksKey{}, links)
}

func pageLinksFrom(ctx context.Context) (PageLinks, bool) {
	links, ok := ctx.Value(pageLinksKey{}).(PageLinks)
	return links, ok && links.URLs != nil
}

var pageLinksContextKey = parser.NewContextKey()

// pageLinkTransformer rewrites link destinations that name a page's Markdown
// file to that page's URL. External links, links to non-page files and links
// that resolve to nothing are left as written.
type pageLinkTransformer struct{}

func (pageLinkTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	links, ok := pc.Get(pageLinksContextKey).(PageLinks)
	if !ok {
		return
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, isLink := n.(*ast.Link); entering && isLink {
			if dest, rewritten := links.pageURL(string(link.Destination)); rewritten {
				link.Destination = []byte(dest)
			}
		}
		return ast.WalkContinue, nil
	})
}

// pageURL returns the page URL dest names, with its query and fragment, or
// false when dest is not a local link to a page's Markdown file.
func (l PageLinks) pageURL(dest string) (string, bool) {
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.Path == "" {
		return "", false
	}

	var realPath string
	if strings.HasPrefix(u.Path, "/") {
		// Root-relative: a site path, so a language page names its own tree
		// through the prefix; anything else is another tree's page.
		p := path.Clean(u.Path)
		if prefix := l.URLs.LangPrefix(); prefix != "" {
			rest, inLang := strings.CutPrefix(p, prefix+"/")
			if !inLang {
				return "", false
			}
			p = rest
		}
		realPath = strings.TrimPrefix(p, "/")
	} else {
		realPath = path.Join(l.SourceDir, u.Path)
		if realPath == ".." || strings.HasPrefix(realPath, "../") {
			return "", false
		}
	}

	if l.URLs.MarkdownURL(realPath) == "" {
		return "", false
	}
	target := url.URL{
		Path:     l.URLs.ContentURL(realPath),
		RawQuery: u.RawQuery,
		Fragment: u.Fragment,
	}
	return target.String(), true
}
