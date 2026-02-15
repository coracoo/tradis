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

func TestImportTemplatesMCPUpsertAndDryRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Template{}, &ServerKV{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.POST("/mcp/templates/import", ImportTemplatesMCP(db))

	compose := `services:
  web:
    image: nginx
    command: ["sh","-c","echo $A && echo ${B:-2} && echo $$HOME"]
`
	body1, _ := json.Marshal(MCPImportTemplatesRequest{
		Mode:   "upsert_by_name",
		DryRun: true,
		Templates: []MCPImportTemplate{
			{Name: "nginx", Compose: compose, Dotenv: "A=1\n"},
		},
	})
	req1 := httptest.NewRequest(http.MethodPost, "/mcp/templates/import", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	body2, _ := json.Marshal(MCPImportTemplatesRequest{
		Mode:   "upsert_by_name",
		DryRun: false,
		Templates: []MCPImportTemplate{
			{Name: "nginx", Compose: compose, Dotenv: "A=1\n"},
		},
	})
	req2 := httptest.NewRequest(http.MethodPost, "/mcp/templates/import", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var tpl Template
	if err := db.Where("name = ?", "nginx").First(&tpl).Error; err != nil {
		t.Fatalf("expected template created: %v", err)
	}
	if !strings.Contains(tpl.Compose, "services:") {
		t.Fatalf("unexpected compose saved")
	}

	body3, _ := json.Marshal(MCPImportTemplatesRequest{
		Mode:   "create_only",
		DryRun: false,
		Templates: []MCPImportTemplate{
			{Name: "nginx", Compose: compose, Dotenv: "A=2\n"},
		},
	})
	req3 := httptest.NewRequest(http.MethodPost, "/mcp/templates/import", bytes.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var resp3 struct {
		Skipped int `json:"skipped"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &resp3)
	if resp3.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", resp3.Skipped)
	}
}
