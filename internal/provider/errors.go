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

	// ErrInvalidGitURL is returned when a git:// URL is malformed or uses
	// an unsupported scheme. This error should result in HTTP 400 Bad Request.
	ErrInvalidGitURL = errors.New("invalid git URL")

	// ErrGitAuthFailed is returned when Git authentication fails.
	// Common causes: SSH agent not running, no suitable key found.
	// This error should result in HTTP 401 Unauthorized.
	ErrGitAuthFailed = errors.New("git authentication failed")

	// ErrGitConnectFailed is returned when the remote Git server is unreachable.
	// Common causes: network issues, firewall, wrong hostname.
	// This error should result in HTTP 502 Bad Gateway.
	ErrGitConnectFailed = errors.New("git connection failed")

	// ErrGitRefNotFound is returned when a branch, tag, or commit hash
	// does not exist in the repository.
	// This error should result in HTTP 404 Not Found.
	ErrGitRefNotFound = errors.New("git reference not found")

	// ErrGitLFSNotSupported is returned when a file is a Git LFS pointer.
	// The Git provider does not support fetching LFS content.
	// This error should result in HTTP 501 Not Implemented.
	ErrGitLFSNotSupported = errors.New("git LFS not supported")

	// ErrFileTooLarge is returned when a file exceeds the maximum allowed size.
	// This error should result in HTTP 413 Request Entity Too Large.
	ErrFileTooLarge = errors.New("file too large")
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
