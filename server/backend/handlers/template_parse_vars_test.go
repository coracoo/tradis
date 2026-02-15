package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExtractVarRefsSupportsDollarVar(t *testing.T) {
	refs := extractVarRefs("echo $A && echo ${B:-2} && echo $$HOME")
	seen := map[string]composeVarRef{}
	for _, r := range refs {
		seen[r.Name] = r
	}
	if _, ok := seen["A"]; !ok {
		t.Fatalf("expected A in refs")
	}
	if r, ok := seen["B"]; !ok || !r.HasDefault || r.Default != "2" {
		t.Fatalf("expected B default 2, got %+v", r)
	}
	if _, ok := seen["HOME"]; ok {
		t.Fatalf("expected HOME not extracted from $$HOME")
	}
}

func TestParsePortString(t *testing.T) {
	host, container, ok := parsePortString("127.0.0.1:8080:80/tcp")
	if !ok || host != "8080" || container != "80" {
		t.Fatalf("unexpected port parse: %v %s %s", ok, host, container)
	}
}

func TestParseTemplateVarsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/templates/parse-vars", ParseTemplateVars())

	compose := `
services:
  web:
    environment:
      APP_ENV: prod
      DB_PASS: ${DB_PASS:-p}
    ports:
      - "8080:80"
    volumes:
      - "./data:/data"
    command: ["sh", "-c", "echo $DB_HOST && echo $$HOME"]
`
	body, _ := json.Marshal(map[string]string{"compose": strings.TrimSpace(compose)})
	req := httptest.NewRequest(http.MethodPost, "/templates/parse-vars", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Schema   []map[string]interface{} `json:"schema"`
		Warnings []string                 `json:"warnings"`
		Errors   []string                 `json:"errors"`
		Refs     []composeVarRef          `json:"refs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("expected no errors, got: %+v", resp.Errors)
	}
	if len(resp.Schema) == 0 {
		t.Fatalf("expected schema not empty")
	}

	seenRef := map[string]composeVarRef{}
	for _, r := range resp.Refs {
		seenRef[r.Name] = r
	}
	if _, ok := seenRef["DB_HOST"]; !ok {
		t.Fatalf("expected DB_HOST in refs")
	}
	if r, ok := seenRef["DB_PASS"]; !ok || !r.HasDefault || r.Default != "p" {
		t.Fatalf("expected DB_PASS default p, got %+v", r)
	}
	if _, ok := seenRef["HOME"]; ok {
		t.Fatalf("expected HOME not extracted from $$HOME")
	}
}
