// Package breadcrumb derives the ancestor trail for a page path.
//
// Deciding whether the target is a directory costs a provider Stat — the
// intermediate segments are taken as directories without one — and both the
// breadcrumb bar and the JSON-LD partial want the trail, so it is built once
// into PageContext rather than by each consumer.
package breadcrumb

// Breadcrumb represents a single breadcrumb item in the navigation trail
type Breadcrumb struct {
	Path  string // URL path for the breadcrumb link
	Label string // Display label for the breadcrumb
}

// Generator defines the interface for generating breadcrumbs
type Generator interface {
	// Generate creates breadcrumbs for the given path
	Generate(path string) []Breadcrumb
}
