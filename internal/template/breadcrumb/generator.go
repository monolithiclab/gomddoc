package breadcrumb

import (
	"log/slog"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/text"
)

const (
	HomePath  = "/"
	HomeLabel = "Home"
	RootDir   = "."
)

// InfoProvider defines the interface for file information needed by the generator
type InfoProvider interface {
	// IsDir checks if the given path is a directory
	IsDir(path string) bool
}

// DefaultGenerator implements Generator
type DefaultGenerator struct {
	provider InfoProvider
}

// NewGenerator creates a new breadcrumb generator
func NewGenerator(provider InfoProvider) *DefaultGenerator {
	return &DefaultGenerator{
		provider: provider,
	}
}

// Generate creates breadcrumb navigation from a file path
// Returns an ordered slice of breadcrumb items with path and label
// Directories get trailing slashes in URLs, files don't
func (g *DefaultGenerator) Generate(filepath string) []Breadcrumb {
	const (
		initialCapacity = 8
		maxPathLength   = 2048
	)
	breadcrumbs := make([]Breadcrumb, 0, initialCapacity)

	// Always include root
	breadcrumbs = append(breadcrumbs, Breadcrumb{
		Path:  HomePath,
		Label: HomeLabel,
	})

	if len(filepath) > maxPathLength {
		slog.Warn("Path exceeds maximum length for breadcrumb generation",
			slog.Int("length", len(filepath)),
			slog.Int("max", maxPathLength))
		return breadcrumbs
	}

	// Clean the path but keep the full path including filename
	filepath = strings.Trim(path.Clean(filepath), "/")

	// If we're at root, return just the root breadcrumb
	if filepath == RootDir || filepath == "" {
		return breadcrumbs
	}

	// Check if target is a directory
	targetIsDir := g.provider.IsDir("/" + filepath)

	// Split the full path into segments
	segments := strings.Split(filepath, "/")

	// Build cumulative paths efficiently with strings.Builder
	var pathBuilder strings.Builder
	pathBuilder.Grow(len(filepath)) // Pre-allocate capacity

	for i, segment := range segments {
		pathBuilder.WriteByte('/')
		pathBuilder.WriteString(segment)

		currentPath := pathBuilder.String()
		isLastSegment := i == len(segments)-1

		// Determine if this segment should have a trailing slash
		if isLastSegment {
			// For the last segment, use the actual stat result
			if targetIsDir {
				currentPath += "/"
			}
			// If it's a file, no trailing slash
		} else {
			// All intermediate segments are directories - add trailing slash
			currentPath += "/"
		}

		breadcrumbs = append(breadcrumbs, Breadcrumb{
			Path:  currentPath,
			Label: text.TitleCase(strings.TrimSuffix(segment, path.Ext(segment))),
		})
	}

	return breadcrumbs
}
