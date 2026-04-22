package seo

import (
	"net/url"
	"path"
	"strings"
)

// PageURL constructs a full URL from domain and page path.
// Returns "" if domain is empty.
// Uses defaultIndex to strip trailing index files (e.g., "README.md") from the path.
// Domain may or may not include a scheme — defaults to "https://" if missing.
func PageURL(domain, pagePath, defaultIndex string) string {
	if domain == "" {
		return ""
	}

	base := normalizeDomain(domain)

	// Strip trailing default index file from path
	clean := normalizePagePath(pagePath, defaultIndex)

	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	u.Path = path.Join(u.Path, clean)

	// Ensure directory paths end with /
	if clean == "/" || strings.HasSuffix(clean, "/") {
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
	}

	return u.String()
}

// normalizeDomain ensures a domain has a scheme prefix.
func normalizeDomain(domain string) string {
	domain = strings.TrimRight(domain, "/")
	if !strings.Contains(domain, "://") {
		return "https://" + domain
	}
	return domain
}

// normalizePagePath cleans a page path by stripping the default index filename.
// Only strips when preceded by "/" to avoid false positives (e.g., "/notaREADME.md").
func normalizePagePath(pagePath, defaultIndex string) string {
	if defaultIndex != "" {
		pagePath = strings.TrimSuffix(pagePath, "/"+defaultIndex)
	}
	if pagePath == "" || pagePath == "." {
		pagePath = "/"
	}
	return pagePath
}
