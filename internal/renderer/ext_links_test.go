package renderer

import (
	"context"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/resolve"
)

// linkFiles is the content tree the page-link tests resolve against: Markdown
// pages, a directory page and a non-page file.
var linkFiles = fstest.MapFS{
	"README.md":           {Data: []byte("# Home\n")},
	"x.md":                {Data: []byte("# X\n")},
	"guide/intro.md":      {Data: []byte("# Intro\n")},
	"guide/other.md":      {Data: []byte("# Other\n")},
	"guide/my page.md":    {Data: []byte("# Spaced\n")},
	"guide/sub/README.md": {Data: []byte("# Sub\n")},
	"guide/image.png":     {Data: []byte("png")},
}

func linkResolver(trailingSlash bool) *resolve.PathResolver {
	return resolve.Build(linkFiles, resolve.BuildOptions{
		StripExtensions: []string{".md"},
		HasRenderer:     func(m string) bool { return m == "text/markdown" },
		TrailingSlash:   trailingSlash,
	})
}

const linkBody = `[a](other.md)
[b](./sub/README.md)
[c](../x.md#frag)
[d](other.md?v=1#s)
[e](https://e.com/a.md)
[f](nope.md)
[g](image.png)
[h](#local)
[i](/x.md)
[j](../../escape.md)
[k](my%20page.md)
[l][ref]

[ref]: other.md#r
`

var hrefRe = regexp.MustCompile(`<a href="([^"]*)">([a-z])</a>`)

// renderedHrefs renders linkBody under links and maps each link's text to its href.
func renderedHrefs(t *testing.T, links *PageLinks) map[string]string {
	t.Helper()
	ctx := context.Background()
	if links != nil {
		ctx = WithPageLinks(ctx, *links)
	}
	res, err := NewMarkdownRenderer(MarkdownOptions{}).Render(ctx, []byte(linkBody), nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := map[string]string{}
	for _, m := range hrefRe.FindAllStringSubmatch(string(res.Content), -1) {
		got[m[2]] = m[1]
	}
	return got
}

func assertHrefs(t *testing.T, got, want map[string]string) {
	t.Helper()
	for text, href := range want {
		if got[text] != href {
			t.Errorf("link %s: href = %q, want %q", text, got[text], href)
		}
	}
}

// untouched are the hrefs every configuration leaves as written: external,
// missing, non-page, fragment-only and outside the content root.
var untouched = map[string]string{
	"e": "https://e.com/a.md",
	"f": "nope.md",
	"g": "image.png",
	"h": "#local",
	"j": "../../escape.md",
}

func TestPageLinks_RewritesMarkdownLinksToCleanURLs(t *testing.T) {
	t.Parallel()

	got := renderedHrefs(t, &PageLinks{
		Resolver: linkResolver(false), DefaultIndex: "README.md", SourceDir: "guide",
	})
	assertHrefs(t, got, map[string]string{
		"a": "/guide/other",
		"b": "/guide/sub",
		"c": "/x#frag",
		"d": "/guide/other?v=1#s",
		"i": "/x",
		"k": "/guide/my%20page",
		"l": "/guide/other#r",
	})
	assertHrefs(t, got, untouched)
}

func TestPageLinks_LanguagePrefix(t *testing.T) {
	t.Parallel()

	got := renderedHrefs(t, &PageLinks{
		Resolver: linkResolver(false), DefaultIndex: "README.md", SourceDir: "guide", LangPrefix: "/fr-FR",
	})
	assertHrefs(t, got, map[string]string{
		"a": "/fr-FR/guide/other",
		"c": "/fr-FR/x#frag",
		// A root-relative link outside the language names another tree.
		"i": "/x.md",
	})
	assertHrefs(t, got, untouched)
}

func TestPageLinks_LanguageAbsoluteLink(t *testing.T) {
	t.Parallel()

	ctx := WithPageLinks(context.Background(), PageLinks{
		Resolver: linkResolver(false), DefaultIndex: "README.md", LangPrefix: "/fr-FR",
	})
	res, err := NewMarkdownRenderer(MarkdownOptions{}).Render(ctx, []byte("[a](/fr-FR/x.md)"), nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if want := `<a href="/fr-FR/x">a</a>`; !regexp.MustCompile(regexp.QuoteMeta(want)).Match(res.Content) {
		t.Errorf("rendered %q, want %s", res.Content, want)
	}
}

func TestPageLinks_TrailingSlash(t *testing.T) {
	t.Parallel()

	got := renderedHrefs(t, &PageLinks{
		Resolver: linkResolver(true), DefaultIndex: "README.md", SourceDir: "guide",
	})
	assertHrefs(t, got, map[string]string{
		"a": "/guide/other/",
		"b": "/guide/sub/",
		"c": "/x/#frag",
	})
}

// Without page links (the Markdown passthrough's view, or a caller that has
// no resolver) every destination stays the author's.
func TestPageLinks_AbsentLeavesLinks(t *testing.T) {
	t.Parallel()

	got := renderedHrefs(t, nil)
	assertHrefs(t, got, map[string]string{"a": "other.md", "c": "../x.md#frag", "i": "/x.md"})
}
