package server

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

// classifyError maps errors to appropriate HTTP status codes.
// It uses errors.Is() to check for specific error types, including
// wrapped errors, ensuring proper error classification throughout
// the application.
func classifyError(err error) int {
	switch {
	case errors.Is(err, provider.ErrDirListingDisabled):
		return http.StatusForbidden // 403
	case errors.Is(err, os.ErrNotExist), errors.Is(err, provider.ErrNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, fs.ErrPermission):
		return http.StatusForbidden // 403
	case errors.Is(err, context.Canceled):
		return 499 // Client closed request (non-standard but widely used)
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout // 504

	// Git provider errors
	case errors.Is(err, provider.ErrInvalidGitURL):
		return http.StatusBadRequest // 400
	case errors.Is(err, provider.ErrGitAuthFailed):
		return http.StatusUnauthorized // 401
	case errors.Is(err, provider.ErrGitConnectFailed):
		return http.StatusBadGateway // 502
	case errors.Is(err, provider.ErrGitRefNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, provider.ErrGitLFSNotSupported):
		return http.StatusNotImplemented // 501
	case errors.Is(err, provider.ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge // 413

	default:
		return http.StatusInternalServerError // 500
	}
}
