package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaseReportValidation(t *testing.T) {
	h := &Handler{}

	// Invalid case ID (negative or non-numeric)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cases/abc/report", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.caseReport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid id, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/cases/-1/report", nil)
	req.SetPathValue("id", "-1")
	rec = httptest.NewRecorder()
	h.caseReport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for negative id, got %d", rec.Code)
	}
}
