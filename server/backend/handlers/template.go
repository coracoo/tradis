package handlers

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Variable struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Category    string `json:"category"`
	ServiceName string `json:"serviceName"`
	ParamType   string `json:"paramType"` // port, path, env, hardware, other
}

type Variables []Variable

// Value 实现 driver.Valuer 接口
func (v Variables) Value() (driver.Value, error) {
	if len(v) == 0 {
		return "[]", nil
	}
	return json.Marshal(v)
}

// Scan 实现 sql.Scanner 接口
func (v *Variables) Scan(value interface{}) error {
	if value == nil {
		*v = make(Variables, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		// 尝试处理字符串类型
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to unmarshal JSONB value")
		}
		bytes = []byte(str)
	}

	return json.Unmarshal(bytes, v)
}

type StringArray []string

// Value 实现 driver.Valuer 接口
func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner 接口
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = make(StringArray, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to unmarshal JSONB value")
		}
		bytes = []byte(str)
	}

	return json.Unmarshal(bytes, s)
}

type Template struct {
	ID              uint              `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	DeletedAt       gorm.DeletedAt    `gorm:"index" json:"deleted_at"`
	Name            string            `json:"name"`
	Category        string            `json:"category"`
	Description     string            `json:"description"`
	Version         string            `json:"version"`
	Website         string            `json:"website"`
	Logo            string            `json:"logo"`
	Tutorial        string            `json:"tutorial"`
	Dotenv          string            `json:"dotenv" gorm:"type:text"`
	DotenvJSON      map[string]string `json:"dotenv_json,omitempty" gorm:"-"`
	DotenvWarns     []string          `json:"dotenv_warnings,omitempty" gorm:"-"`
	DotenvErrs      []string          `json:"dotenv_errors,omitempty" gorm:"-"`
	Compose         string            `json:"compose"`
	Screenshots     StringArray       `json:"screenshots" gorm:"type:text"`
	Schema          Variables         `json:"schema" gorm:"type:text"`
	Enabled         bool              `json:"enabled" gorm:"default:true"`
	DeploymentCount uint              `json:"deployment_count" gorm:"default:0"`
}

type ServerKV struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ApplicationRequest struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name" gorm:"size:128"`
	Website   string    `json:"website" gorm:"size:512"`
	ClientIP  string    `json:"client_ip" gorm:"size:64"`
	UserAgent string    `json:"user_agent" gorm:"size:512"`
}

const serverVersionKey = "server_version"

func GetServerVersion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var kv ServerKV
		err := db.First(&kv, "key = ?", serverVersionKey).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				v := strings.TrimSpace(os.Getenv("SERVER_VERSION"))
				if v == "" {
					v = "0.0.0"
				}
				kv = ServerKV{Key: serverVersionKey, Value: v}
				if cerr := db.Create(&kv).Error; cerr != nil {
					c.JSON(500, gin.H{"error": "初始化服务器版本失败"})
					return
				}
				c.JSON(200, gin.H{
					"server_version": kv.Value,
					"updated_at":     kv.UpdatedAt,
				})
				return
			}
			c.JSON(500, gin.H{"error": "读取服务器版本失败"})
			return
		}
		c.JSON(200, gin.H{
			"server_version": kv.Value,
			"updated_at":     kv.UpdatedAt,
		})
	}
}

// CreateApplicationRequest 创建“申请应用”记录（免认证），用于收集用户提交的应用信息
func CreateApplicationRequest(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 应用申请：允许免认证提交，但会记录来源 IP / UA，便于后续整理与溯源
		var req struct {
			Name    string `json:"name"`
			Website string `json:"website"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			c.JSON(400, gin.H{"error": "name 不能为空"})
			return
		}
		if len(name) > 128 {
			c.JSON(400, gin.H{"error": "name 过长"})
			return
		}

		website := strings.TrimSpace(req.Website)
		if len(website) > 512 {
			c.JSON(400, gin.H{"error": "website 过长"})
			return
		}
		if website != "" && !(strings.HasPrefix(website, "http://") || strings.HasPrefix(website, "https://")) {
			c.JSON(400, gin.H{"error": "website 必须以 http:// 或 https:// 开头"})
			return
		}

		ip := strings.TrimSpace(c.ClientIP())
		ua := strings.TrimSpace(c.Request.UserAgent())
		if len(ua) > 512 {
			ua = ua[:512]
		}

		item := ApplicationRequest{
			Name:      name,
			Website:   website,
			ClientIP:  ip,
			UserAgent: ua,
			CreatedAt: time.Now(),
		}
		if err := db.Create(&item).Error; err != nil {
			c.JSON(500, gin.H{"error": "保存失败"})
			return
		}

		c.JSON(200, gin.H{"id": item.ID})
	}
}

func UpdateServerVersion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ServerVersion string `json:"server_version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}
		v := strings.TrimSpace(req.ServerVersion)
		if v == "" {
			c.JSON(400, gin.H{"error": "server_version 不能为空"})
			return
		}
		if len(v) > 64 {
			c.JSON(400, gin.H{"error": "server_version 过长"})
			return
		}

		now := time.Now()
		kv := ServerKV{Key: serverVersionKey, Value: v, UpdatedAt: now, CreatedAt: now}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"value": v, "updated_at": now}),
		}).Create(&kv).Error; err != nil {
			c.JSON(500, gin.H{"error": "更新服务器版本失败"})
			return
		}
		c.JSON(200, gin.H{
			"server_version": kv.Value,
		})
	}
}

