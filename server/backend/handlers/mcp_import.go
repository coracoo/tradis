package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MCPImportTemplate struct {
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Website     string            `json:"website"`
	Logo        string            `json:"logo"`
	Tutorial    string            `json:"tutorial"`
	Dotenv      string            `json:"dotenv"`
	DotenvJSON  map[string]string `json:"dotenv_json"`
	Compose     string            `json:"compose"`
	Screenshots StringArray       `json:"screenshots"`
	Schema      Variables         `json:"schema"`
	Enabled     *bool             `json:"enabled"`
}

type MCPImportTemplatesRequest struct {
	Mode      string              `json:"mode"`
	DryRun    bool                `json:"dryRun"`
	Templates []MCPImportTemplate `json:"templates"`
}

func ImportTemplatesMCP(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req MCPImportTemplatesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "无效的请求数据", err)
			return
		}

		mode := strings.ToLower(strings.TrimSpace(req.Mode))
		if mode == "" {
			mode = "upsert_by_name"
		}
		if mode != "upsert_by_name" && mode != "create_only" {
			respondError(c, http.StatusBadRequest, "mode 不支持", fmt.Errorf("mode=%s", mode))
			return
		}
		if len(req.Templates) == 0 {
			c.JSON(http.StatusOK, gin.H{"created": 0, "updated": 0, "skipped": 0, "errors": []any{}})
			return
		}
		if len(req.Templates) > 200 {
			respondError(c, http.StatusBadRequest, "templates 数量过多", fmt.Errorf("len=%d", len(req.Templates)))
			return
		}

		type itemError struct {
			Name  string `json:"name"`
			Error string `json:"error"`
		}

		created := 0
		updated := 0
		skipped := 0
		errs := make([]itemError, 0)
		changed := false

		tx := db.Begin()
		if tx.Error != nil {
			respondError(c, http.StatusInternalServerError, "创建事务失败", tx.Error)
			return
		}
		defer func() {
			if r := recover(); r != nil {
				_ = tx.Rollback()
				panic(r)
			}
		}()

		for _, in := range req.Templates {
			name := strings.TrimSpace(in.Name)
			if name == "" {
				errs = append(errs, itemError{Name: "", Error: "name 不能为空"})
				continue
			}
			if len(name) > 128 {
				errs = append(errs, itemError{Name: name, Error: "name 过长"})
				continue
			}

			tpl := Template{
				Name:        name,
				Category:    strings.TrimSpace(in.Category),
				Description: strings.TrimSpace(in.Description),
				Version:     strings.TrimSpace(in.Version),
				Website:     strings.TrimSpace(in.Website),
				Logo:        strings.TrimSpace(in.Logo),
				Tutorial:    strings.TrimSpace(in.Tutorial),
				Dotenv:      strings.ReplaceAll(in.Dotenv, "\r\n", "\n"),
				Compose:     strings.ReplaceAll(in.Compose, "\r\n", "\n"),
				Screenshots: in.Screenshots,
				Schema:      in.Schema,
			}
			if strings.TrimSpace(tpl.Dotenv) == "" && len(in.DotenvJSON) > 0 {
				tpl.Dotenv = renderDotenvFromMap(in.DotenvJSON)
			}

			if len(tpl.Schema) == 0 {
				parsedSchema, _, parseErrs, _ := parseComposeToSchemaAndRefs(tpl.Compose)
				if len(parseErrs) > 0 {
					errs = append(errs, itemError{Name: name, Error: strings.Join(parseErrs, "; ")})
					continue
				}
				tpl.Schema = parsedSchema
			}

			normalizeTemplateDotenvBySchema(&tpl)

			if req.DryRun {
				var existing Template
				q := tx.Where("name = ?", name).First(&existing)
				if q.Error == nil {
					if mode == "create_only" {
						skipped++
					} else {
						updated++
					}
				} else if errors.Is(q.Error, gorm.ErrRecordNotFound) {
					created++
				} else {
					errs = append(errs, itemError{Name: name, Error: q.Error.Error()})
				}
				continue
			}

			var existing Template
			q := tx.Where("name = ?", name).First(&existing)
			if q.Error == nil {
				if mode == "create_only" {
					skipped++
					continue
				}
				existing.Category = tpl.Category
				existing.Description = tpl.Description
				existing.Version = tpl.Version
				existing.Website = tpl.Website
				existing.Logo = tpl.Logo
				existing.Tutorial = tpl.Tutorial
				existing.Dotenv = tpl.Dotenv
				existing.Compose = tpl.Compose
				existing.Screenshots = tpl.Screenshots
				existing.Schema = tpl.Schema
				if in.Enabled != nil {
					existing.Enabled = *in.Enabled
				}
				normalizeTemplateDotenvBySchema(&existing)
				if err := tx.Save(&existing).Error; err != nil {
					errs = append(errs, itemError{Name: name, Error: err.Error()})
					continue
				}
				updated++
				changed = true
				continue
			}
			if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
				errs = append(errs, itemError{Name: name, Error: q.Error.Error()})
				continue
			}

			if in.Enabled != nil {
				tpl.Enabled = *in.Enabled
			}
			if err := tx.Create(&tpl).Error; err != nil {
				errs = append(errs, itemError{Name: name, Error: err.Error()})
				continue
			}
			created++
			changed = true
		}

		if len(errs) > 0 {
			_ = tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"created": created,
				"updated": updated,
				"skipped": skipped,
				"errors":  errs,
			})
			return
		}

		if !req.DryRun {
			if err := tx.Commit().Error; err != nil {
				respondError(c, http.StatusInternalServerError, "提交事务失败", err)
				return
			}
		} else {
			_ = tx.Rollback()
		}

		if changed {
			go func() {
				if err := SyncTemplatesToGitSync(db); err != nil {
					_ = err
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"created": created,
			"updated": updated,
			"skipped": skipped,
			"errors":  errs,
		})
	}
}
