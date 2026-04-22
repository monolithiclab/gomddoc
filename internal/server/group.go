package server

import (
	"net/http"
	"strings"
)

// RouteGroup registers handlers on an http.ServeMux under a common path
// prefix with shared middleware applied to every handler in the group.
type RouteGroup struct {
	mux        *http.ServeMux
	prefix     string
	middleware []func(http.Handler) http.Handler
}

// NewGroup creates a route group that registers handlers on mux under the
// given path prefix. Middleware are applied outermost-first to every handler.
func NewGroup(mux *http.ServeMux, prefix string, mw ...func(http.Handler) http.Handler) *RouteGroup {
	return &RouteGroup{
		mux:        mux,
		prefix:     prefix,
		middleware: mw,
	}
}

// Subgroup creates a child group. The child inherits the parent's prefix and
// middleware, with its own prefix and middleware appended.
func (g *RouteGroup) Subgroup(prefix string, mw ...func(http.Handler) http.Handler) *RouteGroup {
	combined := make([]func(http.Handler) http.Handler, len(g.middleware)+len(mw))
	copy(combined, g.middleware)
	copy(combined[len(g.middleware):], mw)
	return &RouteGroup{
		mux:        g.mux,
		prefix:     g.prefix + prefix,
		middleware: combined,
	}
}

// Handle registers handler for a Go 1.22+ pattern (e.g. "GET /tags", "/ws").
// The group prefix is prepended to the path portion of the pattern, and all
// group middleware are applied outermost-first.
func (g *RouteGroup) Handle(pattern string, handler http.Handler) {
	h := handler
	for i := len(g.middleware) - 1; i >= 0; i-- {
		h = g.middleware[i](h)
	}
	g.mux.Handle(g.prefixed(pattern), h)
}

// HandleFunc is a convenience that wraps fn as http.Handler before calling Handle.
func (g *RouteGroup) HandleFunc(pattern string, fn http.HandlerFunc) {
	g.Handle(pattern, fn)
}

// prefixed prepends the group prefix to the path portion of a Go 1.22+ pattern.
// "GET /tags" with prefix "/api" becomes "GET /api/tags".
// "/metrics" with prefix "/api" becomes "/api/metrics".
func (g *RouteGroup) prefixed(pattern string) string {
	if method, path, ok := strings.Cut(pattern, " "); ok {
		return method + " " + g.prefix + path
	}
	return g.prefix + pattern
}
