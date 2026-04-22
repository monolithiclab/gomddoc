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
