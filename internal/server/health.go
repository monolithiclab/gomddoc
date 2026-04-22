package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

// HealthHandler provides health check endpoints for liveness and readiness probes.
type HealthHandler struct {
	provider provider.Provider
}

// NewHealthHandler creates a new HealthHandler with the given provider.
func NewHealthHandler(provider provider.Provider) *HealthHandler {
	return &HealthHandler{provider: provider}
}

// healthResponse is the JSON structure returned by health endpoints.
type healthResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// LiveHandler returns 200 OK unconditionally, indicating the process is alive.
func (h *HealthHandler) LiveHandler(w http.ResponseWriter, _ *http.Request) {
	writeHealthJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// ReadyHandler checks whether the provider is ready to serve content.
// It returns 200 if the provider can stat its root, or 503 if not.
func (h *HealthHandler) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := h.provider.Stat(r.Context(), "."); err != nil {
		slog.Warn("readiness check failed", slog.String("error", err.Error()))
		writeHealthJSON(w, http.StatusServiceUnavailable, healthResponse{
			Status: "unavailable",
			Error:  "provider not ready",
		})
		return
	}
	writeHealthJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// writeHealthJSON marshals resp as JSON and writes it with the given status code.
func writeHealthJSON(w http.ResponseWriter, status int, resp healthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to write health response", slog.String("error", err.Error()))
	}
}
