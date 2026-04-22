package server

import "github.com/monolithiclab/gomddoc/internal/negotiate"

// MediaType is an alias for negotiate.MediaType for backward compatibility
// within the server package.
type MediaType = negotiate.MediaType

// ParseAccept delegates to negotiate.ParseAccept.
func ParseAccept(acceptHeader string) []MediaType {
	return negotiate.ParseAccept(acceptHeader)
}
