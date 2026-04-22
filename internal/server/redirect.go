package server

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
)

// URLRedirectMap maps source paths to target paths for URL redirects.
type URLRedirectMap map[string]string

// BuildRedirectMap scans the metadata index for pages with redirect_from
// frontmatter and builds a reverse mapping from old URLs to current page paths.
func BuildRedirectMap(index *metadata.Index) URLRedirectMap {
	if index == nil {
		return nil
	}

	redirects := make(URLRedirectMap)

	for _, page := range index.AllPages() {
		fromRaw, ok := page.Meta["redirect_from"]
		if !ok {
			continue
		}

		sources, ok := fromRaw.([]any)
		if !ok {
			continue
		}

		for _, src := range sources {
			if s, ok := src.(string); ok && s != "" {
				redirects[s] = page.Path
			}
		}
	}

	if len(redirects) == 0 {
		return nil
	}

	return redirects
}

// ExtensionRedirect returns middleware that 301-redirects requests with
// strippable extensions to their canonical extensionless URL.
func ExtensionRedirect(resolver *resolve.PathResolver, stripExts []string) func(http.Handler) http.Handler {
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
				http.Redirect(w, r, "/"+cleanPath, http.StatusMovedPermanently)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GenerateRedirectHTML produces a minimal HTML page that redirects to targetURL
// via meta refresh. Compatible with all static hosts.
func GenerateRedirectHTML(targetURL string) []byte {
	return fmt.Appendf(nil, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="0; url=%[1]s">
<link rel="canonical" href="%[1]s">
<title>Redirect</title>
</head>
<body>
<p>This page has moved to <a href="%[1]s">%[1]s</a>.</p>
</body>
</html>
`, targetURL)
}
