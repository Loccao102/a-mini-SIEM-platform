package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
)

func TestAllRoleProtectedRoutesRejectQueryJWTs(t *testing.T){
	manager:=auth.NewManager("test-secret-32-chars-only-for-ci")
	h:=&Handler{auth:manager}
	token,err:=manager.Token(7,"analyst@example.org","analyst")
	if err!=nil {t.Fatal(err)}
	reached:=0
	handler:=h.requireRole("viewer",http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		reached++
		w.WriteHeader(http.StatusOK)
	}))
	for _,scenario:=range []struct{name,url,authorization string;want int}{
		{"query only","/api/v1/cases/5/report?token="+token,"",http.StatusUnauthorized},
		{"query and valid header","/api/v1/cases/5/report?token="+token,"Bearer "+token,http.StatusUnauthorized},
		{"header only","/api/v1/cases/5/report","Bearer "+token,http.StatusOK},
		{"missing header","/api/v1/cases/5/report","",http.StatusUnauthorized},
		{"bad header","/api/v1/cases/5/report","Bearer invalid",http.StatusUnauthorized},
	}{
		t.Run(scenario.name,func(t *testing.T){
			r:=httptest.NewRequest(http.MethodGet,scenario.url,nil)
			if scenario.authorization!="" {r.Header.Set("Authorization",scenario.authorization)}
			record:=httptest.NewRecorder()
			handler.ServeHTTP(record,r)
			if record.Code!=scenario.want {t.Fatalf("HTTP %d, expected %d",record.Code,scenario.want)}
		})
	}
	if reached!=1 {t.Fatalf("expected only one request to reach protected endpoint, got %d",reached)}
}
