package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
)

func (handler *Handler) rulesRoute(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		handler.rules(response, request)
		return
	}
	claims := handler.claims(request)
	if claims.Role != "admin" {
		writeError(response, http.StatusForbidden, fmt.Errorf("admin role is required"))
		return
	}
	id, _ := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if request.Method == http.MethodDelete && id > 0 {
		_, err := handler.postgres.Exec(request.Context(), `DELETE FROM rules WHERE rule_id=$1`, id)
		if err != nil {
			writeError(response, 500, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
		return
	}
	var payload struct {
		Name, Description, RegexPattern, TargetField, Severity, Category string
		Condition                                                        map[string]any
		Enabled                                                          *bool
	}
	if json.NewDecoder(request.Body).Decode(&payload) != nil || payload.Name == "" || payload.RegexPattern == "" || payload.TargetField == "" {
		writeError(response, 400, fmt.Errorf("name, regex_pattern and target_field are required"))
		return
	}
	condition, _ := json.Marshal(payload.Condition)
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	if id > 0 {
		_, err := handler.postgres.Exec(request.Context(), `UPDATE rules SET name=$1, description=$2, regex_pattern=$3, target_field=$4, condition=$5, severity=$6, category=$7, enabled=$8 WHERE rule_id=$9`, payload.Name, payload.Description, payload.RegexPattern, payload.TargetField, condition, payload.Severity, payload.Category, enabled, id)
		if err != nil {
			writeError(response, 500, err)
			return
		}
		writeJSON(response, 200, map[string]any{"rule_id": id})
		return
	}
	var newID int64
	err := handler.postgres.QueryRow(request.Context(), `INSERT INTO rules (name, description, regex_pattern, target_field, condition, severity, category, enabled) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING rule_id`, payload.Name, payload.Description, payload.RegexPattern, payload.TargetField, condition, payload.Severity, payload.Category, enabled).Scan(&newID)
	if err != nil {
		writeError(response, 500, err)
		return
	}
	writeJSON(response, 201, map[string]any{"rule_id": newID})
}

func (handler *Handler) alertsRoute(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		handler.alerts(response, request)
		return
	}
	claims := handler.claims(request)
	if !allowed(claims.Role, "analyst") {
		writeError(response, 403, fmt.Errorf("analyst role is required"))
		return
	}
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil {
		writeError(response, 400, fmt.Errorf("invalid alert id"))
		return
	}
	var payload struct{ Status, AssignedTo string }
	if json.NewDecoder(request.Body).Decode(&payload) != nil || (payload.Status != "open" && payload.Status != "acknowledged" && payload.Status != "closed") {
		writeError(response, 400, fmt.Errorf("status must be open, acknowledged, or closed"))
		return
	}
	_, err = handler.postgres.Exec(request.Context(), `UPDATE alerts SET status=$1, assigned_to=NULLIF($2,'') WHERE alert_id=$3`, payload.Status, payload.AssignedTo, id)
	if err != nil {
		writeError(response, 500, err)
		return
	}
	writeJSON(response, 200, map[string]any{"alert_id": id, "status": payload.Status})
}

func (handler *Handler) usersRoute(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		rows, err := handler.postgres.Query(request.Context(), `SELECT user_id,email,display_name,role,enabled,created_at FROM users ORDER BY user_id`)
		if err != nil {
			writeError(response, 500, err)
			return
		}
		defer rows.Close()
		result := make([]map[string]any, 0)
		for rows.Next() {
			var id int64
			var email, name, role string
			var enabled bool
			var created any
			if err := rows.Scan(&id, &email, &name, &role, &enabled, &created); err != nil {
				writeError(response, 500, err)
				return
			}
			result = append(result, map[string]any{"user_id": id, "email": email, "display_name": name, "role": role, "enabled": enabled, "created_at": created})
		}
		writeJSON(response, 200, result)
		return
	}
	if request.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(request.PathValue("id"), 10, 64)
		_, err := handler.postgres.Exec(request.Context(), `UPDATE users SET enabled=false, updated_at=now() WHERE user_id=$1`, id)
		if err != nil {
			writeError(response, 500, err)
			return
		}
		response.WriteHeader(204)
		return
	}
	var payload struct{ Email, Password, DisplayName, Role string }
	if json.NewDecoder(request.Body).Decode(&payload) != nil || payload.Email == "" || payload.Role == "" {
		writeError(response, 400, fmt.Errorf("email and role are required"))
		return
	}
	if payload.Role != "admin" && payload.Role != "analyst" && payload.Role != "viewer" {
		writeError(response, 400, fmt.Errorf("invalid role"))
		return
	}
	hash, err := auth.HashPassword(payload.Password)
	if err != nil {
		writeError(response, 400, err)
		return
	}
	var id int64
	err = handler.postgres.QueryRow(request.Context(), `INSERT INTO users(email,password_hash,display_name,role) VALUES($1,$2,$3,$4) RETURNING user_id`, payload.Email, hash, payload.DisplayName, payload.Role).Scan(&id)
	if err != nil {
		writeError(response, 409, err)
		return
	}
	writeJSON(response, 201, map[string]any{"user_id": id, "email": payload.Email, "role": payload.Role})
}

func (handler *Handler) claims(request *http.Request) auth.Claims {
	token, _ := auth.Bearer(request.Header.Get("Authorization"))
	claims, _ := handler.auth.Verify(token)
	return claims
}
