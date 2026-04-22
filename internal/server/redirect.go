package server

import (
	"fmt"

	"github.com/monolithiclab/gomddoc/internal/metadata"
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
