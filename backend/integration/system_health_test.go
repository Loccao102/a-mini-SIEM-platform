//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProtectedSystemHealthIntegration(t *testing.T) {
	base := env("INTEGRATION_API_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 20 * time.Second}
	request := func(path,token string) (int,[]byte) {
		t.Helper()
		req,err := http.NewRequest(http.MethodGet,base+path,nil)
		if err!=nil {t.Fatal(err)}
		if token!="" {req.Header.Set("Authorization","Bearer "+token)}
		res,err:=client.Do(req)
		if err!=nil {t.Fatal(err)}
		defer res.Body.Close()
		body,err:=io.ReadAll(io.LimitReader(res.Body,1<<20))
		if err!=nil {t.Fatal(err)}
		return res.StatusCode,body
	}
	code,public := request("/healthz","")
	if code!=http.StatusOK && code!=http.StatusServiceUnavailable {
		t.Fatalf("public health returned unexpected status %d",code)
	}
	var publicData map[string]any
	if err:=json.Unmarshal(public,&publicData);err!=nil {t.Fatal(err)}
	if publicData["status"]==nil {t.Fatal("public health has no status")}
	for _,sensitive:=range []string{"services","checks","message"} {
		if _,exposed:=publicData[sensitive];exposed {
			t.Fatalf("public health exposed %q",sensitive)
		}
	}
	code,probe:=request("/healthz/redis","")
	if code!=http.StatusOK&&code!=http.StatusServiceUnavailable {
		t.Fatalf("redis probe status %d",code)
	}
	if strings.Contains(string(probe),"checks")||strings.Contains(string(probe),"message") {
		t.Fatal("redis probe exposed detailed diagnostics")
	}
	for _,path:=range []string{"/api/v1/system/health","/metrics"} {
		status,_:=request(path,"")
		if status!=http.StatusUnauthorized {t.Fatalf("unauthorized %s returned %d",path,status)}
	}

	loginJSON,_:=json.Marshal(map[string]string{
		"email":env("INTEGRATION_ADMIN_EMAIL",env("ADMIN_EMAIL","admin@example.com")),
		"password":env("INTEGRATION_ADMIN_PASSWORD",env("ADMIN_PASSWORD","admin")),
	})
	res,err:=client.Post(base+"/api/v1/auth/login","application/json",bytes.NewReader(loginJSON))
	if err!=nil {t.Fatal(err)}
	defer res.Body.Close()
	if res.StatusCode!=http.StatusOK {t.Fatalf("admin login failed: %d",res.StatusCode)}
	var auth struct {Token string `json:"token"`}
	if err:=json.NewDecoder(res.Body).Decode(&auth);err!=nil {t.Fatal(err)}
	if auth.Token=="" {t.Fatal("admin JWT missing")}

	code,details:=request("/api/v1/system/health",auth.Token)
	if code!=http.StatusOK {t.Fatalf("admin system health HTTP %d: %s",code,details)}
	var diagnostics struct {
		Status string `json:"status"`
		Services map[string]struct{
			Status string `json:"status"`
			ResponseTime int64 `json:"response_time_ms"`
		} `json:"services"`
	}
	if err:=json.Unmarshal(details,&diagnostics);err!=nil {t.Fatal(err)}
	if diagnostics.Status==""||len(diagnostics.Services)<4 {
		t.Fatalf("missing detailed health: %#v",diagnostics)
	}
	for _,name:=range []string{"postgres","redis","elasticsearch","database","queues","connectors"} {
		if diagnostics.Services[name].Status=="" {t.Errorf("missing service: %s",name)}
	}
	code,_=request("/metrics",auth.Token)
	if code!=http.StatusOK {t.Fatalf("admin metrics returned %d",code)}
	code,_=request("/api/v1/system/health?token="+url.QueryEscape(auth.Token),"")
	if code!=http.StatusUnauthorized {
		t.Fatalf("JWT in query was accepted (status %d)",code)
	}
}
