package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// Handler holds dependencies for HTTP request handling
type Handler struct {
	provider         provider.Provider
	registry         renderer.RendererRegistry
	templateRenderer tmpl.Renderer
	siteConfig       *config.SiteConfig
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(
	provider provider.Provider,
	registry renderer.RendererRegistry,
	templateRenderer tmpl.Renderer,
	siteConfig *config.SiteConfig,
) *Handler {
	return &Handler{
		provider:         provider,
		registry:         registry,
		templateRenderer: templateRenderer,
		siteConfig:       siteConfig,
	}
}

// ServeContent handles HTTP requests with content negotiation and rendering.
//
// Flow:
//  1. Read file from provider (gets content + input MIME type)
//  2. Get renderer for input MIME type
//  3. Render content (transforms to output MIME type + metadata)
//  4. Content negotiation on output MIME type with Accept header
//  5. Serve as HTML (wrapped in template) or raw (passthrough)
func (h *Handler) ServeContent(w http.ResponseWriter, r *http.Request) {
	// 1. Read file + get MIME type
	content, mimeType, err := h.provider.ReadFile(r.URL.Path)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 2. Get renderer for input MIME type
	normalized := renderer.NormalizeMimeType(mimeType)
	contentRenderer, err := h.registry.Get(normalized)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 3. Render content to get output MIME type and metadata
	renderResult, err := contentRenderer.Render(r.Context(), content)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 4. Determine final MIME type
	finalMimeType := renderResult.MimeType
	if finalMimeType == "" {
		finalMimeType = mimeType // Preserve full MIME type from provider (passthrough)
	}

	// 5. Content negotiation on OUTPUT MIME type
	acceptedTypes := ParseAccept(r.Header.Get("Accept"))
	finalNormalized := renderer.NormalizeMimeType(finalMimeType)

	accepted := false
	for _, mt := range acceptedTypes {
		if mt.Matches(finalNormalized) {
			accepted = true
			break
		}
	}

	if !accepted {
		http.Error(w, "Not Acceptable: server can only provide "+finalNormalized, http.StatusNotAcceptable)
		return
	}

	// 6. Serve based on output MIME type
	if finalNormalized == "text/html" {
		h.serveHTML(w, r, renderResult.Content, renderResult.Metadata, renderResult.TOC)
	} else {
		h.serveRaw(w, renderResult.Content, finalMimeType)
	}
}

// serveHTML wraps HTML content in the site template and serves it.
func (h *Handler) serveHTML(w http.ResponseWriter, r *http.Request, htmlContent []byte, metadata map[string]any, toc *renderer.TOCNode) {
	// Initialize metadata if nil
	if metadata == nil {
		metadata = make(map[string]any)
	}

	// Default title logic
	if _, ok := metadata["title"]; !ok {
		metadata["title"] = deriveTitle(r.URL.Path)
	}

	context := &tmpl.TemplateContext{
		Site: h.siteConfig,
		Page: tmpl.PageContext{
			Content: template.HTML(htmlContent), // #nosec G203
			Path:    r.URL.Path,
			Meta:    metadata,
			TOC:     toc,
		},
	}

	rendered, err := h.templateRenderer.Render(r.Context(), "layout.html.tmpl", context)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(rendered)))
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(rendered)
	if writeErr != nil {
		slog.Error("Cannot write response", slog.Any("error", writeErr))
	}
}

// deriveTitle derives a title from the file path.
// Example: "/docs/my-page.md" -> "My Page"
func deriveTitle(reqPath string) string {
	base := path.Base(reqPath)
	if base == "." || base == "/" {
		return "Home"
	}

	// Remove extension
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Replace hyphens/underscores with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Capitalize Title Case
	return cases.Title(language.English).String(name)
}

// serveRaw serves content directly without template wrapping (passthrough).
func (h *Handler) serveRaw(w http.ResponseWriter, content []byte, mimeType string) {
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(content) // #nosec G705 -- static file content served with correct Content-Type and nosniff header
	if writeErr != nil {
		slog.Error("Cannot write response", slog.Any("error", writeErr))
	}
}

// handleError handles errors and sends appropriate HTTP responses.
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error, path string) {
	statusCode := classifyError(err)

	// Log based on severity
	switch statusCode {
	case http.StatusNotFound:
		slog.Info("File not found", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
			slog.Int("status", statusCode),
			text.Safe("path", path),
		)
	case http.StatusForbidden:
		slog.Info("Access forbidden", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
			slog.Int("status", statusCode),
			text.Safe("path", path),
			slog.String("error", err.Error()),
		)
	default:
		slog.Error("Request failed", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
			slog.Int("status", statusCode),
			text.Safe("path", path),
			slog.String("error", err.Error()),
		)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	// Serve simple error pages
	var message string
	switch statusCode {
	case http.StatusNotFound:
		message = "<h1>404 Not Found</h1>"
	case http.StatusForbidden:
		message = "<h1>403 Forbidden</h1>"
	default:
		message = "<h1>500 Internal Server Error</h1>"
	}

	_, writeErr := w.Write([]byte(message))
	if writeErr != nil {
		slog.Error("Cannot write error response", slog.Any("error", writeErr))
	}
}
