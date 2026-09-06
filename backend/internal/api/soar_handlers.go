package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/soar"
)

func (handler *Handler) SetSOAR(engine *soar.Engine) {
	handler.soarEngine = engine
}

func (handler *Handler) soarPlaybooks(w http.ResponseWriter, r *http.Request) {
	if handler.soarEngine == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SOAR engine not configured"))
		return
	}
	playbooks, err := handler.soarEngine.ListPlaybooks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, playbooks)
}

func (handler *Handler) soarExecutions(w http.ResponseWriter, r *http.Request) {
	if handler.soarEngine == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SOAR engine not configured"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	executions, err := handler.soarEngine.ListExecutions(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, executions)
}

func (handler *Handler) soarApprove(w http.ResponseWriter, r *http.Request) {
	if handler.soarEngine == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SOAR engine not configured"))
		return
	}
	id, err := parsePositiveID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	claims := handler.claims(r)
	if err := handler.soarEngine.ApproveExecution(r.Context(), id, claims.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "executed", "execution_id": id})
}

func (handler *Handler) soarReject(w http.ResponseWriter, r *http.Request) {
	if handler.soarEngine == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SOAR engine not configured"))
		return
	}
	id, err := parsePositiveID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	claims := handler.claims(r)
	if err := handler.soarEngine.RejectExecution(r.Context(), id, claims.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "rejected", "execution_id": id})
}

func (handler *Handler) soarBlocked(w http.ResponseWriter, r *http.Request) {
	if handler.soarEngine == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SOAR engine not configured"))
		return
	}
	blocked, err := handler.soarEngine.ListBlocked(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, blocked)
}

func (handler *Handler) soarReleaseBlocked(w http.ResponseWriter, r *http.Request) {
	if handler.soarEngine == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SOAR engine not configured"))
		return
	}
	id, err := parsePositiveID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	claims := handler.claims(r)
	if err := handler.soarEngine.ReleaseBlockedEntity(r.Context(), id, claims.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "released", "blocked_id": id})
}
