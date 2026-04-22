package server

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// RedirectFinder returns the first page path under a directory for redirect
// when no index file exists. Returns "" if no page is found.
type RedirectFinder func(dirPath string) string

// HandlerConfig holds the dependencies for creating an HTTP content handler.
type HandlerConfig struct {
	Provider         provider.Provider
	Registry         renderer.RendererRegistry
	EnricherRegistry enricher.EnricherRegistry
	TemplateRenderer tmpl.Renderer
	SiteConfig       *config.SiteConfig
	RedirectFinder   RedirectFinder
	URLRedirects     URLRedirectMap
	Resolver         *resolve.PathResolver
}

// Handler holds dependencies for HTTP request handling
type Handler struct {
	provider         provider.Provider
	registry         renderer.RendererRegistry
	enricherRegistry enricher.EnricherRegistry
	templateRenderer tmpl.Renderer
	siteConfig       *config.SiteConfig
	redirectFinder   RedirectFinder
	urlRedirects     URLRedirectMap
	resolver         *resolve.PathResolver
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		provider:         cfg.Provider,
		registry:         cfg.Registry,
		enricherRegistry: cfg.EnricherRegistry,
		templateRenderer: cfg.TemplateRenderer,
		siteConfig:       cfg.SiteConfig,
		redirectFinder:   cfg.RedirectFinder,
		urlRedirects:     cfg.URLRedirects,
		resolver:         cfg.Resolver,
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
	// 0. Check URL redirects before reading files
	if h.urlRedirects != nil {
		if target, ok := h.urlRedirects[r.URL.Path]; ok {
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
	}

	// 1. Read file + get MIME type
	content, mimeType, err := h.provider.ReadFile(r.Context(), r.URL.Path)
	if err != nil && errors.Is(err, provider.ErrNotFound) && h.resolver != nil {
		// Try resolver for extensionless paths
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if realPath, found := h.resolver.Resolve(cleanPath); found {
			content, mimeType, err = h.provider.ReadFile(r.Context(), "/"+realPath)
		}
	}
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

	// 5. Serve based on selected output MIME type.
	// Set Vary: Accept so caches distinguish responses by content negotiation.
	w.Header().Add("Vary", "Accept")
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
			Features:   config.MergeFeatures(h.siteConfig.Theme.Features, enrichment.Features),
			TOC:        enrichment.TOC,
			Navigation: enrichment.Navigation,
			PrevPage:   enrichment.PrevPage,
			NextPage:   enrichment.NextPage,
		},
	}

	templateName := tmpl.ResolveLayout(h.templateRenderer, metadata)
	rendered, err := h.templateRenderer.Render(r.Context(), templateName, context)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	serveWithETag(w, r, rendered, "text/html; charset=utf-8", "public, max-age=300")
}

// serveRaw serves content directly without template wrapping (passthrough).
func (h *Handler) serveRaw(w http.ResponseWriter, r *http.Request, content []byte, mimeType string) {
	serveWithETag(w, r, content, mimeType, "public, max-age=300")
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

	page := h.renderErrorPage(r, statusCode, path)
	_, writeErr := w.Write(page) // #nosec G104,G705 -- best-effort error response, content from trusted templates
	if writeErr != nil {
		slog.Error("Cannot write error response", slog.Any("error", writeErr))
	}
}

// renderErrorPage renders an error page through the template engine.
// Falls back to plain text if template rendering fails.
func (h *Handler) renderErrorPage(r *http.Request, statusCode int, pagePath string) []byte {
	statusTitle := http.StatusText(statusCode)

	errorMeta := map[string]any{
		"title":         statusTitle,
		"robots":        "noindex",
		"error_code":    statusCode,
		"error_title":   statusTitle,
		"error_message": StatusMessage(statusCode),
	}
	context := &tmpl.TemplateContext{
		Site: h.siteConfig,
		Page: tmpl.PageContext{
			Path:     pagePath,
			Meta:     errorMeta,
			Features: config.MergeFeatures(h.siteConfig.Theme.Features),
		},
	}

	rendered, err := h.templateRenderer.Render(r.Context(), "error.html.tmpl", context)
	if err != nil {
		return fmt.Appendf(nil, "%d %s", statusCode, statusTitle)
	}
	return rendered
}
