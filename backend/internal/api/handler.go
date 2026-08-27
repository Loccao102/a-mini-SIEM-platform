package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	postgres *pgxpool.Pool
	elastic  *storage.Elasticsearch
	ingest   *ingest.Client
	auth     *auth.Manager
}

func New(postgres *pgxpool.Pool, elastic *storage.Elasticsearch, ingestClient *ingest.Client, authManager *auth.Manager) *Handler {
	return &Handler{postgres: postgres, elastic: elastic, ingest: ingestClient, auth: authManager}
}

func (handler *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.HandleFunc("POST /api/v1/ingest", handler.ingestLog)
	mux.Handle("GET /api/v1/events", handler.requireRole("viewer", http.HandlerFunc(handler.events)))
	mux.Handle("/api/v1/rules", handler.requireRole("viewer", http.HandlerFunc(handler.rulesRoute)))
	mux.Handle("/api/v1/rules/{id}", handler.requireRole("viewer", http.HandlerFunc(handler.rulesRoute)))
	mux.Handle("/api/v1/alerts", handler.requireRole("viewer", http.HandlerFunc(handler.alertsRoute)))
	mux.Handle("/api/v1/alerts/{id}", handler.requireRole("viewer", http.HandlerFunc(handler.alertsRoute)))
	mux.Handle("/api/v1/users", handler.requireRole("admin", http.HandlerFunc(handler.usersRoute)))
	mux.Handle("/api/v1/users/{id}", handler.requireRole("admin", http.HandlerFunc(handler.usersRoute)))
	return withCORS(mux)
}

func (handler *Handler) login(response http.ResponseWriter, request *http.Request) {
	var payload struct{ Email, Password string }
	if json.NewDecoder(request.Body).Decode(&payload) != nil || payload.Email == "" || payload.Password == "" {
		writeError(response, http.StatusBadRequest, fmt.Errorf("email and password are required"))
		return
	}
	var id int64
	var email, hash, role string
	var enabled bool
	err := handler.postgres.QueryRow(request.Context(), `SELECT user_id, email, password_hash, role, enabled FROM users WHERE lower(email) = lower($1)`, payload.Email).Scan(&id, &email, &hash, &role, &enabled)
	if err != nil || !enabled || auth.CheckPassword(hash, payload.Password) != nil {
		writeError(response, http.StatusUnauthorized, fmt.Errorf("invalid credentials"))
		return
	}
	token, err := handler.auth.Token(id, email, role)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"token": token, "user": map[string]any{"user_id": id, "email": email, "role": role}})
}

func (handler *Handler) requireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token, err := auth.Bearer(request.Header.Get("Authorization"))
		if err != nil {
			writeError(response, http.StatusUnauthorized, err)
			return
		}
		claims, err := handler.auth.Verify(token)
		if err != nil {
			writeError(response, http.StatusUnauthorized, err)
			return
		}
		if !allowed(claims.Role, role) {
			writeError(response, http.StatusForbidden, fmt.Errorf("role %s is required", role))
			return
		}
		next.ServeHTTP(response, request)
	})
}

func allowed(actual, required string) bool {
	if required == "viewer" {
		return actual == "viewer" || actual == "analyst" || actual == "admin"
	}
	if required == "analyst" {
		return actual == "analyst" || actual == "admin"
	}
	return actual == required
}

type ingestRequest struct {
	Message    string `json:"message"`
	SourceType string `json:"source_type"`
	Hostname   string `json:"hostname"`
	AgentID    string `json:"agent_id"`
	Event      struct {
		Original string `json:"original"`
	} `json:"event"`
	Host struct {
		Name string `json:"name"`
	} `json:"host"`
	Agent struct {
		ID string `json:"id"`
	} `json:"agent"`
}

func (handler *Handler) ingestLog(response http.ResponseWriter, request *http.Request) {
	var payload ingestRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.Message == "" {
		if payload.Message == "" {
			payload.Message = payload.Event.Original
		}
		if payload.Message == "" {
			writeError(response, http.StatusBadRequest, fmt.Errorf("message is required"))
			return
		}
	}
	if payload.SourceType == "" {
		payload.SourceType = "generic"
	}
	if payload.Hostname == "" {
		payload.Hostname = payload.Host.Name
	}
	if payload.Hostname == "" {
		payload.Hostname = "http-agent"
	}
	if payload.AgentID == "" {
		payload.AgentID = payload.Agent.ID
	}
	id, err := handler.ingest.Publish(request.Context(), ingest.Message{Raw: payload.Message, SourceType: payload.SourceType, Hostname: payload.Hostname, AgentID: payload.AgentID, ReceivedAt: time.Now()})
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusAccepted, map[string]string{"stream_id": id})
}

func (handler *Handler) events(response http.ResponseWriter, request *http.Request) {
	limit := 100
	if value, err := strconv.Atoi(request.URL.Query().Get("limit")); err == nil && value > 0 && value <= 500 {
		limit = value
	}
	result, err := handler.elastic.Search(request.Context(), map[string]any{"size": limit, "sort": []any{map[string]any{"event_time": "desc"}}, "query": map[string]any{"match_all": map[string]any{}}})
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (handler *Handler) rules(response http.ResponseWriter, request *http.Request) {
	rows, err := handler.postgres.Query(request.Context(), `SELECT rule_id, name, description, regex_pattern, target_field, condition, severity, category, enabled, created_at FROM rules ORDER BY rule_id`)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var name, pattern, field, severity, category string
		var description *string
		var condition []byte
		var enabled bool
		var createdAt any
		if err := rows.Scan(&id, &name, &description, &pattern, &field, &condition, &severity, &category, &enabled, &createdAt); err != nil {
			writeError(response, http.StatusInternalServerError, err)
			return
		}
		result = append(result, map[string]any{"rule_id": id, "name": name, "description": description, "regex_pattern": pattern, "target_field": field, "condition": json.RawMessage(condition), "severity": severity, "category": category, "enabled": enabled, "created_at": createdAt})
	}
	writeJSON(response, http.StatusOK, result)
}

func (handler *Handler) alerts(response http.ResponseWriter, request *http.Request) {
	rows, err := handler.postgres.Query(request.Context(), `SELECT alert_id, rule_id, asset_id, triggered_at, severity, status, assigned_to, summary FROM alerts ORDER BY triggered_at DESC LIMIT 100`)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var alertID, ruleID int64
		var assetID *int64
		var triggeredAt any
		var severity, status, summary string
		var assignedTo *string
		if err := rows.Scan(&alertID, &ruleID, &assetID, &triggeredAt, &severity, &status, &assignedTo, &summary); err != nil {
			writeError(response, http.StatusInternalServerError, err)
			return
		}
		result = append(result, map[string]any{"alert_id": alertID, "rule_id": ruleID, "asset_id": assetID, "triggered_at": triggeredAt, "severity": severity, "status": status, "assigned_to": assignedTo, "summary": summary})
	}
	writeJSON(response, http.StatusOK, result)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Access-Control-Allow-Origin", "*")
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(response, request)
	})
}
func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
func writeError(response http.ResponseWriter, status int, err error) {
	writeJSON(response, status, map[string]string{"error": err.Error()})
}
