package server

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// RedirectFinder returns the first page path under a directory for redirect
// when no index file exists. Returns "" if no page is found.
type RedirectFinder func(dirPath string) string

// Handler holds dependencies for HTTP request handling
type Handler struct {
	provider         provider.Provider
	registry         renderer.RendererRegistry
	enricherRegistry enricher.EnricherRegistry
	templateRenderer tmpl.Renderer
	siteConfig       *config.SiteConfig
	redirectFinder   RedirectFinder
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(
	provider provider.Provider,
	registry renderer.RendererRegistry,
	enricherRegistry enricher.EnricherRegistry,
	templateRenderer tmpl.Renderer,
	siteConfig *config.SiteConfig,
	redirectFinder RedirectFinder,
) *Handler {
	return &Handler{
		provider:         provider,
		registry:         registry,
		enricherRegistry: enricherRegistry,
		templateRenderer: templateRenderer,
		siteConfig:       siteConfig,
		redirectFinder:   redirectFinder,
	}
}

// ServeContent handles HTTP requests with content negotiation and rendering.
//
// Flow:
//  1. Read file from provider (gets content + input MIME type)
//  2. Parse Accept header and negotiate renderer (2D: input type + output type)
//  3. Enrich content (extract metadata, TOC, navigation, related docs)
//  4. Render content with enrichment data
//  5. Serve as HTML (wrapped in template) or raw (passthrough)
func (h *Handler) ServeContent(w http.ResponseWriter, r *http.Request) {
	// 1. Read file + get MIME type
	content, mimeType, err := h.provider.ReadFile(r.Context(), r.URL.Path)
	if err != nil {
		// When a directory has no index file, redirect to the first page
		// in the navigation tree instead of returning 403.
		if errors.Is(err, provider.ErrDirListingDisabled) && h.redirectFinder != nil {
			if target := h.redirectFinder(r.URL.Path); target != "" {
				http.Redirect(w, r, target, http.StatusFound)
				return
			}
		}
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 2. Content negotiation BEFORE rendering (2D lookup)
	normalized := negotiate.NormalizeMimeType(mimeType)
	acceptedTypes := negotiate.ParseAccept(r.Header.Get("Accept"))

	contentRenderer, selectedOutput, err := h.registry.Get(normalized, acceptedTypes)
	if err != nil {
		if errors.Is(err, renderer.ErrNoMatchingRenderer) {
			available := h.registry.AvailableOutputTypes(normalized)
			http.Error(w,
				"Not Acceptable: available types: "+strings.Join(available, ", "),
				http.StatusNotAcceptable,
			)
			return
		}
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 3. Enrich content (extract metadata, TOC, navigation, related docs)
	enrichment, err := h.enricherRegistry.Get(normalized).Enrich(r.Context(), content, r.URL.Path)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 4. Render content
	renderResult, err := contentRenderer.Render(r.Context(), content, enrichment)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	// 5. Serve based on selected output MIME type
	outputNormalized := negotiate.NormalizeMimeType(selectedOutput)
	if outputNormalized == "text/html" {
		h.serveHTML(w, r, renderResult.Content, enrichment)
	} else {
		// Use the full MIME type from RenderResult if available, otherwise the selected output
		serveMimeType := renderResult.MimeType
		if serveMimeType == "" {
			serveMimeType = mimeType // Passthrough: preserve original
		}
		h.serveRaw(w, r, renderResult.Content, serveMimeType)
	}
}

// serveHTML wraps HTML content in the site template and serves it.
func (h *Handler) serveHTML(w http.ResponseWriter, r *http.Request, htmlContent []byte, enrichment *enricher.EnrichmentData) {
	metadata := enrichment.Metadata
	if metadata == nil {
		metadata = make(map[string]any)
	}

	// Default title logic
	if _, ok := metadata["title"]; !ok {
		metadata["title"] = text.DeriveTitle(r.URL.Path)
	}

	context := &tmpl.TemplateContext{
		Site: h.siteConfig,
		Page: tmpl.PageContext{
			Content:    template.HTML(htmlContent), // #nosec G203
			Path:       r.URL.Path,
			Meta:       metadata,
			TOC:        enrichment.TOC,
			Navigation: enrichment.Navigation,
		},
	}

	templateName := tmpl.ResolveLayout(h.templateRenderer, metadata)
	rendered, err := h.templateRenderer.Render(r.Context(), templateName, context)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	etag := generateETag(rendered)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=300")

	if checkETag(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(rendered)))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(rendered) // #nosec G705 -- template-rendered HTML served with correct Content-Type
	if writeErr != nil {
		slog.Error("Cannot write response",
			slog.String("request_id", GetRequestID(r.Context())),
			slog.Any("error", writeErr))
	}
}

// serveRaw serves content directly without template wrapping (passthrough).
func (h *Handler) serveRaw(w http.ResponseWriter, r *http.Request, content []byte, mimeType string) {
	etag := generateETag(content)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=300")

	if checkETag(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(content) // #nosec G705 -- static file content served with correct Content-Type and nosniff header
	if writeErr != nil {
		slog.Error("Cannot write response",
			slog.String("request_id", GetRequestID(r.Context())),
			slog.Any("error", writeErr))
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
			slog.String("request_id", GetRequestID(r.Context())),
			text.Safe("path", path),
		)
	case http.StatusForbidden:
		slog.Info("Access forbidden", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
			slog.Int("status", statusCode),
			slog.String("request_id", GetRequestID(r.Context())),
			text.Safe("path", path),
			slog.String("error", err.Error()),
		)
	default:
		slog.Error("Request failed", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
			slog.Int("status", statusCode),
			slog.String("request_id", GetRequestID(r.Context())),
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
