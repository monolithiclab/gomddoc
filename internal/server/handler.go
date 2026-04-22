package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"

	"github.com/monolithiclab/gomddoc/internal/processor"
	"github.com/monolithiclab/gomddoc/internal/provider"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

// Handler holds dependencies for HTTP request handling
type Handler struct {
	provider  provider.Provider
	processor processor.Processor
	renderer  tmpl.Renderer
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(provider provider.Provider, processor processor.Processor, renderer tmpl.Renderer) *Handler {
	return &Handler{
		provider:  provider,
		processor: processor,
		renderer:  renderer,
	}
}

// ServeMarkdown handles HTTP requests for markdown files
func (h *Handler) ServeMarkdown(w http.ResponseWriter, r *http.Request) {
	// Read the file content
	filename := r.URL.Path
	md, err := h.provider.ReadFile(filename)
	if err != nil {
		h.handleError(w, err, filename)
		return
	}

	// Process markdown to HTML
	processedContent, err := h.processor.Process(md)
	if err != nil {
		slog.Error("Cannot process markdown", slog.String("filename", filename), slog.Any("error", err))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeErrorResponse(w, "Internal server error")
		return
	}

	// Prepare template context
	context := &tmpl.TemplateContext{
		Title:   "Prose",
		Content: template.HTML(string(processedContent)), // #nosec G203
		Theme: tmpl.ThemeOptions{
			BaseURL: "/assets/themes/default",
		},
	}

	// Render the template
	renderedHTML, err := h.renderer.Render("layout.html.tmpl", context)
	if err != nil {
		slog.Error("Cannot render template", slog.Any("error", err))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeErrorResponse(w, "Internal server error")
		return
	}

	// Write successful response
	w.Header().Set("Content-Type", h.processor.ContentType())
	w.Header().Set("Cache-Control", "public, max-age=300") // 5 minute cache
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(renderedHTML)
	if writeErr != nil {
		slog.Error("Cannot write response", slog.Any("error", writeErr))
	}
}

// handleError handles file reading errors and sends appropriate HTTP responses
func (h *Handler) handleError(w http.ResponseWriter, err error, filename string) {
	if os.IsNotExist(err) {
		slog.Info("File not found", slog.String("filename", filename))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		h.writeErrorResponse(w, "File not found")
	} else {
		slog.Error("Cannot read file", slog.String("filename", filename), slog.Any("error", err))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeErrorResponse(w, "Internal server error")
	}
}

// writeErrorResponse writes an error message to the response
func (h *Handler) writeErrorResponse(w http.ResponseWriter, message string) {
	_, writeErr := w.Write([]byte(message))
	if writeErr != nil {
		slog.Error("Cannot write error response", slog.Any("error", writeErr))
	}
}
