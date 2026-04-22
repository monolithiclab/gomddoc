package provider

import (
	"errors"
	"fmt"
)

var (
	// ErrDirListingDisabled is returned when directory listing is requested
	// but the DirIndex configuration option is set to false.
	// This error should result in HTTP 403 Forbidden.
	ErrDirListingDisabled = errors.New("directory listing disabled")

	// ErrNotFound is returned when a requested resource does not exist.
	// This error should result in HTTP 404 Not Found.
	ErrNotFound = errors.New("not found")
)

// PathError provides structured error information for provider operations.
// It includes the operation name, the path involved, and the underlying error.
type PathError struct {
	Op   string // Operation that failed (e.g., "read", "stat")
	Path string // Path that caused the error
	Err  error  // Underlying error
}

// Error returns a formatted error message.
func (e *PathError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *PathError) Unwrap() error { return e.Err }
