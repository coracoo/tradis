package handlers

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func errorCodeFromStatus(status int) string {
	switch status {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		return "OK"
	case http.StatusBadRequest:
		return "INVALID_PARAM"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusTooManyRequests:
		return "RATE_LIMIT"
	case http.StatusNotFound:
		return "NOT_FOUND"
	default:
		return "INTERNAL"
	}
}

func respondError(c *gin.Context, status int, message string, err error) {
	code := errorCodeFromStatus(status)
	details := ""
	if err != nil {
		details = err.Error()
	}
	errorText := message
	if details != "" {
		errorText = fmt.Sprintf("%s: %s", message, details)
	}
	payload := gin.H{
		"code":    code,
		"message": message,
		"error":   errorText,
	}
	if details != "" {
		payload["details"] = details
	}
	c.JSON(status, payload)
}

func RespondError(c *gin.Context, status int, message string, err error) {
	respondError(c, status, message, err)
}

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
					respondError(c, http.StatusInternalServerError, "初始化服务器版本失败", cerr)
					return
				}
				c.JSON(200, gin.H{
					"server_version": kv.Value,
					"updated_at":     kv.UpdatedAt,
				})
				return
			}
			respondError(c, http.StatusInternalServerError, "读取服务器版本失败", err)
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
			respondError(c, http.StatusBadRequest, "参数错误", err)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			respondError(c, http.StatusBadRequest, "name 不能为空", nil)
			return
		}
		if len(name) > 128 {
			respondError(c, http.StatusBadRequest, "name 过长", nil)
			return
		}

		website := strings.TrimSpace(req.Website)
		if len(website) > 512 {
			respondError(c, http.StatusBadRequest, "website 过长", nil)
			return
		}
		if website != "" && !(strings.HasPrefix(website, "http://") || strings.HasPrefix(website, "https://")) {
			respondError(c, http.StatusBadRequest, "website 必须以 http:// 或 https:// 开头", nil)
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
			respondError(c, http.StatusInternalServerError, "保存失败", err)
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
			respondError(c, http.StatusBadRequest, "参数错误", err)
			return
		}
		v := strings.TrimSpace(req.ServerVersion)
		if v == "" {
			respondError(c, http.StatusBadRequest, "server_version 不能为空", nil)
			return
		}
		if len(v) > 64 {
			respondError(c, http.StatusBadRequest, "server_version 过长", nil)
			return
		}

		now := time.Now()
		kv := ServerKV{Key: serverVersionKey, Value: v, UpdatedAt: now, CreatedAt: now}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"value": v, "updated_at": now}),
		}).Create(&kv).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "更新服务器版本失败", err)
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
			respondError(c, http.StatusBadRequest, "无效的模板ID", nil)
			return
		}

		res := db.Model(&Template{}).
			Where("id = ?", id).
			UpdateColumn("deployment_count", gorm.Expr("deployment_count + ?", 1))
		if res.Error != nil {
			respondError(c, http.StatusInternalServerError, "更新部署次数失败", res.Error)
			return
		}
		if res.RowsAffected == 0 {
			respondError(c, http.StatusNotFound, "模板不存在", nil)
			return
		}

		var tpl Template
		if err := db.First(&tpl, id).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "读取部署次数失败", err)
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
			respondError(c, http.StatusInternalServerError, "获取模板列表失败", err)
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
			respondError(c, http.StatusNotFound, "模板不存在", result.Error)
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

func GetTemplateVars(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var template Template
		idOrName := c.Param("id")

		var result *gorm.DB
		if isNumeric(idOrName) {
			result = db.First(&template, idOrName)
		} else {
			result = db.Where("name = ?", idOrName).First(&template)
		}

		if result.Error != nil {
			respondError(c, http.StatusNotFound, "模板不存在", result.Error)
			return
		}

		normalizeTemplateDotenvBySchema(&template)
		dotenvJSON, dotenvWarns, dotenvErrs := parseDotenvDetailed(template.Dotenv)

		parsedSchema, parseWarnings, parseErrors, refs := parseComposeToSchemaAndRefs(template.Compose)

		merged := make(Variables, 0, len(template.Schema)+len(parsedSchema))
		warnings := make([]string, 0)
		index := make(map[string]struct{})
		for _, it := range template.Schema {
			_ = addUniqueSchemaItem(&merged, index, it, &warnings)
		}
		for _, it := range parsedSchema {
			_ = addUniqueSchemaItem(&merged, index, it, &warnings)
		}
		warnings = append(warnings, parseWarnings...)

		sort.SliceStable(merged, func(i, j int) bool {
			a := merged[i]
			b := merged[j]
			if a.ServiceName != b.ServiceName {
				return a.ServiceName < b.ServiceName
			}
			if a.ParamType != b.ParamType {
				return a.ParamType < b.ParamType
			}
			return a.Name < b.Name
		})

		c.JSON(http.StatusOK, gin.H{
			"template": gin.H{
				"id":   template.ID,
				"name": template.Name,
			},
			"schema":          merged,
			"refs":            refs,
			"warnings":        warnings,
			"errors":          parseErrors,
			"dotenv":          template.Dotenv,
			"dotenv_json":     dotenvJSON,
			"dotenv_warnings": dotenvWarns,
			"dotenv_errors":   dotenvErrs,
		})
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

type composeVarRef struct {
	Name       string `json:"name"`
	HasDefault bool   `json:"hasDefault"`
	Default    string `json:"defaultValue"`
	Raw        string `json:"raw"`
}

func isPlainMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func stringifyValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int, int64, int32, float64, float32, uint, uint64, uint32:
		return fmt.Sprintf("%v", x)
	default:
		b, err := json.Marshal(x)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", x)
	}
}