// parseDotenvDetailed 解析 dotenv 文本为 map，并输出可展示的告警/错误信息（用于前端提示）
func parseDotenvDetailed(content string) (map[string]string, []string, []string) {
	out := make(map[string]string)
	warnings := make([]string, 0)
	errorsList := make([]string, 0)
	seen := make(map[string]struct{})

	lines := strings.Split(content, "\n")
	for i, rawLine := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			key := strings.TrimSpace(line)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				warnings = append(warnings, fmt.Sprintf(".env 第%d行：重复 key %s", lineNo, key))
			}
			seen[key] = struct{}{}
			out[key] = ""
			warnings = append(warnings, fmt.Sprintf(".env 第%d行：未赋值 %s（已按空值处理）", lineNo, key))
			continue
		}
		if idx == 0 {
			warnings = append(warnings, fmt.Sprintf(".env 第%d行：无法解析（key 为空）: %s", lineNo, strings.TrimSpace(rawLine)))
			continue
		}

		key := strings.TrimSpace(line[:idx])
		valRaw := strings.TrimSpace(line[idx+1:])
		if key == "" {
			warnings = append(warnings, fmt.Sprintf(".env 第%d行：无法解析（key 为空）: %s", lineNo, strings.TrimSpace(rawLine)))
			continue
		}

		if _, ok := seen[key]; ok {
			warnings = append(warnings, fmt.Sprintf(".env 第%d行：重复 key %s（后者覆盖前者）", lineNo, key))
		}
		seen[key] = struct{}{}

		val := valRaw
		if len(valRaw) >= 2 {
			if (valRaw[0] == '"' && valRaw[len(valRaw)-1] == '"') || (valRaw[0] == '\'' && valRaw[len(valRaw)-1] == '\'') {
				val = valRaw[1 : len(valRaw)-1]
			} else if valRaw[0] == '"' || valRaw[0] == '\'' {
				// 引号不闭合
				warnings = append(warnings, fmt.Sprintf(".env 第%d行：引号未闭合（保留原值）: %s", lineNo, strings.TrimSpace(rawLine)))
				val = valRaw
			}
		}

		out[key] = val
	}

	_ = errorsList
	return out, warnings, errorsList
}

func IncrementTemplateDeploymentCount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" || !isNumeric(id) {
			c.JSON(400, gin.H{"error": "无效的模板ID"})
			return
		}

		res := db.Model(&Template{}).
			Where("id = ?", id).
			UpdateColumn("deployment_count", gorm.Expr("deployment_count + ?", 1))
		if res.Error != nil {
			c.JSON(500, gin.H{"error": "更新部署次数失败"})
			return
		}
		if res.RowsAffected == 0 {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}

		var tpl Template
		if err := db.First(&tpl, id).Error; err != nil {
			c.JSON(500, gin.H{"error": "读取部署次数失败"})
			return
		}

		c.JSON(200, gin.H{
			"id":               tpl.ID,
			"deployment_count": tpl.DeploymentCount,
		})
	}
}

