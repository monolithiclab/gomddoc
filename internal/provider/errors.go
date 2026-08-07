package provider

import (
	"errors"
	"fmt"
	"io/fs"
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

	// ErrProviderClosed is returned when an operation is attempted on a
	// provider that has already been closed.
	ErrProviderClosed = errors.New("provider closed")

	// ErrEmptyDefaultIndex is returned when a provider is constructed
	// with an empty default index filename.
	ErrEmptyDefaultIndex = errors.New("defaultIndex must not be empty")
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

// io/fs operation names, spelled as os.dirFS spells them so a caller switching
// on (*fs.PathError).Op gets the same answer whichever provider backs the site.
// OverlayFS and gitTreeFS disagreed before this: ReadFile reported "open" in one
// and "read" in the other.
const (
	opOpen     = "open"
	opReadFile = "readfile"
	opStat     = "stat"
	opReadDir  = "readdir"
)

// fsPathErr builds the *fs.PathError io/fs requires of every method taking a
// name. It is the one construction for the whole package — the fs.FS
// implementations here used to mix a bare sentinel, a raw wrapped-library
// error, and hand-written literals, so errors.As found no path to report.
//
// Not to be confused with the exported PathError above: that one is the
// provider API's own error type and carries no io/fs meaning.
func fsPathErr(op, name string, err error) *fs.PathError {
	return &fs.PathError{Op: op, Path: name, Err: err}
}
