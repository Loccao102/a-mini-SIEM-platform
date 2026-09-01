package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/apikey"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dedup"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dlq"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/health"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/metrics"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ratelimit"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	postgres        *pgxpool.Pool
	elastic         *storage.Elasticsearch
	ingest          *ingest.Client
	auth            *auth.Manager
	dedup           *dedup.Manager
	apiKeyManager   *apikey.Manager
	rateLimiter     *ratelimit.Limiter
	dlqManager      *dlq.Manager
	healthChecker   *health.HealthChecker
	metrics         *metrics.QueueMetrics
}

func New(postgres *pgxpool.Pool, elastic *storage.Elasticsearch, ingestClient *ingest.Client, authManager *auth.Manager, dedupManager *dedup.Manager, apiKeyMgr *apikey.Manager, limiter *ratelimit.Limiter, dlqMgr *dlq.Manager, hc *health.HealthChecker, met *metrics.QueueMetrics) *Handler {
	return &Handler{
		postgres:      postgres,
		elastic:       elastic,
		ingest:        ingestClient,
		auth:          authManager,
		dedup:         dedupManager,
		apiKeyManager: apiKeyMgr,
		rateLimiter:   limiter,
		dlqManager:    dlqMgr,
		healthChecker: hc,
		metrics:       met,
	}
}

func (handler *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.Handle("GET /api/v1/auth/me", handler.requireRole("viewer", http.HandlerFunc(handler.getMe)))
	
	// Ingest endpoint with security middleware: auth + rate limit + size limit
	ingestSecured := handler.withRequestSizeLimit(MaxPayloadSize)(
		handler.withRateLimit(
			handler.withIngestAuth(
				http.HandlerFunc(handler.ingestLog),
			),
		),
	)
	mux.Handle("POST /api/v1/ingest", ingestSecured)
	
	mux.HandleFunc("POST /api/v1/fleet/agents", handler.fleetAgentEnroll)
	mux.Handle("POST /api/v1/rules/test-regex", handler.requireRole("viewer", http.HandlerFunc(handler.testRegex)))
	mux.Handle("GET /api/v1/summary", handler.requireRole("viewer", http.HandlerFunc(handler.summary)))
	mux.Handle("GET /api/v1/pipeline/status", handler.requireRole("viewer", http.HandlerFunc(handler.pipelineStatus)))
	mux.Handle("GET /api/v1/analytics", handler.requireRole("viewer", http.HandlerFunc(handler.analytics)))
	mux.Handle("GET /api/v1/events", handler.requireRole("viewer", http.HandlerFunc(handler.events)))
	mux.Handle("GET /api/v1/assets", handler.requireRole("viewer", http.HandlerFunc(handler.assets)))
	mux.Handle("POST /api/v1/assets", handler.requireRole("analyst", http.HandlerFunc(handler.createAsset)))
	mux.Handle("/api/v1/rules", handler.requireRole("viewer", http.HandlerFunc(handler.rulesRoute)))
	mux.Handle("/api/v1/rules/{id}", handler.requireRole("viewer", http.HandlerFunc(handler.rulesRoute)))
	mux.Handle("/api/v1/alerts", handler.requireRole("viewer", http.HandlerFunc(handler.alertsRoute)))
	mux.Handle("/api/v1/alerts/{id}", handler.requireRole("viewer", http.HandlerFunc(handler.alertsRoute)))
	mux.Handle("/api/v1/cases", handler.requireRole("viewer", http.HandlerFunc(handler.casesRoute)))
	mux.Handle("/api/v1/cases/{id}", handler.requireRole("viewer", http.HandlerFunc(handler.casesRoute)))
	mux.Handle("/api/v1/cases/{id}/notes", handler.requireRole("analyst", http.HandlerFunc(handler.caseNotesRoute)))
	mux.Handle("/api/v1/cases/{id}/timeline", handler.requireRole("viewer", http.HandlerFunc(handler.caseTimeline)))
	mux.Handle("/api/v1/cases/{id}/alerts/{alert_id}", handler.requireRole("analyst", http.HandlerFunc(handler.caseAlertRoute)))
	mux.Handle("/api/v1/users", handler.requireRole("admin", http.HandlerFunc(handler.usersRoute)))
	mux.Handle("/api/v1/users/{id}", handler.requireRole("admin", http.HandlerFunc(handler.usersRoute)))
	
	// Health and monitoring endpoints (no auth required)
	mux.HandleFunc("GET /healthz", handler.handleHealthz)
	mux.HandleFunc("GET /metrics", handler.handleMetrics)
	
	// DLQ management endpoints
	mux.Handle("GET /api/v1/dlq/stats", handler.requireRole("analyst", http.HandlerFunc(handler.handleDLQStats)))
	mux.Handle("POST /api/v1/dlq/replay/{messageId}", handler.requireRole("analyst", http.HandlerFunc(handler.handleDLQReplay)))
	mux.Handle("DELETE /api/v1/dlq/purge", handler.requireRole("admin", http.HandlerFunc(handler.handleDLQPurge)))
	
	return withCORS(mux)
}

