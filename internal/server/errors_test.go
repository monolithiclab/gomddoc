package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

// TestErrorPage_FallsBackToPlainText covers the degraded paths. A response that
// cannot be produced is worse than an ugly one, so a nil writer, a missing
// renderer, and a theme with no error.html.tmpl all still answer — and they must
// answer with the right status, since that is all a client has left.
func TestErrorPage_FallsBackToPlainText(t *testing.T) {
	t.Parallel()

	noErrorLayout := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(`<html></html>`)},
	}
	siteConfig := config.NewSiteConfig(".")

	tests := []struct {
		name string
		page *ErrorPage
	}{
		{"nil writer", nil},
		{"no renderer", NewErrorPage(nil, tmpl.ErrorContextInput{Site: &siteConfig})},
		{"no site config", NewErrorPage(setupTestRenderer(), tmpl.ErrorContextInput{})},
		{"theme without error layout", NewErrorPage(
			tmpl.NewHTMLRenderer(&siteConfig, noErrorLayout),
			tmpl.ErrorContextInput{Site: &siteConfig},
		)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			tt.page.Write(w, httptest.NewRequest("GET", "/missing", nil), http.StatusForbidden, "/missing")

			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", w.Code)
			}
			if got, want := w.Body.String(), "403 Forbidden"; got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
		})
	}
}

// TestErrorPage_NotFoundUsesRequestPath pins the one difference between the two
// entry points: NotFound reports the URL, while Write takes the path explicitly
// because the content handler resolves a clean URL to a real file before failing.
func TestErrorPage_NotFoundUsesRequestPath(t *testing.T) {
	t.Parallel()

	siteConfig := config.NewSiteConfig(".")
	page := NewErrorPage(
		tmpl.NewHTMLRenderer(&siteConfig, fstest.MapFS{
			"assets/themes/default/layouts/error.html.tmpl": {Data: []byte(`{{ .Page.Path }}`)},
		}),
		tmpl.ErrorContextInput{Site: &siteConfig},
	)

	w := httptest.NewRecorder()
	page.NotFound(w, httptest.NewRequest("GET", "/guide/intro", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if got := w.Body.String(); got != "/guide/intro" {
		t.Errorf("rendered path = %q, want %q", got, "/guide/intro")
	}
}

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
		{
			name: "ErrNoRenderer",
			err:  renderer.ErrNoRenderer,
			want: http.StatusUnsupportedMediaType,
		},
		{
			name: "ErrNoRenderer wrapped",
			err:  fmt.Errorf("no handler for type: %w", renderer.ErrNoRenderer),
			want: http.StatusUnsupportedMediaType,
		},
		{
			name: "ErrNoMatchingRenderer",
			err:  renderer.ErrNoMatchingRenderer,
			want: http.StatusNotAcceptable,
		},
		{
			name: "ErrNoMatchingRenderer wrapped",
			err:  fmt.Errorf("output type mismatch: %w", renderer.ErrNoMatchingRenderer),
			want: http.StatusNotAcceptable,
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
