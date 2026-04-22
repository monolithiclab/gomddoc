package provider

import "errors"

var (
	// ErrDirListingDisabled is returned when directory listing is requested
	// but the DirIndex configuration option is set to false.
	// This error should result in HTTP 403 Forbidden.
	ErrDirListingDisabled = errors.New("directory listing disabled")

	// ErrNotFound is returned when a requested resource does not exist.
	// This error should result in HTTP 404 Not Found.
	ErrNotFound = errors.New("not found")
)
