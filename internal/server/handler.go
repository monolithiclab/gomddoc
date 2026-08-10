package server

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

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

	// i18n support
	Lang      string              // BCP 47 language for this handler
	TFunc     func(string) string // translation function
	Languages []tmpl.LanguageInfo // all available languages
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

	// i18n support
	lang      string              // BCP 47 language for this handler
	tFunc     func(string) string // translation function
	languages []tmpl.LanguageInfo // all available languages

	errorPage *ErrorPage
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(cfg HandlerConfig) *Handler {
	// The handler owns its language scope's error page rather than receiving
	// one, so it cannot be handed a writer that disagrees with the renderer and
	// translator it was built with. Every other route in the scope borrows it
	// through ErrorPage() — see that method.
	errorPage := NewErrorPage(cfg.TemplateRenderer, tmpl.ErrorContextInput{
		Site:      cfg.SiteConfig,
		Lang:      cfg.Lang,
		TFunc:     cfg.TFunc,
		Languages: cfg.Languages,
	})
	return &Handler{
		provider:         cfg.Provider,
		registry:         cfg.Registry,
		enricherRegistry: cfg.EnricherRegistry,
		templateRenderer: cfg.TemplateRenderer,
		siteConfig:       cfg.SiteConfig,
		redirectFinder:   cfg.RedirectFinder,
		urlRedirects:     cfg.URLRedirects,
		resolver:         cfg.Resolver,
		lang:             cfg.Lang,
		tFunc:            cfg.TFunc,
		languages:        cfg.Languages,
		errorPage:        errorPage,
	}
}

// ErrorPage returns the language scope's error writer, for the middleware and
// sibling routes registered around this handler. They must write the same body
// this handler does — see ErrorPage's doc comment for why.
func (h *Handler) ErrorPage() *ErrorPage { return h.errorPage }

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

	// 1. Read file + get MIME type.
	filePath := r.URL.Path
	content, mimeType, err := h.provider.ReadFile(r.Context(), filePath)
	if err != nil && errors.Is(err, provider.ErrNotFound) && h.resolver != nil {
		// Try resolver for extensionless paths
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if realPath, found := h.resolver.Resolve(cleanPath); found {
			filePath = "/" + realPath
			content, mimeType, err = h.provider.ReadFile(r.Context(), filePath)
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
			// Plain text, not the themed error page: the client's Accept header
			// just told us it will not take HTML, which is the whole reason we
			// are here. The available-types list is also the actionable part of
			// a 406 and has nowhere to go in the theme's error layout.
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
		h.serveHTML(w, r, renderResult.Content, enrichment, filePath)
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
// filePath is the file that produced htmlContent, which is not r.URL.Path when
// strip_extensions is on.
func (h *Handler) serveHTML(w http.ResponseWriter, r *http.Request, htmlContent []byte, enrichment *enricher.EnrichmentData, filePath string) {
	// The only consumer is JSON-LD's dateModified, and seo.GenerateJSONLD emits
	// nothing at all without a domain — so gate the stat on the same condition
	// rather than spending a syscall per render on every site that has not
	// configured one. Under the git provider the stat is a tree walk holding the
	// exclusive tree lock, which makes the gate worth more than it looks.
	// A failure is not an error: JSON-LD falls back to the frontmatter date.
	var modTime time.Time
	if h.siteConfig.Meta.Domain != "" {
		if info, err := h.provider.Stat(r.Context(), filePath); err == nil {
			modTime = info.ModTime()
		}
	}

	context := tmpl.BuildPageContext(tmpl.PageContextInput{
		Site:       h.siteConfig,
		Path:       r.URL.Path,
		Content:    template.HTML(htmlContent), // #nosec G203
		Enrichment: enrichment,
		ModTime:    modTime,
		Renderer:   h.templateRenderer,
		Lang:       h.lang,
		TFunc:      h.tFunc,
		Languages:  h.languages,
	})

	templateName := tmpl.ResolveLayout(h.templateRenderer, context.Page.Meta)
	rendered, err := h.templateRenderer.Render(r.Context(), templateName, context)
	if err != nil {
		h.handleError(w, r, err, r.URL.Path)
		return
	}

	serveWithETag(w, r, rendered, mimeHTML, cacheDynamic)
}

// serveRaw serves content directly without template wrapping (passthrough).
func (h *Handler) serveRaw(w http.ResponseWriter, r *http.Request, content []byte, mimeType string) {
	serveWithETag(w, r, content, mimeType, cacheDynamic)
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

	h.errorPage.Write(w, r, statusCode, path)
}
