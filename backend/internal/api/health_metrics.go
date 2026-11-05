package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/health"
)

// HealthzResponse is intentionally safe for unauthenticated container probes.
type HealthzResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// DetailedHealthResponse is restricted to authenticated SOC administrators.
type DetailedHealthResponse struct {
	Status    string                          `json:"status"`
	Uptime    string                          `json:"uptime"`
	CheckedAt time.Time                       `json:"checked_at"`
	Services  map[string]health.ServiceHealth `json:"services"`
}

// MetricsResponse represents pipeline metrics
type MetricsResponse struct {
	Status string                 `json:"status"`
	Data   map[string]interface{} `json:"data"`
	Uptime int64                  `json:"uptime_seconds"`
}

// DLQResponse represents DLQ operations response
type DLQResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Stats   map[string]interface{} `json:"stats"`
}

func (h *Handler) uptime() string {
	if h.startedAt.IsZero() {
		return "unknown"
	}
	return time.Since(h.startedAt).Round(time.Second).String()
}

// handleHealthz is safe for unauthenticated load balancers and Docker.
// It never returns service error messages, database schema details or host names.
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	checks := h.healthChecker.CheckAll(r.Context())
	status := health.OverallFromChecks(checks)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if status == health.StatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(HealthzResponse{
		Status: string(status), Uptime: h.uptime(),
	})
}

// handleDetailedHealth executes the checks once per request. Route auth is admin-only.
func (h *Handler) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	checks := h.healthChecker.CheckAll(r.Context())
	status := health.OverallFromChecks(checks)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Always return 200 for authenticated diagnostics so unhealthy details remain readable.
	_ = json.NewEncoder(w).Encode(DetailedHealthResponse{
		Status: string(status), Uptime: h.uptime(),
		CheckedAt: time.Now().UTC(), Services: checks,
	})
}

func (h *Handler) handleServiceHealth(check func(context.Context) health.ServiceHealth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := check(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if result.Status == health.StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": string(result.Status)})
	}
}

// handleMetrics handles GET /metrics requests
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	snapshot := h.metrics.GetSnapshot(ctx)

	response := MetricsResponse{
		Status: "ok",
		Data:   snapshot,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDLQStats handles GET /api/v1/dlq/stats requests
func (h *Handler) handleDLQStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dlqStats, err := h.dlqManager.GetDLQStats(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	retryStats, err := h.dlqManager.GetRetryStats(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	stats := make(map[string]interface{})
	stats["dlq"] = dlqStats
	stats["retry"] = retryStats

	response := DLQResponse{
		Status: "ok",
		Stats:  stats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDLQReplay handles POST /api/v1/dlq/replay/{messageId} requests
func (h *Handler) handleDLQReplay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract message ID from path
	messageID := r.PathValue("messageId")
	if messageID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "message_id required",
		})
		return
	}

	err := h.dlqManager.ReplayMessage(ctx, messageID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	response := DLQResponse{
		Status:  "ok",
		Message: "Message replayed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDLQPurge handles DELETE /api/v1/dlq/purge requests
// Query param: older_than_hours (default: 24)
func (h *Handler) handleDLQPurge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get age parameter
	olderThanHours := r.URL.Query().Get("older_than_hours")
	if olderThanHours == "" {
		olderThanHours = "24"
	}

	var hours int64
	if err := json.Unmarshal([]byte(olderThanHours), &hours); err != nil {
		hours = 24
	}

	purged, err := h.dlqManager.PurgeDLQ(ctx, time.Duration(hours)*time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	response := map[string]interface{}{
		"status":           "ok",
		"messages_purged":  purged,
		"older_than_hours": hours,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
