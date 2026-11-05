package api

import (
  "strings"
  "testing"
)

func TestIncidentReportEscapingUntrustedMarkup(t *testing.T){
  input := "<img src=x onerror=\"alert(1)\">&'\""
  got:=escapeHTML(input)
  for _,piece:=range []string{"<img","onerror=\"","<script"} {
    if strings.Contains(got,piece) {t.Fatalf("unescaped markup: %q",got)}
  }
  for _,piece:=range []string{"&lt;img","&quot;","&#39;","&amp;"} {
    if !strings.Contains(got,piece) {t.Fatalf("missing HTML escape %s: %q",piece,got)}
  }
}
