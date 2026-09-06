package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEventSearchBounds(t *testing.T) {
	for _, raw := range []string{"page=-1", "page=999999999", "page_size=200", "from=yesterday", "from=2026-09-06T00:00:00Z&to=2026-09-05T00:00:00Z"} {
		v, _ := url.ParseQuery(raw)
		if _, _, _, err := eventSearchQuery(v); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	v, _ := url.ParseQuery("hostname=lab&severity=high&page=2&page_size=50&from=2026-09-05T00:00:00Z")
	q, p, size, err := eventSearchQuery(v)
	if err != nil || p != 2 || size != 50 || q["from"] != 50 {
		t.Fatalf("%v %v", q, err)
	}
	if len(q["sort"].([]any)) != 2 {
		t.Fatal("missing stable tiebreaker")
	}
}
func TestChunkedBodyLimit(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader("123456789"))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	h := &Handler{}
	h.withRequestSizeLimit(8)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("oversize reached handler") })).ServeHTTP(response, request)
	if response.Code != 413 {
		t.Fatal(response.Code)
	}
}