func isSensitiveKey(k string) bool {
	s := strings.ToLower(strings.TrimSpace(k))
	return strings.Contains(s, "password") || strings.Contains(s, "passwd") || strings.Contains(s, "secret") || strings.Contains(s, "token") || strings.Contains(s, "key")
}

func makeSchemaItem(name string, serviceName string, paramType string, def interface{}) Variable {
	n := strings.TrimSpace(name)
	svc := strings.TrimSpace(serviceName)
	if svc == "" {
		svc = "Global"
	}
	pt := strings.TrimSpace(paramType)
	t := "string"
	if pt == "env" {
		if isSensitiveKey(n) {
			t = "password"
		} else {
			t = "string"
		}
	} else if pt == "port" {
		t = "port"
	} else if pt == "path" {
		t = "path"
	}
	return Variable{
		Name:        n,
		Label:       n,
		Description: "",
		Type:        t,
		Default:     stringifyValue(def),
		Category:    "basic",
		ServiceName: svc,
		ParamType:   pt,
	}
}

func addUniqueSchemaItem(schema *Variables, index map[string]struct{}, item Variable, warnings *[]string) bool {
	key := fmt.Sprintf("%s::%s::%s", item.ServiceName, item.ParamType, item.Name)
	if _, ok := index[key]; ok {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("发现重复配置项：%s/%s/%s", item.ServiceName, item.ParamType, item.Name))
		}
		return false
	}
	index[key] = struct{}{}
	*schema = append(*schema, item)
	return true
}

func parsePortString(raw string) (string, string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", false
	}
	noProto := s
	if idx := strings.Index(noProto, "/"); idx >= 0 {
		noProto = noProto[:idx]
	}
	rest := noProto
	if strings.HasPrefix(rest, "[") {
		if idx := strings.Index(rest, "]"); idx >= 0 && idx+1 < len(rest) && rest[idx+1] == ':' {
			rest = rest[idx+2:]
		}
	}
	parts := strings.Split(rest, ":")
	trimmed := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			trimmed = append(trimmed, t)
		}
	}
	if len(trimmed) < 2 {
		return "", "", false
	}
	containerPort := trimmed[len(trimmed)-1]
	hostPort := trimmed[len(trimmed)-2]
	if hostPort == "" || containerPort == "" {
		return "", "", false
	}
	return hostPort, containerPort, true
}

func parseVolumeString(raw string) (string, string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", false
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return "", "", false
	}
	host := strings.TrimSpace(parts[0])
	container := strings.TrimSpace(parts[1])
	if host == "" || container == "" {
		return "", "", false
	}
	return host, container, true
}

func extractEnvMap(envNode interface{}, warnings *[]string, serviceName string) map[string]string {
	envMap := make(map[string]string)
	duplicates := make(map[string]struct{})
	put := func(k string, v string, rawLine string) {
		key := strings.TrimSpace(k)
		if key == "" {
			return
		}
		if _, ok := envMap[key]; ok {
			duplicates[key] = struct{}{}
		}
		envMap[key] = v
		if v == "" && rawLine != "" && warnings != nil {
			svc := strings.TrimSpace(serviceName)
			if svc == "" {
				svc = "Global"
			}
			*warnings = append(*warnings, fmt.Sprintf("发现未赋值的环境变量：%s/%s", svc, key))
		}
	}

	switch e := envNode.(type) {
	case []interface{}:
		for _, it := range e {
			switch x := it.(type) {
			case string:
				idx := strings.Index(x, "=")
				if idx < 0 {
					put(x, "", x)
					continue
				}
				put(x[:idx], x[idx+1:], x)
			default:
				if m, ok := isPlainMap(it); ok {
					for kk, vv := range m {
						put(kk, stringifyValue(vv), kk+"=...")
					}
				}
			}
		}
	case map[string]interface{}:
		for kk, vv := range e {
			val := ""
			if vv != nil {
				val = stringifyValue(vv)
			}
			put(kk, val, kk+"=...")
		}
	}

	if warnings != nil {
		svc := strings.TrimSpace(serviceName)
		if svc == "" {
			svc = "Global"
		}
		for k := range duplicates {
			*warnings = append(*warnings, fmt.Sprintf("发现重复环境变量 key：%s/%s", svc, k))
		}
	}

	return envMap
}