func (handler *Handler) login(response http.ResponseWriter, request *http.Request) {
	var payload struct{ Email, Password string }
	if json.NewDecoder(request.Body).Decode(&payload) != nil || payload.Email == "" || payload.Password == "" {
		writeError(response, http.StatusBadRequest, fmt.Errorf("email and password are required"))
		return
	}
	payload.Email = strings.TrimSpace(payload.Email)
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

type fleetEnrollmentRequest struct {
	AgentID     string            `json:"agent_id"`
	Hostname    string            `json:"hostname"`
	IPAddress   *string           `json:"ip_address"`
	OSType      string            `json:"os_type"`
	SourceTypes []string          `json:"source_types"`
	Tags        map[string]string `json:"tags"`
}

func normalizeFleetEnrollment(payload fleetEnrollmentRequest) (fleetEnrollmentRequest, error) {
	payload.Hostname = strings.TrimSpace(payload.Hostname)
	payload.OSType = strings.TrimSpace(strings.ToLower(payload.OSType))
	if payload.Hostname == "" {
		return payload, fmt.Errorf("hostname is required")
	}
	if payload.OSType == "" {
		payload.OSType = "linux"
	}
	if payload.AgentID == "" {
		payload.AgentID = "fleet-agent"
	}
	if len(payload.SourceTypes) == 0 {
		payload.SourceTypes = []string{"elastic_agent"}
	}
	if payload.Tags == nil {
		payload.Tags = map[string]string{}
	}
	payload.Tags["fleet.agent_id"] = payload.AgentID
	return payload, nil
}

func (handler *Handler) fleetAgentEnroll(response http.ResponseWriter, request *http.Request) {
	var payload fleetEnrollmentRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(response, http.StatusBadRequest, fmt.Errorf("invalid fleet enrollment payload"))
		return
	}
	payload, err := normalizeFleetEnrollment(payload)
	if err != nil {
		writeError(response, http.StatusBadRequest, err)
		return
	}
	assetID, err := handler.upsertFleetAsset(request.Context(), payload)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	for _, sourceType := range payload.SourceTypes {
		if err := handler.registerLogSourceWithAssetID(request.Context(), assetID, sourceType, payload.AgentID); err != nil {
			writeError(response, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(response, http.StatusAccepted, map[string]any{
		"status":       "registered",
		"asset_id":     assetID,
		"hostname":     payload.Hostname,
		"agent_id":     payload.AgentID,
		"source_types": payload.SourceTypes,
	})
}

func (handler *Handler) upsertFleetAsset(ctx context.Context, payload fleetEnrollmentRequest) (int64, error) {
	tagBytes, err := json.Marshal(payload.Tags)
	if err != nil {
		return 0, err
	}
	var assetID int64
	if payload.IPAddress != nil {
		_, err = handler.postgres.Exec(ctx, `
			INSERT INTO assets (hostname, ip_address, os_type, criticality, owner, tags)
			VALUES ($1, $2::inet, $3, 'medium', NULL, $4::jsonb)
			ON CONFLICT (hostname) DO UPDATE SET
				ip_address = EXCLUDED.ip_address,
				os_type = EXCLUDED.os_type,
				tags = COALESCE(assets.tags, '{}'::jsonb) || EXCLUDED.tags
			RETURNING asset_id
		`, payload.Hostname, strings.TrimSpace(*payload.IPAddress), payload.OSType, string(tagBytes))
		if err != nil {
			return 0, err
		}
	} else {
		err = handler.postgres.QueryRow(ctx, `
			INSERT INTO assets (hostname, ip_address, os_type, criticality, owner, tags)
			VALUES ($1, NULL, $2, 'medium', NULL, $3::jsonb)
			ON CONFLICT (hostname) DO UPDATE SET
				os_type = EXCLUDED.os_type,
				tags = COALESCE(assets.tags, '{}'::jsonb) || EXCLUDED.tags
			RETURNING asset_id
		`, payload.Hostname, payload.OSType, string(tagBytes)).Scan(&assetID)
		if err != nil {
			return 0, err
		}
	}
	if payload.IPAddress == nil {
		if err := handler.postgres.QueryRow(ctx, `SELECT asset_id FROM assets WHERE LOWER(hostname) = LOWER($1)`, payload.Hostname).Scan(&assetID); err != nil {
			return 0, err
		}
	}
	return assetID, nil
}

func (handler *Handler) registerLogSourceWithAssetID(ctx context.Context, assetID int64, sourceType, agentID string) error {
	result, err := handler.postgres.Exec(ctx, `
		INSERT INTO log_sources (asset_id, source_type, agent_id, status, last_seen)
		VALUES ($1, $2, $3, 'active', now())
		ON CONFLICT (asset_id, source_type) DO UPDATE SET
			agent_id = EXCLUDED.agent_id,
			status = 'active',
			last_seen = now()
	`, assetID, sourceType, agentID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return nil
	}
	return nil
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
	payload.Hostname = strings.TrimSpace(payload.Hostname)
	if payload.AgentID == "" {
		payload.AgentID = payload.Agent.ID
	}
	if err := handler.registerLogSource(request, payload.Hostname, payload.SourceType, payload.AgentID); err != nil {
		if err == pgx.ErrNoRows {
			writeError(response, http.StatusUnprocessableEntity, fmt.Errorf("asset %q is not registered", payload.Hostname))
			return
		}
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	id, err := handler.ingest.Publish(request.Context(), ingest.Message{Raw: payload.Message, SourceType: payload.SourceType, Hostname: payload.Hostname, AgentID: payload.AgentID, ReceivedAt: time.Now()})
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusAccepted, map[string]string{"stream_id": id})
}

func (handler *Handler) registerLogSource(request *http.Request, hostname, sourceType, agentID string) error {
	hostname = strings.TrimSpace(hostname)
	var assetID int64
	err := handler.postgres.QueryRow(request.Context(), `SELECT asset_id FROM assets WHERE LOWER(hostname) = LOWER($1)`, hostname).Scan(&assetID)
	if err == nil {
		result, err := handler.postgres.Exec(request.Context(), `UPDATE log_sources SET status='active', agent_id=$1, last_seen=now() WHERE asset_id=$2 AND source_type=$3`, agentID, assetID, sourceType)
		if err != nil || result.RowsAffected() > 0 {
			return err
		}
		_, err = handler.postgres.Exec(request.Context(), `INSERT INTO log_sources (asset_id, source_type, agent_id, status, last_seen) VALUES ($1, $2, $3, 'active', now())`, assetID, sourceType, agentID)
		return err
	}
	if err != pgx.ErrNoRows {
		return err
	}
	return pgx.ErrNoRows
}

type createAssetRequest struct {
	Hostname    string  `json:"hostname"`
	IPAddress   *string `json:"ip_address"`
	OSType      string  `json:"os_type"`
	Criticality string  `json:"criticality"`
	Owner       *string `json:"owner"`
}

func (handler *Handler) createAsset(response http.ResponseWriter, request *http.Request) {
	var payload createAssetRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(response, http.StatusBadRequest, fmt.Errorf("invalid asset payload"))
		return
	}
	payload.Hostname = strings.TrimSpace(payload.Hostname)
	payload.OSType = strings.TrimSpace(strings.ToLower(payload.OSType))
	payload.Criticality = strings.TrimSpace(strings.ToLower(payload.Criticality))
	if payload.Hostname == "" || payload.OSType == "" {
		writeError(response, http.StatusBadRequest, fmt.Errorf("hostname and os_type are required"))
		return
	}
	if payload.Criticality == "" {
		payload.Criticality = "medium"
	}
	if payload.Criticality != "critical" && payload.Criticality != "high" && payload.Criticality != "medium" && payload.Criticality != "low" {
		writeError(response, http.StatusBadRequest, fmt.Errorf("criticality must be critical, high, medium, or low"))
		return
	}
	claims := handler.claims(request)
	transaction, err := handler.postgres.Begin(request.Context())
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	defer transaction.Rollback(request.Context())
	var assetID int64
	err = transaction.QueryRow(request.Context(), `INSERT INTO assets (hostname, ip_address, os_type, criticality, owner) VALUES ($1, NULLIF($2, '')::inet, $3, $4, NULLIF($5, '')) RETURNING asset_id`, payload.Hostname, valueOrEmpty(payload.IPAddress), payload.OSType, payload.Criticality, valueOrEmpty(payload.Owner)).Scan(&assetID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(response, http.StatusConflict, fmt.Errorf("asset %q already exists", payload.Hostname))
			return
		}
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	if err = recordAudit(request.Context(), transaction, claims.UserID, "asset.created", "asset", assetID, map[string]any{"hostname": payload.Hostname, "os_type": payload.OSType}); err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	if err = transaction.Commit(request.Context()); err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"asset_id": assetID, "hostname": payload.Hostname})
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (handler *Handler) events(response http.ResponseWriter, request *http.Request) {
	values := request.URL.Query()
	page, _ := strconv.Atoi(values.Get("page"))
	if page < 1 { page = 1 }
	pageSize, _ := strconv.Atoi(values.Get("page_size"))
	if pageSize != 25 && pageSize != 50 && pageSize != 100 { pageSize = 25 }
	must := make([]any, 0, 5)
	if query := strings.TrimSpace(values.Get("q")); query != "" {
		// Build bool query with should clauses for both exact match and prefix/wildcard match
		should := make([]any, 0)
		// Exact word match on multiple fields
		should = append(should, map[string]any{"multi_match": map[string]any{"query": query, "fields": []string{"message", "hostname", "log_category", "event_type", "username"}}})
		// Wildcard match for substring search (case-insensitive via lowercase)
		lowerQuery := strings.ToLower(query)
		should = append(should, map[string]any{"wildcard": map[string]any{"message": map[string]any{"value": "*" + lowerQuery + "*", "case_insensitive": true}}})
		should = append(should, map[string]any{"wildcard": map[string]any{"hostname": map[string]any{"value": "*" + lowerQuery + "*", "case_insensitive": true}}})
		
		must = append(must, map[string]any{"bool": map[string]any{"should": should, "minimum_should_match": 1}})
	}
	for field := range map[string]bool{"severity.keyword": true, "log_category.keyword": true, "hostname.keyword": true} {
		param := strings.TrimSuffix(field, ".keyword")
		if value := strings.TrimSpace(values.Get(param)); value != "" { must = append(must, map[string]any{"term": map[string]any{field: value}}) }
	}
	if from, to := values.Get("from"), values.Get("to"); from != "" || to != "" {
		rangeQuery := map[string]any{}
		if from != "" { rangeQuery["gte"] = from }
		if to != "" { rangeQuery["lte"] = to }
		must = append(must, map[string]any{"range": map[string]any{"event_time": rangeQuery}})
	}
	query := map[string]any{"from": (page - 1) * pageSize, "size": pageSize, "track_total_hits": true, "sort": []any{map[string]any{"event_time": "desc"}}, "query": map[string]any{"bool": map[string]any{"must": must}}}
	result, err := handler.elastic.Search(request.Context(), query)
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	var payload struct {
		Hits struct {
			Total struct { Value int64 `json:"value"` } `json:"total"`
			Hits []struct { Source map[string]any `json:"_source"` } `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(result, &payload); err != nil { writeError(response, http.StatusBadGateway, err); return }
	items := make([]map[string]any, 0, len(payload.Hits.Hits))
	for _, hit := range payload.Hits.Hits {
		event := hit.Source
		// Enrich event với dedup metadata từ Redis
		if fingerprint, ok := event["fingerprint"].(string); ok && fingerprint != "" {
			group, err := handler.dedup.GetGroup(request.Context(), fingerprint)
			if err == nil && group != nil {
				// Update event với dedup info từ Redis
				event["duplicate_count"] = group.Count
				event["first_seen"] = group.FirstSeen
				event["last_seen"] = group.LastSeen
			}
		}
		items = append(items, event)
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": items, "total": payload.Hits.Total.Value, "page": page, "page_size": pageSize})
}

func (handler *Handler) summary(response http.ResponseWriter, request *http.Request) {
	var openAlerts, assets int64
	if err := handler.postgres.QueryRow(request.Context(), `SELECT COUNT(*) FROM alerts WHERE status = 'open'`).Scan(&openAlerts); err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	if err := handler.postgres.QueryRow(request.Context(), `SELECT COUNT(*) FROM assets`).Scan(&assets); err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	events, err := handler.elastic.Count(request.Context())
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]int64{"open_alerts": openAlerts, "events_processed": events, "connected_assets": assets})
}

func (handler *Handler) pipelineStatus(response http.ResponseWriter, request *http.Request) {
	status, err := handler.ingest.Status(request.Context(), parser.DefaultConsumerGroup)
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"stream":         ingest.DefaultStream,
		"consumer_group": parser.DefaultConsumerGroup,
		"stream_length":  status.StreamLength,
		"pending":        status.Pending,
		"consumers":      status.Consumers,
		"last_id":        status.LastID,
	})
}

func (handler *Handler) assets(response http.ResponseWriter, request *http.Request) {
	rows, err := handler.postgres.Query(request.Context(), `SELECT asset_id, hostname, ip_address::text, os_type, criticality, owner, created_at FROM assets ORDER BY criticality, hostname`)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var hostname, osType, criticality string
		var ipAddress, owner *string
		var createdAt any
		if err := rows.Scan(&id, &hostname, &ipAddress, &osType, &criticality, &owner, &createdAt); err != nil {
			writeError(response, http.StatusInternalServerError, err)
			return
		}
		ip := ""
		if ipAddress != nil {
			ip = *ipAddress
		}
		result = append(result, map[string]any{"asset_id": id, "hostname": hostname, "ip_address": ip, "os_type": osType, "criticality": criticality, "owner": owner, "created_at": createdAt})
	}
	if err := rows.Err(); err != nil {
		writeError(response, http.StatusInternalServerError, err)
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
	rows, err := handler.postgres.Query(request.Context(), `SELECT alert_id, rule_id, asset_id, triggered_at, COALESCE(last_seen, triggered_at), COALESCE(occurrences, 1), COALESCE(entity_key, ''), severity, status, assigned_to, summary FROM alerts ORDER BY last_seen DESC LIMIT 100`)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var alertID, ruleID int64
		var assetID *int64
		var triggeredAt, lastSeen any
		var occurrences int
		var entityKey, severity, status, summary string
		var assignedTo *string
		if err := rows.Scan(&alertID, &ruleID, &assetID, &triggeredAt, &lastSeen, &occurrences, &entityKey, &severity, &status, &assignedTo, &summary); err != nil {
			writeError(response, http.StatusInternalServerError, err)
			return
		}
		result = append(result, map[string]any{
			"alert_id":     alertID,
			"rule_id":      ruleID,
			"asset_id":     assetID,
			"triggered_at": triggeredAt,
			"last_seen":    lastSeen,
			"occurrences":  occurrences,
			"entity_key":   entityKey,
			"severity":     severity,
			"status":       status,
			"assigned_to":  assignedTo,
			"summary":      summary,
		})
	}
	writeJSON(response, http.StatusOK, result)
}

func (handler *Handler) getMe(response http.ResponseWriter, request *http.Request) {
	claims := handler.claims(request)
	if claims.UserID <= 0 {
		writeError(response, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}
	var email, name, role string
	err := handler.postgres.QueryRow(request.Context(), `SELECT email, display_name, role FROM users WHERE user_id=$1`, claims.UserID).Scan(&email, &name, &role)
	if err != nil {
		writeError(response, http.StatusNotFound, fmt.Errorf("user not found"))
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"user_id":      claims.UserID,
		"email":        email,
		"display_name": name,
		"role":         role,
	})
}

func (handler *Handler) testRegex(response http.ResponseWriter, request *http.Request) {
	var payload struct {
		Pattern     string `json:"pattern"`
		Log         string `json:"log"`
		TargetField string `json:"target_field"`
	}
	if json.NewDecoder(request.Body).Decode(&payload) != nil || strings.TrimSpace(payload.Pattern) == "" {
		writeError(response, http.StatusBadRequest, fmt.Errorf("pattern and log sample are required"))
		return
	}
	re, err := regexp.Compile(payload.Pattern)
	if err != nil {
		writeError(response, http.StatusBadRequest, fmt.Errorf("invalid regex: %w", err))
		return
	}
	matches := re.FindStringSubmatch(payload.Log)
	if matches == nil {
		writeJSON(response, http.StatusOK, map[string]any{
			"matched": false,
			"groups":  []string{},
			"pattern": payload.Pattern,
		})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"matched": true,
		"groups":  matches,
		"pattern": payload.Pattern,
	})
}

func (handler *Handler) analytics(response http.ResponseWriter, request *http.Request) {
	now := time.Now().UTC()
	result, err := handler.elastic.Search(request.Context(), map[string]any{
		"size":             0,
		"track_total_hits": true,
		"query":            map[string]any{"match_all": map[string]any{}},
		"aggs": map[string]any{
			"events_by_severity": map[string]any{"terms": map[string]any{"field": "severity.keyword", "size": 20}},
			"events_by_category": map[string]any{"terms": map[string]any{"field": "log_category.keyword", "size": 20}},
			"top_attacking_ips": map[string]any{
				"terms": map[string]any{"field": "src_ip.keyword", "size": 5, "exclude": ""},
				"aggs":  map[string]any{"sample": map[string]any{"top_hits": map[string]any{"size": 1, "_source": []string{"extra_fields"}}}},
			},
			"top_targeted_users": map[string]any{"terms": map[string]any{"field": "username.keyword", "size": 5, "exclude": ""}},
			"timeline_last_24h": map[string]any{
				"filter": map[string]any{"range": map[string]any{"event_time": map[string]any{"gte": now.Add(-24 * time.Hour).Format(time.RFC3339), "lte": now.Format(time.RFC3339)}}},
				"aggs":   map[string]any{"buckets": map[string]any{"date_histogram": map[string]any{"field": "event_time", "fixed_interval": "4h", "min_doc_count": 0, "extended_bounds": map[string]any{"min": now.Add(-24 * time.Hour).Format(time.RFC3339), "max": now.Format(time.RFC3339)}}}},
			},
		},
	})
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	var analytics struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations struct {
			Severity struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"events_by_severity"`
			Category struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"events_by_category"`
			Attackers struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
					Sample   struct {
						Hits struct {
							Hits []struct {
								Source struct {
									Extra map[string]string `json:"extra_fields"`
								} `json:"_source"`
							} `json:"hits"`
						} `json:"hits"`
					} `json:"sample"`
				} `json:"buckets"`
			} `json:"top_attacking_ips"`
			Users struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"top_targeted_users"`
			Timeline struct {
				Buckets struct {
					Buckets []struct {
						KeyAsString string `json:"key_as_string"`
						Key         int64  `json:"key"`
						DocCount    int64  `json:"doc_count"`
					} `json:"buckets"`
				} `json:"buckets"`
			} `json:"timeline_last_24h"`
		} `json:"aggregations"`
	}
	if err := json.Unmarshal(result, &analytics); err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	eventsBySeverity := map[string]int64{}
	for _, bucket := range analytics.Aggregations.Severity.Buckets {
		eventsBySeverity[bucket.Key] = bucket.DocCount
	}
	eventsByCategory := map[string]int64{}
	for _, bucket := range analytics.Aggregations.Category.Buckets {
		eventsByCategory[bucket.Key] = bucket.DocCount
	}
	topAttackerIPs := make([]map[string]any, 0, len(analytics.Aggregations.Attackers.Buckets))
	for _, bucket := range analytics.Aggregations.Attackers.Buckets {
		entry := map[string]any{"ip": bucket.Key, "count": bucket.DocCount}
		if len(bucket.Sample.Hits.Hits) > 0 {
			for key, value := range bucket.Sample.Hits.Hits[0].Source.Extra {
				switch key {
				case "country", "country_code", "threat_level", "threat_category":
					entry[key] = value
				case "reputation_score":
					if score, parseErr := strconv.Atoi(value); parseErr == nil {
						entry[key] = score
					}
				}
			}
		}
		topAttackerIPs = append(topAttackerIPs, entry)
	}
	topTargetedUsers := make([]map[string]any, 0, len(analytics.Aggregations.Users.Buckets))
	for _, bucket := range analytics.Aggregations.Users.Buckets {
		topTargetedUsers = append(topTargetedUsers, map[string]any{"username": bucket.Key, "count": bucket.DocCount})
	}
	timeline := make([]map[string]any, 0, len(analytics.Aggregations.Timeline.Buckets.Buckets))
	for _, bucket := range analytics.Aggregations.Timeline.Buckets.Buckets {
		timestamp := bucket.KeyAsString
		if timestamp == "" {
			timestamp = time.UnixMilli(bucket.Key).UTC().Format(time.RFC3339)
		}
		timeline = append(timeline, map[string]any{"time": timestamp, "count": bucket.DocCount})
	}

	writeJSON(response, http.StatusOK, map[string]any{
		"total_events":       analytics.Hits.Total.Value,
		"events_by_severity": eventsBySeverity,
		"events_by_category": eventsByCategory,
		"top_attacking_ips":  topAttackerIPs,
		"top_targeted_users": topTargetedUsers,
		"timeline":           timeline,
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Access-Control-Allow-Origin", "*")
		response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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
