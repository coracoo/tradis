package api

import (
	"strconv"
	"strings"
	"testing"
)

func TestExtractComposeVarRefs(t *testing.T) {
	compose := `
services:
  db:
    image: postgres
    environment:
      - POSTGRES_PASSWORD=${DB_PASS:-p}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $DB_USER -d ${DB_NAME}"]
    labels:
      - "traefik.http.routers.db.rule=Host(${DB_HOST})"
  web:
    command: ["sh", "-c", "echo $$HOME && echo $WEB_HOST && echo ${WEB_PORT:-8080}"]
`
	refs := extractComposeVarRefs(compose)

	if _, ok := refs["DB_PASS"]; !ok {
		t.Fatalf("expected DB_PASS in refs")
	}
	if !refs["DB_PASS"].HasDefault || refs["DB_PASS"].DefaultValue != "p" {
		t.Fatalf("expected DB_PASS default p, got %+v", refs["DB_PASS"])
	}
	if _, ok := refs["DB_USER"]; !ok {
		t.Fatalf("expected DB_USER in refs")
	}
	if _, ok := refs["DB_NAME"]; !ok {
		t.Fatalf("expected DB_NAME in refs")
	}
	if _, ok := refs["DB_HOST"]; !ok {
		t.Fatalf("expected DB_HOST in refs")
	}
	if _, ok := refs["WEB_HOST"]; !ok {
		t.Fatalf("expected WEB_HOST in refs")
	}
	if _, ok := refs["WEB_PORT"]; !ok {
		t.Fatalf("expected WEB_PORT in refs")
	}
	if _, ok := refs["HOME"]; ok {
		t.Fatalf("expected HOME not to be extracted from $$HOME")
	}
}

func TestExtractComposeInterpolationKeysIncludesDollarVar(t *testing.T) {
	compose := "services:\n  a:\n    command: echo $A && echo ${B:-2}\n"
	keys := extractComposeInterpolationKeys(compose)
	if _, ok := keys["A"]; !ok {
		t.Fatalf("expected A key")
	}
	if _, ok := keys["B"]; !ok {
		t.Fatalf("expected B key")
	}
}

func TestExtractComposeVarRefsMaxVars(t *testing.T) {
	var b strings.Builder
	b.WriteString("services:\n  a:\n    command: |\n      echo ")
	for i := 0; i < 1200; i++ {
		b.WriteString("${K_")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("}\n      echo ")
	}
	refs := extractComposeVarRefs(b.String())
	if len(refs) > 500 {
		t.Fatalf("expected refs size <= 500, got %d", len(refs))
	}
}