func isLikelyEnvKey(key string) bool {
	k := strings.TrimSpace(key)
	if k == "" {
		return false
	}
	b0 := k[0]
	if !((b0 >= 'A' && b0 <= 'Z') || (b0 >= 'a' && b0 <= 'z') || b0 == '_') {
		return false
	}
	for i := 1; i < len(k); i++ {
		b := k[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' {
			continue
		}
		return false
	}
	return true
}

func extractVarRefs(text string) []composeVarRef {
	const maxLen = 2_000_000
	const maxRefs = 500
	s := text
	if len(s) > maxLen {
		s = s[:maxLen]
	}

	out := make([]composeVarRef, 0)
	seen := make(map[string]struct{})

	push := func(name string, hasDefault bool, def string, raw string) {
		n := strings.TrimSpace(name)
		if n == "" || !isLikelyEnvKey(n) {
			return
		}
		if len(out) >= maxRefs {
			return
		}
		key := fmt.Sprintf("%s::%t::%s", n, hasDefault, def)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, composeVarRef{Name: n, HasDefault: hasDefault, Default: def, Raw: raw})
	}

	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		next := s[i+1]
		if next == '$' {
			i++
			continue
		}
		if next == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				continue
			}
			end = i + 2 + end
			inner := s[i+2 : end]
			raw := s[i : end+1]

			namePart := inner
			def := ""
			hasDefault := false
			if idx := strings.Index(inner, ":-"); idx >= 0 {
				namePart = inner[:idx]
				def = inner[idx+2:]
				hasDefault = true
			} else if idx := strings.IndexByte(inner, '-'); idx >= 0 {
				namePart = inner[:idx]
				def = inner[idx+1:]
				hasDefault = true
			}
			push(namePart, hasDefault, def, raw)
			i = end
			continue
		}
		if (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') || next == '_' {
			j := i + 1
			for j < len(s) {
				b := s[j]
				if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' {
					j++
					continue
				}
				break
			}
			name := s[i+1 : j]
			push(name, false, "", "$"+name)
			i = j - 1
			continue
		}
	}

	return out
}

