package server

// Content-Type values returned by handlers in this package.
const (
	mimeHTML  = "text/html; charset=utf-8"
	mimeJSON  = "application/json"
	mimeXML   = "application/xml; charset=utf-8"
	mimeAtom  = "application/atom+xml; charset=utf-8"
	mimePlain = "text/plain; charset=utf-8"
)

// Cache-Control header values used across handlers.
//
// cacheDynamic applies to rendered HTML, sitemaps, and feeds — content that
// can change between deploys but is OK to serve from edge caches briefly.
//
// cacheImmutable applies to fingerprinted static assets that never change
// once published (one-year max-age + immutable disables revalidation).
const (
	cacheDynamic   = "public, max-age=300"
	cacheImmutable = "public, max-age=31536000, immutable"
)
