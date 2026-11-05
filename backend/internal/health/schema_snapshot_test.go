package health

import "testing"

func TestAnnotateRequiredTablesReportsAllMissingInOneSnapshot(t *testing.T) {
	required := []string{"users", "assets", "api_keys", "rules", "alerts"}
	checks := map[string]string{"migrations": "ok"}
	present := map[string]bool{"users": true, "assets": true, "alerts": true}
	missing := annotateRequiredTables(checks, required, present)
	if missing != "api_keys" { t.Fatalf("expected first missing api_keys, got %q", missing) }
	if checks["migrations"] != "ok" { t.Fatal("existing checks must be preserved") }
	for _, key := range []string{"api_keys", "rules"} {
		if checks[key] != "missing" { t.Errorf("%s should be missing, got %q", key, checks[key]) }
	}
	for _, key := range []string{"users", "assets", "alerts"} {
		if checks[key] != "ok" { t.Errorf("%s should be present, got %q", key, checks[key]) }
	}
}

func TestAnnotateRequiredTablesHandlesHealthyAndEmptyCatalog(t *testing.T) {
	required := []string{"users", "assets"}
	checks := map[string]string{}
	if missing := annotateRequiredTables(checks, required, map[string]bool{"users": true, "assets": true}); missing != "" {
		t.Fatalf("all tables present but reported %q", missing)
	}
	checks = map[string]string{}
	if missing := annotateRequiredTables(checks, required, nil); missing != "users" {
		t.Fatalf("empty table catalog must fail safe; got %q", missing)
	}
}