func parseComposeToSchemaAndRefs(compose string) (Variables, []string, []string, []composeVarRef) {
	content := strings.ReplaceAll(compose, "\r\n", "\n")
	if strings.TrimSpace(content) == "" {
		return Variables{}, []string{}, []string{}, []composeVarRef{}
	}
	if len(content) > 2_000_000 {
		return Variables{}, []string{}, []string{fmt.Sprintf("compose 内容过大：len=%d", len(content))}, []composeVarRef{}
	}

	schema := make(Variables, 0)
	warnings := make([]string, 0)
	errorsOut := make([]string, 0)
	index := make(map[string]struct{})

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		errorsOut = append(errorsOut, fmt.Sprintf("YAML 解析失败：%s", strings.TrimSpace(err.Error())))
	}

	if parsed != nil {
		if servicesAny, ok := parsed["services"]; ok {
			if services, ok := isPlainMap(servicesAny); ok {
				for svcName, svcAny := range services {
					svcMap, ok := isPlainMap(svcAny)
					if !ok {
						continue
					}
					serviceName := strings.TrimSpace(svcName)
					if serviceName == "" {
						serviceName = "Global"
					}

					if portsAny, ok := svcMap["ports"]; ok {
						if portsArr, ok := portsAny.([]interface{}); ok {
							for _, pAny := range portsArr {
								switch p := pAny.(type) {
								case string:
									host, target, ok := parsePortString(p)
									if !ok {
										warnings = append(warnings, fmt.Sprintf("发现无法解析的端口映射：%s/%s", serviceName, strings.TrimSpace(p)))
										continue
									}
									_ = addUniqueSchemaItem(&schema, index, makeSchemaItem(host, serviceName, "port", target), &warnings)
								default:
									if m, ok := isPlainMap(pAny); ok {
										host := strings.TrimSpace(stringifyValue(m["published"]))
										target := strings.TrimSpace(stringifyValue(m["target"]))
										if host == "" || target == "" {
											continue
										}
										_ = addUniqueSchemaItem(&schema, index, makeSchemaItem(host, serviceName, "port", target), &warnings)
									}
								}
							}
						}
					}

					if volsAny, ok := svcMap["volumes"]; ok {
						if volsArr, ok := volsAny.([]interface{}); ok {
							for _, vAny := range volsArr {
								switch v := vAny.(type) {
								case string:
									host, target, ok := parseVolumeString(v)
									if !ok {
										warnings = append(warnings, fmt.Sprintf("发现无法解析的挂载配置：%s/%s", serviceName, strings.TrimSpace(v)))
										continue
									}
									if !(strings.HasPrefix(host, "./") || strings.HasPrefix(host, "../") || strings.HasPrefix(host, "/")) {
										continue
									}
									_ = addUniqueSchemaItem(&schema, index, makeSchemaItem(host, serviceName, "path", target), &warnings)
								default:
									if m, ok := isPlainMap(vAny); ok {
										host := strings.TrimSpace(stringifyValue(m["source"]))
										target := strings.TrimSpace(stringifyValue(m["target"]))
										if host == "" || target == "" {
											continue
										}
										if !(strings.HasPrefix(host, "./") || strings.HasPrefix(host, "../") || strings.HasPrefix(host, "/")) {
											continue
										}
										_ = addUniqueSchemaItem(&schema, index, makeSchemaItem(host, serviceName, "path", target), &warnings)
									}
								}
							}
						}
					}

					if envAny, ok := svcMap["environment"]; ok {
						envMap := extractEnvMap(envAny, &warnings, serviceName)
						for k, v := range envMap {
							if strings.EqualFold(k, "PATH") {
								continue
							}
							_ = addUniqueSchemaItem(&schema, index, makeSchemaItem(k, serviceName, "env", v), &warnings)
						}
					}
				}
			}
		}
	}

	refs := extractVarRefs(content)
	for _, r := range refs {
		if strings.EqualFold(r.Name, "PATH") {
			continue
		}
		item := makeSchemaItem(r.Name, "Global", "env", "")
		if r.HasDefault {
			item.Default = r.Default
		}
		added := addUniqueSchemaItem(&schema, index, item, &warnings)
		if added && !r.HasDefault {
			warnings = append(warnings, fmt.Sprintf("发现未赋值的变量引用：%s", r.Raw))
		}
	}

	sort.SliceStable(schema, func(i, j int) bool {
		a := schema[i]
		b := schema[j]
		if a.ServiceName != b.ServiceName {
			return a.ServiceName < b.ServiceName
		}
		if a.ParamType != b.ParamType {
			return a.ParamType < b.ParamType
		}
		return a.Name < b.Name
	})

	return schema, warnings, errorsOut, refs
}

func ParseTemplateVars() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Compose string `json:"compose"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "无效的请求数据", err)
			return
		}
		schema, warnings, errorsOut, refs := parseComposeToSchemaAndRefs(req.Compose)

		c.JSON(http.StatusOK, gin.H{
			"schema":   schema,
			"warnings": warnings,
			"errors":   errorsOut,
			"refs":     refs,
		})
	}
}

func CreateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var template Template
		if err := c.ShouldBindJSON(&template); err != nil {
			respondError(c, http.StatusBadRequest, "无效的请求数据", err)
			return
		}

		// 兼容：如果前端只传了 dotenv_json，则在后端合成 dotenv 文本保存
		if strings.TrimSpace(template.Dotenv) == "" && len(template.DotenvJSON) > 0 {
			template.Dotenv = renderDotenvFromMap(template.DotenvJSON)
		}

		normalizeTemplateDotenvBySchema(&template)
		if err := db.Create(&template).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "创建模板失败", err)
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
			respondError(c, http.StatusNotFound, "模板不存在", err)
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
			respondError(c, http.StatusBadRequest, "无效的请求数据", err)
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
			respondError(c, http.StatusInternalServerError, "更新模板失败", err)
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
			respondError(c, http.StatusNotFound, "模板不存在", err)
			return
		}

		if err := db.Model(&Template{}).Where("id = ?", existingTemplate.ID).Update("enabled", true).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "启用模板失败", err)
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
			respondError(c, http.StatusNotFound, "模板不存在", err)
			return
		}

		if err := db.Model(&Template{}).Where("id = ?", existingTemplate.ID).Update("enabled", false).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "禁用模板失败", err)
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
			respondError(c, http.StatusInternalServerError, "删除模板失败", err)
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
