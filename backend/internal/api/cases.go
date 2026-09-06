package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

func (handler *Handler) caseReport(response http.ResponseWriter, request *http.Request) {
	caseID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		writeError(response, 400, err)
		return
	}

	ctx := request.Context()
	var title, status, priority string
	var classification, assignedTo, resolution *string
	var createdBy int64
	var createdAt, updatedAt time.Time
	err = handler.postgres.QueryRow(ctx, `SELECT title, status, classification, priority, assigned_to, created_by, resolution, created_at, updated_at FROM cases WHERE case_id=$1`, caseID).
		Scan(&title, &status, &classification, &priority, &assignedTo, &createdBy, &resolution, &createdAt, &updatedAt)
	if err == pgx.ErrNoRows {
		writeError(response, 404, fmt.Errorf("case not found"))
		return
	}
	if err != nil {
		writeError(response, 500, err)
		return
	}

	// Fetch linked alerts
	alertRows, err := handler.postgres.Query(ctx, `SELECT a.alert_id, a.rule_id, a.description, a.severity, a.status, a.source_ip, a.dest_ip, a.event_count, a.created_at FROM alerts a JOIN case_alerts ca ON ca.alert_id = a.alert_id WHERE ca.case_id = $1 ORDER BY a.created_at ASC`, caseID)
	linkedAlerts := make([]map[string]any, 0)
	if err == nil {
		defer alertRows.Close()
		for alertRows.Next() {
			var alertID, ruleID, eventCount int64
			var desc, sev, st string
			var srcIP, dstIP *string
			var cAt time.Time
			if err := alertRows.Scan(&alertID, &ruleID, &desc, &sev, &st, &srcIP, &dstIP, &eventCount, &cAt); err == nil {
				linkedAlerts = append(linkedAlerts, map[string]any{
					"alert_id":    alertID,
					"rule_id":     ruleID,
					"description": desc,
					"severity":    sev,
					"status":      st,
					"source_ip":   srcIP,
					"dest_ip":     dstIP,
					"event_count": eventCount,
					"created_at":  cAt,
				})
			}
		}
	}

	// Fetch case notes
	noteRows, err := handler.postgres.Query(ctx, `SELECT n.note_id, n.author_user_id, COALESCE(u.email, 'System'), n.body, n.created_at FROM case_notes n LEFT JOIN users u ON u.user_id = n.author_user_id WHERE n.case_id = $1 ORDER BY n.created_at ASC`, caseID)
	caseNotes := make([]map[string]any, 0)
	if err == nil {
		defer noteRows.Close()
		for noteRows.Next() {
			var noteID, authorID int64
			var authorEmail, body string
			var cAt time.Time
			if err := noteRows.Scan(&noteID, &authorID, &authorEmail, &body, &cAt); err == nil {
				caseNotes = append(caseNotes, map[string]any{
					"note_id":      noteID,
					"author_id":    authorID,
					"author_email": authorEmail,
					"body":         body,
					"created_at":   cAt,
				})
			}
		}
	}

	// Fetch SOAR executions associated with linked alerts
	soarActions := make([]map[string]any, 0)
	execRows, err := handler.postgres.Query(ctx, `SELECT pe.execution_id, p.name, pe.alert_id, pe.status, pe.target, pe.action_type, p.requires_approval, pe.executed_at, pe.created_at FROM playbook_executions pe JOIN playbooks p ON p.playbook_id = pe.playbook_id WHERE pe.alert_id IN (SELECT alert_id FROM case_alerts WHERE case_id = $1) ORDER BY pe.created_at ASC`, caseID)
	if err == nil {
		defer execRows.Close()
		for execRows.Next() {
			var execID int64
			var pName, status, target, actType string
			var alertID *int64
			var reqApp bool
			var execAt *time.Time
			var cAt time.Time
			if err := execRows.Scan(&execID, &pName, &alertID, &status, &target, &actType, &reqApp, &execAt, &cAt); err == nil {
				soarActions = append(soarActions, map[string]any{
					"execution_id":     execID,
					"playbook_name":    pName,
					"alert_id":         alertID,
					"status":           status,
					"target":           target,
					"action_type":      actType,
					"require_approval": reqApp,
					"executed_at":      execAt,
					"created_at":       cAt,
				})
			}
		}
	}

	// Format check: json or html
	format := request.URL.Query().Get("format")
	if format == "json" || (!strings.Contains(request.Header.Get("Accept"), "text/html") && format != "html") {
		writeJSON(response, 200, map[string]any{
			"case": map[string]any{
				"case_id":        caseID,
				"title":          title,
				"status":         status,
				"classification": classification,
				"priority":       priority,
				"assigned_to":    assignedTo,
				"created_by":     createdBy,
				"resolution":     resolution,
				"created_at":     createdAt,
				"updated_at":     updatedAt,
			},
			"alerts":        linkedAlerts,
			"notes":         caseNotes,
			"soar_actions":  soarActions,
			"generated_at":  time.Now().UTC(),
		})
		return
	}

	// Render Printable Executive HTML Incident Report
	classStr := "Unclassified"
	if classification != nil {
		classStr = *classification
	}
	resStr := "Pending Investigation"
	if resolution != nil && *resolution != "" {
		resStr = *resolution
	}

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Incident Report - Case #` + strconv.FormatInt(caseID, 10) + ` - ` + escapeHTML(title) + `</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 40px; color: #1f2937; background: #fff; line-height: 1.5; }
  .header { border-bottom: 3px solid #1e3a8a; padding-bottom: 16px; margin-bottom: 24px; display: flex; justify-content: space-between; align-items: flex-start; }
  .header h1 { margin: 0 0 6px 0; font-size: 24px; color: #1e3a8a; }
  .badge { display: inline-block; padding: 4px 10px; border-radius: 9999px; font-size: 12px; font-weight: 600; text-transform: uppercase; }
  .badge-critical { background: #fee2e2; color: #b91c1c; border: 1px solid #ef4444; }
  .badge-high { background: #ffedd5; color: #c2410c; border: 1px solid #f97316; }
  .badge-medium { background: #fef9c3; color: #854d0e; border: 1px solid #eab308; }
  .badge-low { background: #ecfdf5; color: #047857; border: 1px solid #10b981; }
  .section { margin-bottom: 28px; }
  .section h2 { font-size: 16px; color: #374151; border-bottom: 1px solid #e5e7eb; padding-bottom: 6px; margin-bottom: 12px; text-transform: uppercase; letter-spacing: 0.05em; }
  table { width: 100%; border-collapse: collapse; margin-top: 8px; font-size: 13px; }
  th, td { border: 1px solid #e5e7eb; padding: 8px 12px; text-align: left; }
  th { background-color: #f9fafb; font-weight: 600; color: #4b5563; }
  .timeline-item { border-left: 2px solid #3b82f6; padding-left: 12px; margin-bottom: 14px; }
  .timeline-time { font-size: 11px; color: #6b7280; }
  .sign-off { margin-top: 48px; display: flex; justify-content: space-between; page-break-inside: avoid; }
  .sign-box { border-top: 1px solid #9ca3af; width: 42%; padding-top: 8px; font-size: 13px; color: #4b5563; }
  @media print {
    body { margin: 15mm; }
    .no-print { display: none; }
  }
</style>
</head>
<body>
  <div class="no-print" style="margin-bottom: 16px; text-align: right;">
    <button onclick="window.print()" style="padding: 8px 16px; background: #2563eb; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-weight: 500;">Print / Save as PDF</button>
  </div>
  <div class="header">
    <div>
      <h1>SECURITY INCIDENT REPORT</h1>
      <div style="color: #6b7280; font-size: 14px;">Case #` + strconv.FormatInt(caseID, 10) + ` &bull; ` + escapeHTML(title) + `</div>
    </div>
    <div style="text-align: right;">
      <span class="badge badge-` + strings.ToLower(priority) + `">` + escapeHTML(priority) + ` PRIORITY</span>
      <div style="margin-top: 6px; font-size: 12px; color: #6b7280;">Generated: ` + time.Now().UTC().Format("2006-01-02 15:04:05 UTC") + `</div>
    </div>
  </div>

  <div class="section">
    <h2>1. Executive Summary</h2>
    <table>
      <tr><th style="width: 25%;">Classification</th><td>` + escapeHTML(classStr) + `</td><th style="width: 25%;">Status</th><td>` + escapeHTML(status) + `</td></tr>
      <tr><th>Created</th><td>` + createdAt.UTC().Format("2006-01-02 15:04:05 UTC") + `</td><th>Last Updated</th><td>` + updatedAt.UTC().Format("2006-01-02 15:04:05 UTC") + `</td></tr>
      <tr><th>Resolution Summary</th><td colspan="3">` + escapeHTML(resStr) + `</td></tr>
    </table>
  </div>

  <div class="section">
    <h2>2. Correlated Evidence & Alerts (` + strconv.Itoa(len(linkedAlerts)) + `)</h2>`)

	if len(linkedAlerts) == 0 {
		sb.WriteString(`<p style="color: #6b7280; font-style: italic;">No specific alerts linked to this case.</p>`)
	} else {
		sb.WriteString(`<table>
      <thead><tr><th>Alert ID</th><th>Severity</th><th>Description</th><th>Source IP</th><th>Dest IP</th><th>Count</th><th>Time</th></tr></thead>
      <tbody>`)
		for _, a := range linkedAlerts {
			src := "-"
			if s, ok := a["source_ip"].(*string); ok && s != nil {
				src = *s
			}
			dst := "-"
			if d, ok := a["dest_ip"].(*string); ok && d != nil {
				dst = *d
			}
			cAtStr := ""
			if t, ok := a["created_at"].(time.Time); ok {
				cAtStr = t.UTC().Format("15:04:05")
			}
			sb.WriteString(`<tr><td>#` + fmt.Sprint(a["alert_id"]) + `</td><td>` + fmt.Sprint(a["severity"]) + `</td><td>` + escapeHTML(fmt.Sprint(a["description"])) + `</td><td><code>` + escapeHTML(src) + `</code></td><td><code>` + escapeHTML(dst) + `</code></td><td>` + fmt.Sprint(a["event_count"]) + `</td><td>` + cAtStr + `</td></tr>`)
		}
		sb.WriteString(`</tbody></table>`)
	}

	sb.WriteString(`</div>

  <div class="section">
    <h2>3. SOAR Automated Containment & Remediation Actions (` + strconv.Itoa(len(soarActions)) + `)</h2>`)

	if len(soarActions) == 0 {
		sb.WriteString(`<p style="color: #6b7280; font-style: italic;">No automated or gated SOAR playbooks were triggered for this incident.</p>`)
	} else {
		sb.WriteString(`<table>
      <thead><tr><th>Execution ID</th><th>Playbook</th><th>Target Entity</th><th>Action</th><th>Approval Gated</th><th>Status</th></tr></thead>
      <tbody>`)
		for _, ex := range soarActions {
			reqApp := "No (Auto)"
			if r, ok := ex["require_approval"].(bool); ok && r {
				reqApp = "Yes (Analyst Confirmed)"
			}
			sb.WriteString(`<tr><td>#` + fmt.Sprint(ex["execution_id"]) + `</td><td>` + escapeHTML(fmt.Sprint(ex["playbook_name"])) + `</td><td><code>` + escapeHTML(fmt.Sprint(ex["target"])) + `</code></td><td>` + fmt.Sprint(ex["action_type"]) + `</td><td>` + reqApp + `</td><td><strong>` + fmt.Sprint(ex["status"]) + `</strong></td></tr>`)
		}
		sb.WriteString(`</tbody></table>`)
	}

	sb.WriteString(`</div>

  <div class="section">
    <h2>4. Investigation Notes & Audit Log (` + strconv.Itoa(len(caseNotes)) + `)</h2>`)

	if len(caseNotes) == 0 {
		sb.WriteString(`<p style="color: #6b7280; font-style: italic;">No investigation notes recorded.</p>`)
	} else {
		for _, n := range caseNotes {
			tStr := ""
			if t, ok := n["created_at"].(time.Time); ok {
				tStr = t.UTC().Format("2006-01-02 15:04:05 UTC")
			}
			sb.WriteString(`<div class="timeline-item"><div class="timeline-time">` + tStr + ` &bull; Analyst: ` + escapeHTML(fmt.Sprint(n["author_email"])) + `</div><div style="margin-top: 4px;">` + escapeHTML(fmt.Sprint(n["body"])) + `</div></div>`)
		}
	}

	sb.WriteString(`</div>

  <div class="sign-off">
    <div class="sign-box">Lead Incident Responder Signature<br><br><br>Date: ________________________</div>
    <div class="sign-box">Security Operations Manager Approval<br><br><br>Date: ________________________</div>
  </div>
</body>
</html>`)

	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(200)
	response.Write([]byte(sb.String()))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

