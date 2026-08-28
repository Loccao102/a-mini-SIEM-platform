package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func recordAudit(ctx context.Context, transaction pgx.Tx, actorID int64, action, entityType string, entityID int64, details any) error {
	payload, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = transaction.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, details) VALUES ($1, $2, $3, $4, $5)`, actorID, action, entityType, entityID, payload)
	return err
}

func parsePositiveID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func (handler *Handler) casesRoute(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		if request.PathValue("id") == "" {
			handler.listCases(response, request)
		} else {
			handler.getCase(response, request)
		}
		return
	}
	if request.Method == http.MethodPost && request.PathValue("id") == "" {
		handler.createCase(response, request)
		return
	}
	if request.Method == http.MethodPatch && request.PathValue("id") != "" {
		handler.updateCase(response, request)
		return
	}
	response.Header().Set("Allow", "GET, POST, PATCH")
	writeError(response, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func (handler *Handler) listCases(response http.ResponseWriter, request *http.Request) {
	rows, err := handler.postgres.Query(request.Context(), `SELECT c.case_id, c.title, c.status, c.classification, c.priority, c.assigned_to, c.created_by, c.resolution, c.created_at, c.updated_at, COUNT(ca.alert_id) FROM cases c LEFT JOIN case_alerts ca ON ca.case_id=c.case_id GROUP BY c.case_id ORDER BY c.updated_at DESC`)
	if err != nil {
		writeError(response, 500, err)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, createdBy, alertCount int64
		var title, status, priority string
		var classification, assignedTo, resolution *string
		var createdAt, updatedAt any
		if err := rows.Scan(&id, &title, &status, &classification, &priority, &assignedTo, &createdBy, &resolution, &createdAt, &updatedAt, &alertCount); err != nil {
			writeError(response, 500, err)
			return
		}
		result = append(result, map[string]any{"case_id": id, "title": title, "status": status, "classification": classification, "priority": priority, "assigned_to": assignedTo, "created_by": createdBy, "resolution": resolution, "alert_count": alertCount, "created_at": createdAt, "updated_at": updatedAt})
	}
	writeJSON(response, 200, result)
}

func (handler *Handler) getCase(response http.ResponseWriter, request *http.Request) {
	id, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}
	var title, status, priority string
	var classification, assignedTo, resolution *string
	var createdBy int64
	var createdAt, updatedAt any
	err = handler.postgres.QueryRow(request.Context(), `SELECT title, status, classification, priority, assigned_to, created_by, resolution, created_at, updated_at FROM cases WHERE case_id=$1`, id).Scan(&title, &status, &classification, &priority, &assignedTo, &createdBy, &resolution, &createdAt, &updatedAt)
	if err == pgx.ErrNoRows {
		writeError(response, 404, fmt.Errorf("case not found"))
		return
	}
	if err != nil {
		writeError(response, 500, err)
		return
	}
	writeJSON(response, 200, map[string]any{"case_id": id, "title": title, "status": status, "classification": classification, "priority": priority, "assigned_to": assignedTo, "created_by": createdBy, "resolution": resolution, "created_at": createdAt, "updated_at": updatedAt})
}

func (handler *Handler) createCase(response http.ResponseWriter, request *http.Request) {
	claims := handler.claims(request)
	var payload struct {
		Title      string `json:"title"`
		Priority   string `json:"priority"`
		AssignedTo *int64 `json:"assigned_to"`
		AlertID    *int64 `json:"alert_id"`
	}
	if json.NewDecoder(request.Body).Decode(&payload) != nil || strings.TrimSpace(payload.Title) == "" {
		writeError(response, 400, fmt.Errorf("title is required"))
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Priority == "" {
		payload.Priority = "medium"
	}
	if payload.Priority != "low" && payload.Priority != "medium" && payload.Priority != "high" && payload.Priority != "critical" {
		writeError(response, 400, fmt.Errorf("invalid priority"))
		return
	}
	transaction, err := handler.postgres.Begin(request.Context())
	if err != nil {
		writeError(response, 500, err)
		return
	}
	defer transaction.Rollback(request.Context())
	var caseID int64
	err = transaction.QueryRow(request.Context(), `INSERT INTO cases (title, priority, assigned_to, created_by) VALUES ($1, $2, $3, $4) RETURNING case_id`, payload.Title, payload.Priority, payload.AssignedTo, claims.UserID).Scan(&caseID)
	if err == nil && payload.AlertID != nil {
		_, err = transaction.Exec(request.Context(), `INSERT INTO case_alerts (case_id, alert_id, added_by) VALUES ($1, $2, $3)`, caseID, *payload.AlertID, claims.UserID)
	}
	if err == nil {
		err = recordAudit(request.Context(), transaction, claims.UserID, "case.created", "case", caseID, map[string]any{"title": payload.Title, "priority": payload.Priority, "alert_id": payload.AlertID})
	}
	if err != nil {
		writeError(response, 500, err)
		return
	}
	if err = transaction.Commit(request.Context()); err != nil {
		writeError(response, 500, err)
		return
	}
	writeJSON(response, 201, map[string]any{"case_id": caseID})
}

func (handler *Handler) updateCase(response http.ResponseWriter, request *http.Request) {
	claims := handler.claims(request)
	id, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}
	var payload struct {
		Title          *string `json:"title"`
		Status         *string `json:"status"`
		Classification *string `json:"classification"`
		Priority       *string `json:"priority"`
		AssignedTo     *int64  `json:"assigned_to"`
		Resolution     *string `json:"resolution"`
	}
	if json.NewDecoder(request.Body).Decode(&payload) != nil {
		writeError(response, 400, fmt.Errorf("invalid case payload"))
		return
	}
	transaction, err := handler.postgres.Begin(request.Context())
	if err != nil {
		writeError(response, 500, err)
		return
	}
	defer transaction.Rollback(request.Context())
	_, err = transaction.Exec(request.Context(), `UPDATE cases SET title=COALESCE($1,title), status=COALESCE($2,status), classification=COALESCE($3,classification), priority=COALESCE($4,priority), assigned_to=$5, resolution=COALESCE($6,resolution), updated_at=now() WHERE case_id=$7`, payload.Title, payload.Status, payload.Classification, payload.Priority, payload.AssignedTo, payload.Resolution, id)
	if err == nil {
		err = recordAudit(request.Context(), transaction, claims.UserID, "case.updated", "case", id, payload)
	}
	if err == nil {
		err = transaction.Commit(request.Context())
	}
	if err != nil {
		writeError(response, 500, err)
		return
	}
	writeJSON(response, 200, map[string]any{"case_id": id})
}

func (handler *Handler) caseNotesRoute(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		writeError(response, 405, fmt.Errorf("method not allowed"))
		return
	}
	claims := handler.claims(request)
	caseID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}
	var payload struct {
		Body string `json:"body"`
	}
	if json.NewDecoder(request.Body).Decode(&payload) != nil || strings.TrimSpace(payload.Body) == "" {
		writeError(response, 400, fmt.Errorf("body is required"))
		return
	}
	transaction, err := handler.postgres.Begin(request.Context())
	if err != nil {
		writeError(response, 500, err)
		return
	}
	defer transaction.Rollback(request.Context())
	var noteID int64
	err = transaction.QueryRow(request.Context(), `INSERT INTO case_notes (case_id, author_user_id, body) VALUES ($1, $2, $3) RETURNING note_id`, caseID, claims.UserID, strings.TrimSpace(payload.Body)).Scan(&noteID)
	if err == nil {
		err = recordAudit(request.Context(), transaction, claims.UserID, "case.note_added", "case", caseID, map[string]any{"note_id": noteID})
	}
	if err == nil {
		err = transaction.Commit(request.Context())
	}
	if err != nil {
		writeError(response, 500, err)
		return
	}
	writeJSON(response, 201, map[string]any{"note_id": noteID, "case_id": caseID})
}

func (handler *Handler) caseAlertRoute(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost && request.Method != http.MethodDelete {
		response.Header().Set("Allow", "POST, DELETE")
		writeError(response, 405, fmt.Errorf("method not allowed"))
		return
	}
	claims := handler.claims(request)
	caseID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}
	alertID, err := parsePositiveID(request.PathValue("alert_id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}
	transaction, err := handler.postgres.Begin(request.Context())
	if err != nil {
		writeError(response, 500, err)
		return
	}
	defer transaction.Rollback(request.Context())
	if request.Method == http.MethodPost {
		_, err = transaction.Exec(request.Context(), `INSERT INTO case_alerts (case_id, alert_id, added_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, caseID, alertID, claims.UserID)
	} else {
		_, err = transaction.Exec(request.Context(), `DELETE FROM case_alerts WHERE case_id=$1 AND alert_id=$2`, caseID, alertID)
	}
	if err == nil {
		action := "case.alert_linked"
		if request.Method == http.MethodDelete {
			action = "case.alert_unlinked"
		}
		err = recordAudit(request.Context(), transaction, claims.UserID, action, "case", caseID, map[string]any{"alert_id": alertID})
	}
	if err == nil {
		err = transaction.Commit(request.Context())
	}
	if err != nil {
		writeError(response, 500, err)
		return
	}
	response.WriteHeader(204)
}

func (handler *Handler) caseTimeline(response http.ResponseWriter, request *http.Request) {
	caseID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}
	rows, err := handler.postgres.Query(request.Context(), `SELECT 'note' AS kind, note_id, author_user_id, body, created_at FROM case_notes WHERE case_id=$1 UNION ALL SELECT 'audit', audit_id, actor_user_id, action, created_at FROM audit_logs WHERE entity_type='case' AND entity_id=$1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		writeError(response, 500, err)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var kind, body string
		var itemID, actorID int64
		var createdAt any
		if err := rows.Scan(&kind, &itemID, &actorID, &body, &createdAt); err != nil {
			writeError(response, 500, err)
			return
		}
		result = append(result, map[string]any{"kind": kind, "id": itemID, "actor_user_id": actorID, "body": body, "created_at": createdAt})
	}
	writeJSON(response, 200, result)
}
