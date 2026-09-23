package server

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
)

// URLRedirectMap maps source paths to target paths for URL redirects.
type URLRedirectMap map[string]string

// BuildRedirectMap scans the metadata index for pages with redirect_from
// frontmatter and builds a reverse mapping from old URLs to current page paths.
// When a resolver is provided, redirect targets use clean (extensionless) paths
// to avoid double redirects (old-url → /page.md → /page).
//
// basePath is prepended to the targets only. Per-language handlers are mounted
// under /{lang} with the prefix already stripped, so the sources stay
// content-root-relative while the targets must be absolute site paths. Pass ""
// for the default language.
//
// It also returns what it had to ignore or could not honour, as findings for
// the caller to log: a redirect_from that is not a list of paths, a source two
// pages claim (the later page wins), and a source that is itself a page — the
// handler consults redirects before content, so that page becomes unreachable.
func BuildRedirectMap(index *metadata.Index, resolver *resolve.PathResolver, basePath string) (URLRedirectMap, []diag.Finding) {
	if index == nil {
		return nil, nil
	}

	redirects := make(URLRedirectMap)
	claimedBy := map[string]string{} // source → file of the page that claimed it
	var findings []diag.Finding

	for _, page := range index.AllPages() {
		fromRaw, ok := page.Meta["redirect_from"]
		if !ok {
			continue
		}
		file := strings.TrimPrefix(page.Path, "/")

		sources, ok := fromRaw.([]any)
		if !ok {
			findings = append(findings, diag.New("content.frontmatter-type", file, 0, "redirect_from",
				"redirect_from must be a list of paths; this one is ignored",
				"write it as a list: redirect_from: [/old/path]"))
			continue
		}

		target := page.Path
		if resolver != nil {
			cleanPath := strings.TrimPrefix(target, "/")
			if clean, found := resolver.CleanPath(cleanPath); found {
				target = "/" + clean
			}
		}
		target = basePath + target

		for _, src := range sources {
			s, ok := src.(string)
			if !ok || s == "" {
				findings = append(findings, diag.New("content.frontmatter-type", file, 0, "redirect_from",
					fmt.Sprintf("redirect_from entry %v is not a path and is ignored", src),
					"list URL paths as strings, e.g. /old/path"))
				continue
			}
			if first, dup := claimedBy[s]; dup && first != file {
				findings = append(findings, diag.New("content.redirect-conflict", file, 0, s,
					fmt.Sprintf("%s is also claimed by %s; this page wins", s, first),
					"keep the source in only one page's redirect_from"))
			}
			if isPage(index, resolver, s) {
				findings = append(findings, diag.New("content.redirect-conflict", file, 0, s,
					fmt.Sprintf("%s is an existing page; redirecting it makes that page unreachable", s),
					"remove the source, or move the page it names"))
			}
			claimedBy[s] = file
			redirects[s] = target
		}
	}

	if len(redirects) == 0 {
		return nil, findings
	}

	return redirects, findings
}

// isPage reports whether a redirect source names a real page: an indexed page
// by file path, or — the resolver knowing every renderable file, frontmatter
// or not — a page by clean URL or by file path.
func isPage(index *metadata.Index, resolver *resolve.PathResolver, source string) bool {
	if index.ByPath(source) != nil {
		return true
	}
	if resolver == nil {
		return false
	}
	p := strings.TrimPrefix(source, "/")
	if _, found := resolver.Resolve(p); found {
		return true
	}
	_, found := resolver.CleanPath(p)
	return found
}

// stripPathPrefix returns middleware that removes a leading path prefix
// (e.g. "/fr-FR") from the request URL before invoking the next handler. Used
// to mount per-language handlers whose provider/resolver are rooted at the
// language subdirectory and therefore expect content-root-relative paths.
func stripPathPrefix(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.StripPrefix(prefix, next)
	}
}

// ExtensionRedirect returns middleware that 301-redirects requests with
// strippable extensions to their canonical extensionless URL.
//
// basePath is prepended to the redirect Location, allowing per-language
// handlers (mounted under /{lang} with the prefix already stripped) to emit
// language-prefixed redirect targets. Pass "" for the default language.
func ExtensionRedirect(resolver *resolve.PathResolver, stripExts []string, basePath string) func(http.Handler) http.Handler {
	// Build a set for fast lookup.
	extSet := make(map[string]bool, len(stripExts))
	for _, ext := range stripExts {
		extSet[ext] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resolver == nil || len(extSet) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ext := path.Ext(r.URL.Path)
			if !extSet[ext] {
				next.ServeHTTP(w, r)
				return
			}

			// Strip leading "/" to get the fs-relative path.
			realPath := strings.TrimPrefix(r.URL.Path, "/")
			if cleanPath, found := resolver.CleanPath(realPath); found {
				http.Redirect(w, r, basePath+"/"+cleanPath, http.StatusMovedPermanently) // #nosec G710 -- cleanPath is from resolver's validated map, not user input
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// redirectTemplate is the HTML template for redirect pages.
// Parsed once at init to avoid per-call overhead.
var redirectTemplate = template.Must(template.New("redirect").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="0; url={{.}}">
<link rel="canonical" href="{{.}}">
<title>Redirect</title>
</head>
<body>
<p>This page has moved to <a href="{{.}}">{{.}}</a>.</p>
</body>
</html>
`))

// GenerateRedirectHTML produces a minimal HTML page that redirects to targetURL
// via meta refresh. Compatible with all static hosts. The URL is HTML-escaped
// to prevent XSS via crafted redirect targets.
func GenerateRedirectHTML(targetURL string) []byte {
	var buf bytes.Buffer
	_ = redirectTemplate.Execute(&buf, targetURL) // #nosec G104 -- template is static, cannot fail
	return buf.Bytes()
}
