package server

import (
	"log/slog"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

// HealthHandler provides health check endpoints for liveness and readiness probes.
type HealthHandler struct {
	provider provider.Provider
}

// NewHealthHandler creates a new HealthHandler with the given provider.
func NewHealthHandler(p provider.Provider) *HealthHandler {
	return &HealthHandler{provider: p}
}

// healthResponse is the JSON structure returned by health endpoints.
type healthResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// LiveHandler returns 200 OK unconditionally, indicating the process is alive.
func (h *HealthHandler) LiveHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// ReadyHandler checks whether the provider is ready to serve content.
// It returns 200 if the provider can stat its root, or 503 if not.
func (h *HealthHandler) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := h.provider.Stat(r.Context(), "."); err != nil {
		slog.Warn("readiness check failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{
			Status: "unavailable",
			Error:  "provider not ready",
		})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}
