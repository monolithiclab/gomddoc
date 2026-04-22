package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

func TestClassifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "ErrDirListingDisabled",
			err:  provider.ErrDirListingDisabled,
			want: http.StatusForbidden,
		},
		{
			name: "ErrDirListingDisabled wrapped",
			err:  fmt.Errorf("failed to list directory: %w", provider.ErrDirListingDisabled),
			want: http.StatusForbidden,
		},
		{
			name: "ErrNotFound",
			err:  provider.ErrNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "ErrNotFound wrapped",
			err:  fmt.Errorf("resource not available: %w", provider.ErrNotFound),
			want: http.StatusNotFound,
		},
		{
			name: "os.ErrNotExist",
			err:  os.ErrNotExist,
			want: http.StatusNotFound,
		},
		{
			name: "os.ErrNotExist wrapped",
			err:  fmt.Errorf("file read failed: %w", os.ErrNotExist),
			want: http.StatusNotFound,
		},
		{
			name: "fs.ErrPermission",
			err:  fs.ErrPermission,
			want: http.StatusForbidden,
		},
		{
			name: "fs.ErrPermission wrapped",
			err:  fmt.Errorf("access denied: %w", fs.ErrPermission),
			want: http.StatusForbidden,
		},
		{
			name: "context.Canceled",
			err:  context.Canceled,
			want: 499,
		},
		{
			name: "context.Canceled wrapped",
			err:  fmt.Errorf("operation aborted: %w", context.Canceled),
			want: 499,
		},
		{
			name: "context.DeadlineExceeded",
			err:  context.DeadlineExceeded,
			want: http.StatusGatewayTimeout,
		},
		{
			name: "context.DeadlineExceeded wrapped",
			err:  fmt.Errorf("timeout occurred: %w", context.DeadlineExceeded),
			want: http.StatusGatewayTimeout,
		},
		{
			name: "generic error",
			err:  errors.New("something went wrong"),
			want: http.StatusInternalServerError,
		},
		{
			name: "nil error",
			err:  nil,
			want: http.StatusInternalServerError,
		},
		// Git provider errors
		{
			name: "ErrInvalidGitURL",
			err:  provider.ErrInvalidGitURL,
			want: http.StatusBadRequest,
		},
		{
			name: "ErrInvalidGitURL wrapped",
			err:  fmt.Errorf("failed to parse URL: %w", provider.ErrInvalidGitURL),
			want: http.StatusBadRequest,
		},
		{
			name: "ErrGitAuthFailed",
			err:  provider.ErrGitAuthFailed,
			want: http.StatusUnauthorized,
		},
		{
			name: "ErrGitAuthFailed wrapped",
			err:  fmt.Errorf("SSH key not found: %w", provider.ErrGitAuthFailed),
			want: http.StatusUnauthorized,
		},
		{
			name: "ErrGitConnectFailed",
			err:  provider.ErrGitConnectFailed,
			want: http.StatusBadGateway,
		},
		{
			name: "ErrGitConnectFailed wrapped",
			err:  fmt.Errorf("network error: %w", provider.ErrGitConnectFailed),
			want: http.StatusBadGateway,
		},
		{
			name: "ErrGitRefNotFound",
			err:  provider.ErrGitRefNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "ErrGitRefNotFound wrapped",
			err:  fmt.Errorf("branch main not found: %w", provider.ErrGitRefNotFound),
			want: http.StatusNotFound,
		},
		{
			name: "ErrGitLFSNotSupported",
			err:  provider.ErrGitLFSNotSupported,
			want: http.StatusNotImplemented,
		},
		{
			name: "ErrGitLFSNotSupported wrapped",
			err:  fmt.Errorf("file is LFS pointer: %w", provider.ErrGitLFSNotSupported),
			want: http.StatusNotImplemented,
		},
		{
			name: "ErrFileTooLarge",
			err:  provider.ErrFileTooLarge,
			want: http.StatusRequestEntityTooLarge,
		},
		{
			name: "ErrFileTooLarge wrapped",
			err:  fmt.Errorf("100MB exceeds 50MB limit: %w", provider.ErrFileTooLarge),
			want: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyError(tt.err)
			if got != tt.want {
				t.Errorf("classifyError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
