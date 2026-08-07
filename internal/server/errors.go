package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/template"
)

// ErrorPage renders error responses through the theme's error.html.tmpl.
//
// Every route in a language scope shares one, and that sharing is the point:
// ContentExclusion used to write a plain-text "File not found" while a
// genuinely missing page got the themed page. Both answer 404 by construction —
// that is what exclusion is *for* — so the body was the only thing left that
// could tell a client which of the two it had hit, i.e. the existence leak the
// 404 exists to prevent. The tag routes were a third shape (net/http's default),
// from which the theme's 404 was unreachable entirely.
//
// Deliberate exemptions, each with its reason at the call site: /_assets/
// (a sub-resource fetch), the 406 in Handler.ServeContent (the client just said
// it will not take HTML), MethodFilter's bodyless 405 (a body it does not have
// cannot leak), and the admin listener's mux, which has no renderer to build one
// from. Go's default mux 404 is unreachable on the main listener — server.go
// registers "/" unconditionally, so every path reaches the content pipeline.
//
// The trade is size: a themed error is the theme's full page (~49 KB with the
// default theme's inlined CSS, gzipped on the way out) where the plain-text 404
// was 14 bytes, and ContentExclusion's traffic is largely hidden-path scanning.
// The bytes buy the indistinguishability; memoizing per (scope, status) is the
// mitigation if it ever matters, but it is only sound for themes whose error
// layout ignores Page.Path.
type ErrorPage struct {
	renderer template.Renderer

	// scope is the language scope's half of the error context — Site, Lang,
	// TFunc, Languages. Path, StatusCode and Message are filled per response,
	// so a field added to ErrorContextInput has no second list to update here.
	// Renderer must be the scope's own; see LangPipelineConfig.TemplateRenderer.
	scope template.ErrorContextInput
}

// NewErrorPage builds the error writer for one language scope. Only the Site,
// Lang, TFunc and Languages fields of scope are read.
func NewErrorPage(r template.Renderer, scope template.ErrorContextInput) *ErrorPage {
	return &ErrorPage{renderer: r, scope: scope}
}

// NotFound writes a themed 404 for the requested path.
func (e *ErrorPage) NotFound(w http.ResponseWriter, r *http.Request) {
	e.Write(w, r, http.StatusNotFound, r.URL.Path)
}

// Write renders statusCode through the theme and sends it. pagePath is what the
// template reports as the failing path, which is not always r.URL.Path — the
// content handler resolves a clean URL to a real file before failing on it.
//
// A nil receiver, a nil renderer, or a template that will not render all fall
// back to plain text: an error response that cannot be produced is worse than
// an ugly one.
func (e *ErrorPage) Write(w http.ResponseWriter, r *http.Request, statusCode int, pagePath string) {
	w.Header().Set("Content-Type", mimeHTML)
	w.WriteHeader(statusCode)

	body, err := e.Render(r.Context(), statusCode, pagePath)
	if err != nil {
		body = fmt.Appendf(nil, "%d %s", statusCode, http.StatusText(statusCode))
	}

	if _, err := w.Write(body); err != nil { // #nosec G104,G705 -- best-effort error response, content from trusted templates
		slog.Error("Cannot write error response", slog.Any("error", err))
	}
}

// Render produces the error page body. The build command renders the same page
// statically and needs the failure surfaced rather than swallowed, which is why
// this is separate from Write.
func (e *ErrorPage) Render(ctx context.Context, statusCode int, pagePath string) ([]byte, error) {
	if e == nil || e.renderer == nil || e.scope.Site == nil {
		return nil, ErrNoErrorPage
	}

	in := e.scope
	in.Path = pagePath
	in.StatusCode = statusCode
	in.Message = StatusMessage(statusCode)

	return e.renderer.Render(ctx, "error.html.tmpl", template.BuildErrorContext(in))
}

// ErrNoErrorPage reports that no theme render was possible — no writer, no
// renderer, or no site config.
var ErrNoErrorPage = errors.New("no error page renderer configured")

const defaultStatusMessage = "Something went wrong. Please try again later."

var statusMessages = map[int]string{
	http.StatusNotFound:            "The page you're looking for doesn't exist.",
	http.StatusForbidden:           "You don't have permission to access this page.",
	http.StatusInternalServerError: defaultStatusMessage,
}

func StatusMessage(statusCode int) string {
	msg, ok := statusMessages[statusCode]
	if ok {
		return msg
	}
	return defaultStatusMessage

}

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
	case errors.Is(err, renderer.ErrNoRenderer):
		return http.StatusUnsupportedMediaType // 415
	case errors.Is(err, renderer.ErrNoMatchingRenderer):
		return http.StatusNotAcceptable // 406

	default:
		return http.StatusInternalServerError // 500
	}
}