// normalizeTemplateDotenvBySchema 将全局环境变量（Global/env）补齐到模板的 .env 文本中，避免全局变量来源混淆
func normalizeTemplateDotenvBySchema(t *Template) {
	if t == nil {
		return
	}

	existingMap, _, _ := parseDotenvDetailed(t.Dotenv)
	exists := make(map[string]struct{}, len(existingMap))
	for k := range existingMap {
		kk := strings.TrimSpace(k)
		if kk != "" {
			exists[kk] = struct{}{}
		}
	}

	linesToAppend := make([]string, 0)
	for _, v := range t.Schema {
		service := strings.TrimSpace(v.ServiceName)
		if service == "" {
			service = "Global"
		}
		if !strings.EqualFold(service, "Global") {
			continue
		}
		if strings.TrimSpace(v.ParamType) != "env" {
			continue
		}

		key := strings.TrimSpace(v.Name)
		if key == "" {
			continue
		}
		if _, ok := exists[key]; ok {
			continue
		}
		exists[key] = struct{}{}
		linesToAppend = append(linesToAppend, fmt.Sprintf("%s=%s", key, quoteDotenvValueIfNeeded(strings.TrimSpace(v.Default))))
	}

	if len(linesToAppend) == 0 {
		return
	}

	base := strings.ReplaceAll(t.Dotenv, "\r\n", "\n")
	base = strings.TrimRight(base, "\n")
	if base == "" {
		t.Dotenv = strings.Join(linesToAppend, "\n") + "\n"
		return
	}
	t.Dotenv = base + "\n" + strings.Join(linesToAppend, "\n") + "\n"
}

// quoteDotenvValueIfNeeded 将包含空格/特殊符号的值用双引号包裹，尽量保证 .env 可读性
func quoteDotenvValueIfNeeded(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t#\"'") {
		escaped := strings.ReplaceAll(v, "\"", "\\\"")
		return "\"" + escaped + "\""
	}
	return v
}

// renderDotenvFromMap 将 dotenv 的键值对渲染为 .env 文本（按 key 排序，保证输出稳定）
func renderDotenvFromMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		keys = append(keys, kk)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, quoteDotenvValueIfNeeded(strings.TrimSpace(m[k]))))
	}
	return strings.Join(lines, "\n") + "\n"
}

func ListTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var templates []Template
		query := db.Model(&Template{})
		enabledQ := strings.TrimSpace(c.Query("enabled"))
		if enabledQ != "" {
			switch strings.ToLower(enabledQ) {
			case "1", "true", "yes", "y", "on":
				query = query.Where("enabled = ?", true)
			case "0", "false", "no", "n", "off":
				query = query.Where("enabled = ?", false)
			}
		}

		if err := query.Find(&templates).Error; err != nil {
			c.JSON(500, gin.H{"error": "获取模板列表失败"})
			return
		}
		for i := range templates {
			normalizeTemplateDotenvBySchema(&templates[i])
			m, w, e := parseDotenvDetailed(templates[i].Dotenv)
			templates[i].DotenvJSON = m
			templates[i].DotenvWarns = w
			templates[i].DotenvErrs = e
		}
		c.JSON(200, templates)
	}
}

func GetTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var template Template
		idOrName := c.Param("id")

		var result *gorm.DB
		// 简单的数字检查 (或者使用 strconv.Atoi)
		if isNumeric(idOrName) {
			result = db.First(&template, idOrName)
		} else {
			result = db.Where("name = ?", idOrName).First(&template)
		}

		if result.Error != nil {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}
		normalizeTemplateDotenvBySchema(&template)
		m, w, e := parseDotenvDetailed(template.Dotenv)
		template.DotenvJSON = m
		template.DotenvWarns = w
		template.DotenvErrs = e
		c.JSON(200, template)
	}
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func CreateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var template Template
		if err := c.ShouldBindJSON(&template); err != nil {
			c.JSON(400, gin.H{"error": "无效的请求数据"})
			return
		}

		// 兼容：如果前端只传了 dotenv_json，则在后端合成 dotenv 文本保存
		if strings.TrimSpace(template.Dotenv) == "" && len(template.DotenvJSON) > 0 {
			template.Dotenv = renderDotenvFromMap(template.DotenvJSON)
		}

		normalizeTemplateDotenvBySchema(&template)
		if err := db.Create(&template).Error; err != nil {
			c.JSON(500, gin.H{"error": "创建模板失败"})
			return
		}
		m, w, e := parseDotenvDetailed(template.Dotenv)
		template.DotenvJSON = m
		template.DotenvWarns = w
		template.DotenvErrs = e
		go func() {
			if err := SyncTemplatesToGitSync(db); err != nil {
				log.Printf("[git_sync] 同步失败: %v", err)
			}
		}()
		c.JSON(201, template)
	}
}

func UpdateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var existingTemplate Template
		if err := db.First(&existingTemplate, c.Param("id")).Error; err != nil {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}

		type UpdateTemplateInput struct {
			Name        *string            `json:"name"`
			Category    *string            `json:"category"`
			Description *string            `json:"description"`
			Version     *string            `json:"version"`
			Website     *string            `json:"website"`
			Logo        *string            `json:"logo"`
			Tutorial    *string            `json:"tutorial"`
			Dotenv      *string            `json:"dotenv"`
			DotenvJSON  *map[string]string `json:"dotenv_json"`
			Compose     *string            `json:"compose"`
			Screenshots *StringArray       `json:"screenshots"`
			Schema      *Variables         `json:"schema"`
			Enabled     *bool              `json:"enabled"`
		}

		var input UpdateTemplateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(400, gin.H{"error": "无效的请求数据"})
			return
		}

		// 更新字段
		if input.Name != nil {
			existingTemplate.Name = *input.Name
		}
		if input.Category != nil {
			existingTemplate.Category = *input.Category
		}
		if input.Description != nil {
			existingTemplate.Description = *input.Description
		}
		if input.Version != nil {
			existingTemplate.Version = *input.Version
		}
		if input.Website != nil {
			existingTemplate.Website = *input.Website
		}
		if input.Logo != nil {
			existingTemplate.Logo = *input.Logo
		}
		if input.Tutorial != nil {
			existingTemplate.Tutorial = *input.Tutorial
		}
		if input.Dotenv != nil {
			existingTemplate.Dotenv = *input.Dotenv
		} else if input.DotenvJSON != nil && len(*input.DotenvJSON) > 0 {
			existingTemplate.Dotenv = renderDotenvFromMap(*input.DotenvJSON)
		}
		if input.Compose != nil {
			existingTemplate.Compose = *input.Compose
		}
		if input.Screenshots != nil {
			existingTemplate.Screenshots = *input.Screenshots
		}
		if input.Schema != nil {
			existingTemplate.Schema = *input.Schema
		}
		if input.Enabled != nil {
			existingTemplate.Enabled = *input.Enabled
		}

		normalizeTemplateDotenvBySchema(&existingTemplate)
		if err := db.Save(&existingTemplate).Error; err != nil {
			c.JSON(500, gin.H{"error": "更新模板失败"})
			return
		}
		m, w, e := parseDotenvDetailed(existingTemplate.Dotenv)
		existingTemplate.DotenvJSON = m
		existingTemplate.DotenvWarns = w
		existingTemplate.DotenvErrs = e
		go func() {
			if err := SyncTemplatesToGitSync(db); err != nil {
				log.Printf("[git_sync] 同步失败: %v", err)
			}
		}()
		c.JSON(200, existingTemplate)
	}
}

func EnableTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var existingTemplate Template
		if err := db.First(&existingTemplate, c.Param("id")).Error; err != nil {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}

		if err := db.Model(&Template{}).Where("id = ?", existingTemplate.ID).Update("enabled", true).Error; err != nil {
			c.JSON(500, gin.H{"error": "启用模板失败"})
			return
		}

		existingTemplate.Enabled = true
		m, w, e := parseDotenvDetailed(existingTemplate.Dotenv)
		existingTemplate.DotenvJSON = m
		existingTemplate.DotenvWarns = w
		existingTemplate.DotenvErrs = e

		go func() {
			if err := SyncTemplatesToGitSync(db); err != nil {
				log.Printf("[git_sync] 同步失败: %v", err)
			}
		}()

		c.JSON(200, existingTemplate)
	}
}

func DisableTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var existingTemplate Template
		if err := db.First(&existingTemplate, c.Param("id")).Error; err != nil {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}

		if err := db.Model(&Template{}).Where("id = ?", existingTemplate.ID).Update("enabled", false).Error; err != nil {
			c.JSON(500, gin.H{"error": "禁用模板失败"})
			return
		}

		existingTemplate.Enabled = false
		m, w, e := parseDotenvDetailed(existingTemplate.Dotenv)
		existingTemplate.DotenvJSON = m
		existingTemplate.DotenvWarns = w
		existingTemplate.DotenvErrs = e

		go func() {
			if err := SyncTemplatesToGitSync(db); err != nil {
				log.Printf("[git_sync] 同步失败: %v", err)
			}
		}()

		c.JSON(200, existingTemplate)
	}
}

func DeleteTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.Delete(&Template{}, c.Param("id")).Error; err != nil {
			c.JSON(500, gin.H{"error": "删除模板失败"})
			return
		}
		go func() {
			if err := SyncTemplatesToGitSync(db); err != nil {
				log.Printf("[git_sync] 同步失败: %v", err)
			}
		}()
		c.JSON(200, gin.H{"message": "删除成功"})
	}
}
