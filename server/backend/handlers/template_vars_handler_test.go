package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetTemplateVarsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Template{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	compose := `
services:
  web:
    command: ["sh", "-c", "echo $A && echo ${B:-2} && echo $$HOME"]
    environment:
      APP_ENV: prod
`
	tpl := Template{
		Name:    "t1",
		Compose: strings.TrimSpace(compose),
		Dotenv:  "A=1\n",
		Schema: Variables{
			{Name: "APP_ENV", Label: "APP_ENV", Type: "string", Default: "prod", Category: "basic", ServiceName: "web", ParamType: "env"},
		},
	}
	if err := db.Create(&tpl).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	r := gin.New()
	r.GET("/templates/:id/vars", GetTemplateVars(db))

	req := httptest.NewRequest(http.MethodGet, "/templates/1/vars", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Schema []Variable      `json:"schema"`
		Refs   []composeVarRef `json:"refs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if len(resp.Schema) == 0 {
		t.Fatalf("expected schema not empty")
	}

	seenRef := map[string]composeVarRef{}
	for _, r := range resp.Refs {
		seenRef[r.Name] = r
	}
	if _, ok := seenRef["A"]; !ok {
		t.Fatalf("expected A in refs")
	}
	if r, ok := seenRef["B"]; !ok || !r.HasDefault || r.Default != "2" {
		t.Fatalf("expected B default 2, got %+v", r)
	}
	if _, ok := seenRef["HOME"]; ok {
		t.Fatalf("expected HOME not extracted from $$HOME")
	}
}
