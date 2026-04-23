package seo

import (
	"net/url"
	"path"
	"strings"
	"sync/atomic"
)

// parsedDomain caches a parsed domain alongside its raw input so PageURL
// can skip url.Parse on a hit. Single-entry — multi-domain processes
// degrade gracefully to per-call parse.
type parsedDomain struct {
	raw  string
	base url.URL
}

var domainCache atomic.Pointer[parsedDomain]

// PageURL constructs a full URL from domain and page path.
// Returns "" if domain is empty.
// Uses defaultIndex to strip trailing index files (e.g., "README.md") from the path.
// Domain may or may not include a scheme — defaults to "https://" if missing.
func PageURL(domain, pagePath, defaultIndex string) string {
	if domain == "" {
		return ""
	}

	base, ok := lookupBase(domain)
	if !ok {
		return ""
	}

	clean := normalizePagePath(pagePath, defaultIndex)
	base.Path = path.Join(base.Path, clean)

	// Ensure directory paths end with /
	if clean == "/" || strings.HasSuffix(clean, "/") {
		if !strings.HasSuffix(base.Path, "/") {
			base.Path += "/"
		}
	}

	return base.String()
}

// lookupBase returns a value copy of the parsed base URL for domain, so the
// caller may safely mutate fields like Path without affecting the cache.
func lookupBase(domain string) (url.URL, bool) {
	if cached := domainCache.Load(); cached != nil && cached.raw == domain {
		return cached.base, true
	}
	parsed, err := url.Parse(normalizeDomain(domain))
	if err != nil {
		return url.URL{}, false
	}
	domainCache.Store(&parsedDomain{raw: domain, base: *parsed})
	return *parsed, true
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
