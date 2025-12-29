package api

import (
	"bytes"
	"context"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/task"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// 应用商城配置
// getAppCacheDir 获取应用缓存目录
func getAppCacheDir() string {
	return filepath.Join(settings.GetDataDir(), "apps")
}

// 获取应用商城服务器地址
func getAppStoreServerURL() string {
	s, err := settings.GetSettings()
	if err != nil || s.AppStoreServerUrl == "" {
		return settings.DefaultAppStoreServerURL
	}
	return s.AppStoreServerUrl
}

// 应用结构
type App struct {
	ID          uint       `json:"id"` // ID 从 string 改为 uint，以匹配 gorm.Model
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Description string     `json:"description"`
	Version     string     `json:"version"`
	Logo        string     `json:"logo"`
	Website     string     `json:"website"`
	Tutorial    string     `json:"tutorial"`
	Dotenv      string     `json:"dotenv"`
	Compose     string     `json:"compose"` // Compose 从 map 改为 string
	Screenshots []string   `json:"screenshots"`
	Schema      []Variable `json:"schema"`
	DeploymentCount int    `json:"deployment_count"`
}

type Variable struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Category    string `json:"category"` // "basic", "advanced"
	ServiceName string `json:"serviceName"`
	ParamType   string `json:"paramType"` // port, path, env, hardware, other
}

type Port struct {
	Container   int    `json:"container"`
	Host        int    `json:"host"`
	Description string `json:"description"`
}

type Volume struct {
	Container   string `json:"container"`
	Host        string `json:"host"`
	Description string `json:"description"`
}

type EnvVar struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// mapHostPortsToContainerIDs 根据容器列表构建宿主机端口到容器ID的映射（仅记录 TCP 映射端口）。
func mapHostPortsToContainerIDs(containers []types.Container) map[int]string {
	portToContainer := make(map[int]string)
	for _, ctr := range containers {
		for _, p := range ctr.Ports {
			if p.PublicPort == 0 {
				continue
			}
			if strings.ToLower(p.Type) != "tcp" {
				continue
			}
			portToContainer[int(p.PublicPort)] = ctr.ID
		}
	}
	return portToContainer
}

// 注册应用商城路由
func RegisterAppStoreRoutes(r *gin.Engine) {
	// 应用商城的基础信息获取不需要认证
	public := r.Group("/api/appstore")
	{
		public.GET("/apps", listApps)
		public.GET("/apps/:id", getApp)
	}

	// 部署和状态查询需要认证 (与 protected 组一致)
	// 注意：这里需要传入 AuthMiddleware，或者将这部分逻辑移到 main.go 的 protected 组中
	// 为了简单起见，我们在 main.go 中统一处理
}

// 注册需要认证的应用商城路由
func RegisterAppStoreProtectedRoutes(r *gin.RouterGroup) {
	group := r.Group("/appstore")
	{
		group.POST("/deploy/:id", deployApp)
		group.POST("/deploy_count/:id", submitDeployCount)
		group.GET("/status/:id", getAppStatus)
		// SSE 路由通常通过 URL Token 认证，或者放行
		// 如果在 protected 组中，前端 EventSource 需要带 Token (通常只能通过 URL Query)
		group.GET("/tasks/:id/events", taskEvents)
	}
}

func submitDeployCount(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的应用ID"})
		return
	}

	if err := submitAppStoreDeploymentCount(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交部署次数失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func submitAppStoreDeploymentCount(appID string) error {
	base := strings.TrimRight(getAppStoreServerURL(), "/")
	url := fmt.Sprintf("%s/api/templates/%s/deploy", base, appID)
	if settings.IsDebugEnabled() {
		log.Printf("[Debug] submitAppStoreDeploymentCount: %s", settings.RedactAppStoreURL(url))
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("appstore server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	return nil
}

// 获取应用列表
func listApps(c *gin.Context) {
	if err := os.MkdirAll(getAppCacheDir(), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建缓存目录失败"})
		return
	}

	// 从应用商城服务器获取应用列表
	url := fmt.Sprintf("%s/api/templates", getAppStoreServerURL())
	if settings.IsDebugEnabled() {
		log.Printf("Requesting AppStore URL: %s", settings.RedactAppStoreURL(url))
	}
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error connecting to AppStore: %s", settings.RedactAppStoreURL(err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接应用商城服务器失败: " + settings.RedactAppStoreURL(err.Error())})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取应用列表失败"})
		return
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取应用列表失败"})
		return
	}

	// 解析应用列表
	var apps []App
	if err := json.Unmarshal(body, &apps); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析应用列表失败"})
		return
	}

	// 缓存应用列表
	for _, app := range apps {
		appData, err := json.Marshal(app)
		if err != nil {
			continue
		}
		// 使用应用名称作为文件名
		appPath := filepath.Join(getAppCacheDir(), fmt.Sprintf("%s.json", app.Name))
		os.WriteFile(appPath, appData, 0644)
	}

	c.JSON(http.StatusOK, apps)
}

// 从缓存或服务器获取应用详情 (Helper)
func getAppFromCacheOrServer(idOrName string) (*App, error) {
	if settings.IsDebugEnabled() {
		log.Printf("[Debug] getAppFromCacheOrServer: idOrName=%s", idOrName)
	}

	// 1. 尝试从服务器获取 (优先，以获取最新信息和正确的 Name)
	url := fmt.Sprintf("%s/api/templates/%s", getAppStoreServerURL(), idOrName)
	if settings.IsDebugEnabled() {
		log.Printf("[Debug] Requesting Server: %s", settings.RedactAppStoreURL(url))
	}
	resp, err := http.Get(url)

	var app App

	if err == nil && resp.StatusCode == http.StatusOK {
		// 服务器获取成功
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(body, &app); err == nil {
			// 更新缓存 (使用 Name)
			appData, _ := json.Marshal(app)
			os.MkdirAll(getAppCacheDir(), 0755)
			appPath := filepath.Join(getAppCacheDir(), fmt.Sprintf("%s.json", app.Name))
			os.WriteFile(appPath, appData, 0644)
			return &app, nil
		}
	}

	// 2. 如果服务器失败，尝试从缓存查找
	// 由于 ID 和 Name 映射关系不明确，如果传入的是 ID，我们可能需要遍历缓存
	files, err := os.ReadDir(getAppCacheDir())
	if err == nil {
		for _, file := range files {
			// 如果传入的是 Name，直接匹配文件名
			if file.Name() == idOrName+".json" {
				content, _ := os.ReadFile(filepath.Join(getAppCacheDir(), file.Name()))
				json.Unmarshal(content, &app)
				return &app, nil
			}

			// 否则读取内容匹配 ID
			content, err := os.ReadFile(filepath.Join(getAppCacheDir(), file.Name()))
			if err == nil {
				var cachedApp App
				if err := json.Unmarshal(content, &cachedApp); err == nil {
					// 兼容 string 和 int 类型的 ID 比较
					if fmt.Sprintf("%v", cachedApp.ID) == idOrName {
						return &cachedApp, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("无法获取应用详情 (ID/Name: %s)", idOrName)
}

// 获取单个应用
func getApp(c *gin.Context) {
	id := c.Param("id")
	app, err := getAppFromCacheOrServer(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

type DeployRequest struct {
	Compose     string            `json:"compose"`
	Env         map[string]string `json:"env"`
	Dotenv      string            `json:"dotenv"`
	Config      []Variable        `json:"config"`
	ProjectName string            `json:"projectName"`
}

func normalizeProjectName(name string) string {
	lower := strings.ToLower(name)
	buf := make([]rune, 0, len(lower))
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			buf = append(buf, r)
		}
	}
	if len(buf) == 0 {
		return "project"
	}

	out := string(buf)
	out = strings.TrimLeftFunc(out, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	if out == "" {
		return "project"
	}
	return out
}

func removeExplicitContainerNames(composeContent string) (string, error) {
	var composeMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &composeMap); err != nil {
		return "", err
	}

	servicesRaw, ok := composeMap["services"]
	if !ok {
		return composeContent, nil
	}
	services, ok := servicesRaw.(map[string]interface{})
	if !ok {
		return composeContent, nil
	}

	changed := false
	for _, serviceRaw := range services {
		service, ok := serviceRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := service["container_name"]; ok {
			delete(service, "container_name")
			changed = true
		}
	}

	if !changed {
		return composeContent, nil
	}
	out, err := marshalComposeYAMLOrdered(composeMap)
	if err != nil {
		return "", err
	}
	return out, nil
}

type envFileRef struct {
	Path     string
	Required bool
}

// isLikelyEnvKey 判断是否是常见的环境变量 key（避免将异常 key 注入到 docker compose 进程环境）
func isLikelyEnvKey(key string) bool {
	if key == "" {
		return false
	}
	b0 := key[0]
	if !((b0 >= 'A' && b0 <= 'Z') || (b0 >= 'a' && b0 <= 'z') || b0 == '_') {
		return false
	}
	for i := 1; i < len(key); i++ {
		b := key[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' {
			continue
		}
		return false
	}
	return true
}

func isSelfEnvPlaceholder(key, val string) bool {
	k := strings.TrimSpace(key)
	v := strings.TrimSpace(val)
	if k == "" || v == "" {
		return false
	}
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			v = strings.TrimSpace(v[1 : len(v)-1])
		}
	}
	if v == "["+k+"]" {
		return true
	}
	prefix := "${" + k
	if !strings.HasPrefix(v, prefix) || !strings.HasSuffix(v, "}") {
		return false
	}
	if len(v) == len(prefix)+1 && v[len(prefix)] == '}' {
		return true
	}
	if len(v) > len(prefix)+1 {
		next := v[len(prefix)]
		if next == ':' || next == '-' || next == '?' || next == '+' {
			return true
		}
	}
	return false
}

func sanitizeDotenvText(dotenvText string, fallbackDotenv string) string {
	src := string(dotenvText)
	fallbackMap := parseDotenvToMap(fallbackDotenv)
	if len(fallbackMap) == 0 {
		return src
	}

	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		prefix := ""
		line := trimmed
		if strings.HasPrefix(line, "export ") {
			prefix = "export "
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		if isSelfEnvPlaceholder(key, val) {
			if fb, ok := fallbackMap[key]; ok {
				lines[i] = prefix + key + "=" + fb
			}
		}
	}
	return strings.Join(lines, "\n")
}

// parseDotenvToMap 将 dotenv 文本解析为 map，用于变量优先级合并与 Compose 插值
func parseDotenvToMap(content string) map[string]string {
	out := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, rawLine := range lines {
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
			out[key] = ""
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		out[key] = val
	}
	return out
}

// extractComposeInterpolationKeys 提取 Compose 文本中出现的 ${KEY...} 插值变量名集合
func extractComposeInterpolationKeys(composeContent string) map[string]struct{} {
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)`)
	out := make(map[string]struct{})
	for _, m := range re.FindAllStringSubmatch(composeContent, -1) {
		if len(m) < 2 {
			continue
		}
		key := strings.TrimSpace(m[1])
		if !isLikelyEnvKey(key) {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}

// filterDotenvByAllowedKeys 仅保留允许集合中的 KEY=VALUE 行，其它变量行会被剔除（注释/空行保留）
func filterDotenvByAllowedKeys(dotenvText string, allowed map[string]struct{}) string {
	if len(allowed) == 0 {
		return ""
	}

	lines := strings.Split(dotenvText, "\n")
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			out = append(out, raw)
			continue
		}

		line := trimmed
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.Index(line, "=")
		key := ""
		if idx < 0 {
			key = strings.TrimSpace(line)
		} else {
			key = strings.TrimSpace(line[:idx])
		}
		if key == "" {
			continue
		}
		if _, ok := allowed[key]; !ok {
			continue
		}
		out = append(out, raw)
	}
	return strings.Join(out, "\n")
}

// renderDotenvFromMap 将 map 渲染为 dotenv 文本（兼容旧版仅传 env map 的行为）
func renderDotenvFromMap(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range env {
		if strings.TrimSpace(k) == "" {
			continue
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}
	return b.String()
}

// collectComposeEnvironmentDefaults 从 Compose 的 environment 字段提取默认值，辅助变量插值（不修改容器环境）
func collectComposeEnvironmentDefaults(composeContent string) map[string]string {
	out := make(map[string]string)
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &data); err != nil {
		return out
	}
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return out
	}

	for _, serviceRaw := range services {
		svc, ok := serviceRaw.(map[string]interface{})
		if !ok {
			continue
		}
		envRaw, ok := svc["environment"]
		if !ok {
			continue
		}
		switch e := envRaw.(type) {
		case map[string]interface{}:
			for k, v := range e {
				key := strings.TrimSpace(fmt.Sprintf("%v", k))
				if key == "" {
					continue
				}
				out[key] = fmt.Sprintf("%v", v)
			}
		case []interface{}:
			for _, item := range e {
				s := strings.TrimSpace(fmt.Sprintf("%v", item))
				if s == "" {
					continue
				}
				parts := strings.SplitN(s, "=", 2)
				key := strings.TrimSpace(parts[0])
				if key == "" {
					continue
				}
				if len(parts) == 2 {
					out[key] = strings.TrimSpace(parts[1])
				} else {
					if _, exists := out[key]; !exists {
						out[key] = ""
					}
				}
			}
		}
	}
	return out
}

// extractEnvFileRefs 提取 services.*.env_file 的引用路径（支持 string / list / {path,required}）
func extractEnvFileRefs(composeContent string) ([]envFileRef, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &data); err != nil {
		return nil, err
	}
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	merged := make(map[string]bool)
	for _, serviceRaw := range services {
		svc, ok := serviceRaw.(map[string]interface{})
		if !ok {
			continue
		}
		envFileRaw, ok := svc["env_file"]
		if !ok {
			continue
		}
		addRef := func(p string, required bool) {
			p = strings.TrimSpace(p)
			if p == "" {
				return
			}
			if prev, ok := merged[p]; ok {
				merged[p] = prev || required
				return
			}
			merged[p] = required
		}

		switch v := envFileRaw.(type) {
		case string:
			addRef(v, true)
		case []interface{}:
			for _, item := range v {
				switch it := item.(type) {
				case string:
					addRef(it, true)
				case map[string]interface{}:
					pathVal, _ := it["path"]
					reqVal, hasReq := it["required"]
					pathStr := strings.TrimSpace(fmt.Sprintf("%v", pathVal))
					required := true
					if hasReq {
						if b, ok := reqVal.(bool); ok {
							required = b
						} else {
							s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", reqVal)))
							if s == "false" || s == "0" || s == "no" {
								required = false
							}
						}
					}
					addRef(pathStr, required)
				default:
					addRef(fmt.Sprintf("%v", it), true)
				}
			}
		default:
			addRef(fmt.Sprintf("%v", v), true)
		}
	}

	out := make([]envFileRef, 0, len(merged))
	for p, required := range merged {
		out = append(out, envFileRef{Path: p, Required: required})
	}
	return out, nil
}

func removeDotenvEnvFileRefs(composeContent string) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeContent), &doc); err != nil {
		return "", err
	}
	if len(doc.Content) == 0 {
		return composeContent, nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return composeContent, nil
	}

	findMapValue := func(m *yaml.Node, key string) *yaml.Node {
		if m == nil || m.Kind != yaml.MappingNode {
			return nil
		}
		for i := 0; i+1 < len(m.Content); i += 2 {
			k := m.Content[i]
			v := m.Content[i+1]
			if k != nil && k.Kind == yaml.ScalarNode && k.Value == key {
				return v
			}
		}
		return nil
	}

	deleteMapKey := func(m *yaml.Node, key string) {
		if m == nil || m.Kind != yaml.MappingNode {
			return
		}
		next := make([]*yaml.Node, 0, len(m.Content))
		for i := 0; i+1 < len(m.Content); i += 2 {
			k := m.Content[i]
			v := m.Content[i+1]
			if k != nil && k.Kind == yaml.ScalarNode && k.Value == key {
				continue
			}
			next = append(next, k, v)
		}
		m.Content = next
	}

	services := findMapValue(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return composeContent, nil
	}

	for i := 0; i+1 < len(services.Content); i += 2 {
		svcVal := services.Content[i+1]
		if svcVal == nil || svcVal.Kind != yaml.MappingNode {
			continue
		}

		envFile := findMapValue(svcVal, "env_file")
		if envFile == nil {
			continue
		}

		switch envFile.Kind {
		case yaml.ScalarNode:
			if strings.TrimSpace(envFile.Value) == ".env" {
				deleteMapKey(svcVal, "env_file")
			}
		case yaml.SequenceNode:
			nextItems := make([]*yaml.Node, 0, len(envFile.Content))
			for _, it := range envFile.Content {
				if it == nil {
					continue
				}
				if it.Kind == yaml.ScalarNode {
					if strings.TrimSpace(it.Value) == ".env" {
						continue
					}
					nextItems = append(nextItems, it)
					continue
				}
				if it.Kind == yaml.MappingNode {
					pathNode := findMapValue(it, "path")
					if pathNode != nil && pathNode.Kind == yaml.ScalarNode && strings.TrimSpace(pathNode.Value) == ".env" {
						continue
					}
					nextItems = append(nextItems, it)
					continue
				}
				nextItems = append(nextItems, it)
			}
			if len(nextItems) == 0 {
				deleteMapKey(svcVal, "env_file")
			} else {
				envFile.Content = nextItems
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()
		return "", err
	}
	_ = enc.Close()
	return buf.String(), nil
}

func removePlaceholderEnvVars(composeContent string, knownDotenvKeys map[string]struct{}, keepDotenvKeys map[string]struct{}) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeContent), &doc); err != nil {
		return "", err
	}
	if len(doc.Content) == 0 {
		return composeContent, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return composeContent, nil
	}

	findMapValue := func(m *yaml.Node, key string) *yaml.Node {
		if m == nil || m.Kind != yaml.MappingNode {
			return nil
		}
		for i := 0; i+1 < len(m.Content); i += 2 {
			k := m.Content[i]
			v := m.Content[i+1]
			if k != nil && k.Kind == yaml.ScalarNode && k.Value == key {
				return v
			}
		}
		return nil
	}
	deleteMapKey := func(m *yaml.Node, key string) {
		if m == nil || m.Kind != yaml.MappingNode {
			return
		}
		next := make([]*yaml.Node, 0, len(m.Content))
		for i := 0; i+1 < len(m.Content); i += 2 {
			k := m.Content[i]
			v := m.Content[i+1]
			if k != nil && k.Kind == yaml.ScalarNode && k.Value == key {
				continue
			}
			next = append(next, k, v)
		}
		m.Content = next
	}

	extractPlaceholders := func(val string) []string {
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = strings.TrimSpace(val[1 : len(val)-1])
			}
		}
		out := make([]string, 0, 2)
		reBracket := regexp.MustCompile(`\[\s*([A-Za-z_][A-Za-z0-9_]*)\s*\]`)
		for _, m := range reBracket.FindAllStringSubmatch(val, -1) {
			if len(m) >= 2 {
				out = append(out, strings.TrimSpace(m[1]))
			}
		}
		reInterp := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)`)
		for _, m := range reInterp.FindAllStringSubmatch(val, -1) {
			if len(m) >= 2 {
				out = append(out, strings.TrimSpace(m[1]))
			}
		}
		return out
	}
	shouldRemove := func(val string) bool {
		for _, ph := range extractPlaceholders(val) {
			if ph == "" {
				continue
			}
			if _, known := knownDotenvKeys[ph]; !known {
				continue
			}
			if _, keep := keepDotenvKeys[ph]; keep {
				continue
			}
			return true
		}
		return false
	}

	services := findMapValue(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return composeContent, nil
	}

	for i := 0; i+1 < len(services.Content); i += 2 {
		svcVal := services.Content[i+1]
		if svcVal == nil || svcVal.Kind != yaml.MappingNode {
			continue
		}

		envNode := findMapValue(svcVal, "environment")
		if envNode == nil {
			continue
		}

		switch envNode.Kind {
		case yaml.MappingNode:
			next := make([]*yaml.Node, 0, len(envNode.Content))
			for j := 0; j+1 < len(envNode.Content); j += 2 {
				kNode := envNode.Content[j]
				vNode := envNode.Content[j+1]
				if kNode == nil || vNode == nil || kNode.Kind != yaml.ScalarNode {
					next = append(next, kNode, vNode)
					continue
				}
				if vNode.Kind == yaml.ScalarNode && shouldRemove(vNode.Value) {
					continue
				}
				next = append(next, kNode, vNode)
			}
			if len(next) == 0 {
				deleteMapKey(svcVal, "environment")
			} else {
				envNode.Content = next
			}
		case yaml.SequenceNode:
			nextItems := make([]*yaml.Node, 0, len(envNode.Content))
			for _, it := range envNode.Content {
				if it == nil || it.Kind != yaml.ScalarNode {
					nextItems = append(nextItems, it)
					continue
				}
				s := strings.TrimSpace(it.Value)
				if s == "" || strings.HasPrefix(s, "#") {
					nextItems = append(nextItems, it)
					continue
				}
				parts := strings.SplitN(s, "=", 2)
				if len(parts) != 2 {
					nextItems = append(nextItems, it)
					continue
				}
				val := strings.TrimSpace(parts[1])
				if shouldRemove(val) {
					continue
				}
				nextItems = append(nextItems, it)
			}
			if len(nextItems) == 0 {
				deleteMapKey(svcVal, "environment")
			} else {
				envNode.Content = nextItems
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()
		return "", err
	}
	_ = enc.Close()
	return buf.String(), nil
}

// ensureEnvFiles 在部署目录中落盘保存 .env 及 Compose 引用的 env_file 文件
func ensureEnvFiles(composeDir string, composeContent string, dotenvText string, envFiles []envFileRef, t *task.Task) error {
	writeFile := func(relPath string, content string) error {
		clean := filepath.Clean(relPath)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return fmt.Errorf("env_file 路径不安全: %s", relPath)
		}
		full := filepath.Join(composeDir, clean)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return err
		}
		return os.WriteFile(full, []byte(content), 0644)
	}

	allowedKeys := extractComposeInterpolationKeys(composeContent)
	dotenvText = filterDotenvByAllowedKeys(dotenvText, allowedKeys)

	dotenvVars := parseDotenvToMap(dotenvText)
	hasDotenvVars := len(dotenvVars) > 0

	requiredDotenvRef := false
	for _, ref := range envFiles {
		p := strings.TrimSpace(ref.Path)
		if p == "" {
			continue
		}
		clean := filepath.Clean(p)
		if clean == ".env" || strings.HasSuffix(clean, string(filepath.Separator)+".env") {
			if ref.Required {
				requiredDotenvRef = true
				break
			}
		}
	}

	if hasDotenvVars || requiredDotenvRef {
		dotenvClean := filepath.Clean(".env")
		content := dotenvText
		if !hasDotenvVars {
			content = ""
			if requiredDotenvRef && t != nil {
				t.AddLog("warning", "Compose 引用了 required 的 .env，但过滤后无可写入变量，已创建空文件")
			}
		}
		if err := writeFile(dotenvClean, content); err != nil {
			return err
		}
	}

	for _, ref := range envFiles {
		p := strings.TrimSpace(ref.Path)
		if p == "" {
			continue
		}
		clean := filepath.Clean(p)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			if t != nil {
				t.AddLog("warning", fmt.Sprintf("跳过不安全的 env_file 路径: %s", p))
			}
			continue
		}

		if clean == ".env" || strings.HasSuffix(clean, string(filepath.Separator)+".env") {
			if hasDotenvVars {
				if err := writeFile(clean, dotenvText); err != nil {
					return err
				}
				continue
			}
			if ref.Required {
				if t != nil {
					t.AddLog("warning", fmt.Sprintf("env_file %s 为 required，但未提供 .env 内容，已创建空文件", p))
				}
				if err := writeFile(clean, ""); err != nil {
					return err
				}
			}
			continue
		}

		full := filepath.Join(composeDir, clean)
		if _, err := os.Stat(full); err == nil {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}

		if ref.Required {
			if t != nil {
				t.AddLog("warning", fmt.Sprintf("env_file %s 为 required，但未找到对应文件内容，已创建空文件占位", p))
			}
		} else {
			if t != nil {
				t.AddLog("info", fmt.Sprintf("env_file %s 未找到对应文件，已创建空文件占位（required=false）", p))
			}
		}
		if err := writeFile(clean, ""); err != nil {
			return err
		}
	}

	return nil
}

// 部署应用
func deployApp(c *gin.Context) {
	id := c.Param("id")
	if settings.IsDebugEnabled() {
		log.Printf("[Debug] deployApp called for id: %s", id)
	}

	var req DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 创建异步任务
	tm := task.GetManager()
	t := tm.CreateTask("deploy_app")
	t.AddLog("info", fmt.Sprintf("开始部署应用 (ID: %s)", id))

	// 异步执行部署逻辑
	go func(taskId string, appId string, deployReq DeployRequest) {
		t := tm.GetTask(taskId)
		t.UpdateStatus(task.StatusRunning)
		notifyName := strings.TrimSpace(appId)
		defer func() {
			summary := t.Summary()
			st := summary.Status
			if st != task.StatusSuccess && st != task.StatusFailed && st != task.StatusCompleted {
				return
			}
			msg := fmt.Sprintf("应用部署任务结束：%s", notifyName)
			typ := "info"
			if st == task.StatusSuccess || st == task.StatusCompleted {
				typ = "success"
				msg = fmt.Sprintf("应用部署成功：%s", notifyName)
			} else {
				typ = "error"
				errText := strings.TrimSpace(summary.Error)
				if errText == "" {
					errText = "未知错误"
				}
				msg = fmt.Sprintf("应用部署失败：%s（%s）", notifyName, errText)
			}
			_ = database.SaveNotification(&database.Notification{
				Type:    typ,
				Message: msg,
				Read:    false,
			})
		}()

		// 打印调试信息，确认接收到的参数
		if settings.IsDebugEnabled() {
			log.Printf("[Debug] Deploy Params for %s: ConfigLen=%d, EnvLen=%d", appId, len(deployReq.Config), len(deployReq.Env))
		}

		// 获取应用详情 (支持缓存回源)
		t.AddLog("info", "正在获取应用配置...")
		app, err := getAppFromCacheOrServer(appId)
		if err != nil {
			t.AddLog("error", fmt.Sprintf("获取应用详情失败: %v", err))
			t.Finish(task.StatusFailed, nil, err.Error())
			return
		}
		if strings.TrimSpace(app.Name) != "" {
			notifyName = strings.TrimSpace(app.Name)
		}

		projectName := app.Name
		if deployReq.ProjectName != "" {
			projectName = deployReq.ProjectName
		}
		projectName = normalizeProjectName(projectName)
		if _, ok := validateComposeProjectName(projectName); !ok {
			projectName = "project"
		}
		if isSelfProjectName(projectName) {
			errMsg := "容器化部署模式下，禁止部署到自身项目目录"
			t.AddLog("error", errMsg)
			t.Finish(task.StatusFailed, nil, errMsg)
			return
		}

		t.AddLog("info", fmt.Sprintf("准备部署目录: %s", projectName))
		baseDir := getProjectsBaseDir()
		composeDir := filepath.Join(baseDir, projectName)

		if _, statErr := os.Stat(composeDir); statErr == nil {
			errMsg := fmt.Sprintf("项目 '%s' 已存在，如需重新部署请先删除现有项目", projectName)
			t.AddLog("error", errMsg)
			t.Finish(task.StatusFailed, nil, errMsg)
			return
		} else if !os.IsNotExist(statErr) {
			t.AddLog("error", fmt.Sprintf("检查部署目录失败: %v", statErr))
			t.Finish(task.StatusFailed, nil, statErr.Error())
			return
		}

		if mkErr := os.MkdirAll(composeDir, 0755); mkErr != nil {
			t.AddLog("error", fmt.Sprintf("创建部署目录失败: %v", mkErr))
			t.Finish(task.StatusFailed, nil, mkErr.Error())
			return
		}

		// 保存compose文件
		composeFile := filepath.Join(composeDir, "docker-compose.yml")

		// 清理旧的 .env 文件，防止干扰 (尤其是从旧版本升级或之前的失败部署遗留的文件)
		envFile := filepath.Join(composeDir, ".env")
		_ = os.Remove(envFile)

		// 优先使用前端传递的 compose 内容，如果没有则使用模板默认的
		composeContent := app.Compose
		// 注意：如果前端传递了 Compose，说明可能包含了前端的处理逻辑
		// 但根据新的后端渲染模式，前端应该只传递 Env，由后端负责渲染
		// 这里保留 deployReq.Compose 的覆盖逻辑，以兼容旧行为或特殊场景，但通常应为空或与 app.Compose 一致
		if deployReq.Compose != "" {
			composeContent = deployReq.Compose
		}

		// 新逻辑：使用 Config 数组重构 YAML
		if len(deployReq.Config) > 0 {
			// 1. 更新本地缓存文件 (User Request: 复写缓存文件里的 json)
			// 将当前的配置保存回 app.Schema，以便下次加载时保留用户的修改
			app.Schema = deployReq.Config
			// 注意：这里我们假设 deployReq.Config 包含了完整的配置列表
			// 如果是部分更新，可能需要更复杂的合并逻辑

			if appData, err := json.MarshalIndent(app, "", "  "); err == nil {
				appPath := filepath.Join(getAppCacheDir(), fmt.Sprintf("%s.json", app.Name))
				if err := os.WriteFile(appPath, appData, 0644); err != nil {
					t.AddLog("warning", fmt.Sprintf("无法更新应用缓存配置: %v", err))
				} else {
					t.AddLog("info", "应用配置已更新到本地缓存")
				}
			}

			t.AddLog("info", "正在应用配置参数...")
			modified, err := applyConfigToYaml(composeContent, deployReq.Config)
			if err != nil {
				t.AddLog("error", fmt.Sprintf("应用配置失败: %v", err))
				t.Finish(task.StatusFailed, nil, err.Error())
				return
			}
			composeContent = modified
		} else if len(deployReq.Env) > 0 {
			// 兼容旧逻辑：仅传递 env map
			// 说明：不再将变量注入 Compose 的 environment 字段，避免将本地变量强行写入容器环境
			// Compose 的变量替换由 docker compose 在执行时根据 .env 与进程环境变量完成
		} else {
			t.AddLog("warning", "未接收到任何配置参数，将使用默认模板部署")
		}

		baseName := normalizeProjectName(app.Name)
		if deployReq.ProjectName != "" && projectName != baseName {
			if modified, err := removeExplicitContainerNames(composeContent); err == nil {
				composeContent = modified
			} else {
				t.AddLog("warning", fmt.Sprintf("移除 container_name 失败: %v", err))
			}
		}

		if hostRoot := settings.GetHostProjectRoot(); hostRoot != "" {
			hostProjectDir := filepath.Join(hostRoot, projectName)
			if normalized, nerr := normalizeComposeBindMountsForHost(composeContent, hostProjectDir); nerr == nil {
				composeContent = normalized
			} else {
				t.AddLog("error", fmt.Sprintf("处理相对路径失败: %v", nerr))
				t.Finish(task.StatusFailed, nil, nerr.Error())
				return
			}
		}

		dotenvText := deployReq.Dotenv
		if strings.TrimSpace(dotenvText) == "" {
			if len(deployReq.Config) == 0 && strings.TrimSpace(app.Dotenv) != "" {
				dotenvText = app.Dotenv
			}
		}
		if strings.TrimSpace(dotenvText) != "" && strings.TrimSpace(app.Dotenv) != "" {
			dotenvText = sanitizeDotenvText(dotenvText, app.Dotenv)
		}

		allowedKeys := extractComposeInterpolationKeys(composeContent)
		dotenvText = filterDotenvByAllowedKeys(dotenvText, allowedKeys)

		knownDotenvKeys := make(map[string]struct{})
		for k := range parseDotenvToMap(app.Dotenv) {
			knownDotenvKeys[k] = struct{}{}
		}
		for k := range parseDotenvToMap(deployReq.Dotenv) {
			knownDotenvKeys[k] = struct{}{}
		}
		keepDotenvKeys := make(map[string]struct{})
		for k := range parseDotenvToMap(dotenvText) {
			keepDotenvKeys[k] = struct{}{}
		}
		if len(knownDotenvKeys) > 0 {
			if modified, err := removePlaceholderEnvVars(composeContent, knownDotenvKeys, keepDotenvKeys); err == nil {
				composeContent = modified
			} else {
				t.AddLog("warning", fmt.Sprintf("清理未启用的环境变量占位符失败，将保持原样: %v", err))
			}
		}

		if strings.TrimSpace(dotenvText) == "" {
			if modified, err := removeDotenvEnvFileRefs(composeContent); err == nil {
				composeContent = modified
			} else {
				t.AddLog("warning", fmt.Sprintf("移除 env_file .env 引用失败，将保持原样: %v", err))
			}
		}

		if writeErr := os.WriteFile(composeFile, []byte(composeContent), 0644); writeErr != nil {
			t.AddLog("error", fmt.Sprintf("保存Compose文件失败: %v", writeErr))
			t.Finish(task.StatusFailed, nil, writeErr.Error())
			return
		}

		if len(deployReq.Config) > 0 {
			app.Dotenv = dotenvText
			if appData, err := json.MarshalIndent(app, "", "  "); err == nil {
				appPath := filepath.Join(getAppCacheDir(), fmt.Sprintf("%s.json", app.Name))
				if err := os.WriteFile(appPath, appData, 0644); err != nil {
					t.AddLog("warning", fmt.Sprintf("无法更新应用缓存 .env: %v", err))
				}
			}
		}

		envFileRefs, efErr := extractEnvFileRefs(composeContent)
		if efErr != nil {
			t.AddLog("warning", fmt.Sprintf("解析 env_file 失败，将仅保存 .env: %v", efErr))
		}
		if err := ensureEnvFiles(composeDir, composeContent, dotenvText, envFileRefs, t); err != nil {
			t.AddLog("error", fmt.Sprintf("保存 .env/env_file 文件失败: %v", err))
			t.Finish(task.StatusFailed, nil, err.Error())
			os.RemoveAll(composeDir)
			return
		}

		composeEnvDefaults := collectComposeEnvironmentDefaults(composeContent)
		interpolationEnv := make(map[string]string)
		for k, v := range composeEnvDefaults {
			interpolationEnv[k] = v
		}
		for k, v := range parseDotenvToMap(dotenvText) {
			interpolationEnv[k] = v
		}
		for k, v := range deployReq.Env {
			if isSelfEnvPlaceholder(k, v) {
				continue
			}
			interpolationEnv[k] = v
		}
		for _, item := range deployReq.Config {
			key := strings.TrimSpace(item.Name)
			if key == "" {
				continue
			}
			if strings.TrimSpace(item.Default) == "" {
				continue
			}
			if isSelfEnvPlaceholder(key, item.Default) {
				continue
			}
			interpolationEnv[key] = item.Default
		}

		// 使用 docker compose 命令行部署，以确保原生行为（包括相对路径处理）
		t.AddLog("info", "开始执行 Docker Compose 部署...")

		args := []string{"compose"}
		if _, err := os.Stat(envFile); err == nil {
			args = append(args, "--env-file", envFile)
		}
		args = append(args, "up", "-d")
		cmd := exec.Command("docker", args...)
		cmd.Dir = composeDir // 设置工作目录为项目目录
		// 优化输出为纯文本，便于流式展示进度
		cmd.Env = append(os.Environ(), "COMPOSE_PROGRESS=plain", "COMPOSE_NO_COLOR=1")
		for k, v := range interpolationEnv {
			k = strings.TrimSpace(k)
			if k == "" || strings.Contains(k, "=") || !isLikelyEnvKey(k) {
				continue
			}
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// 获取输出管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.AddLog("error", fmt.Sprintf("创建输出管道失败: %v", err))
			t.Finish(task.StatusFailed, nil, err.Error())
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.AddLog("error", fmt.Sprintf("创建错误管道失败: %v", err))
			t.Finish(task.StatusFailed, nil, err.Error())
			return
		}

		// 启动命令
		if err := cmd.Start(); err != nil {
			t.AddLog("error", fmt.Sprintf("启动部署命令失败: %v", err))
			t.Finish(task.StatusFailed, nil, err.Error())
			// 部署失败，清理项目目录
			os.RemoveAll(composeDir)
			return
		}

		// 实时读取日志（兼容 \n 和 \r 进度刷新，每2秒节流一次）
		streamPipe := func(r io.Reader) {
			buf := make([]byte, 4096)
			var acc []byte
			lastFlush := time.Now()
			flush := func(force bool) {
				if len(acc) == 0 {
					return
				}
				// 将累积内容拆分成多条消息
				chunks := strings.Split(strings.ReplaceAll(string(acc), "\r", "\n"), "\n")
				for _, c := range chunks {
					line := strings.TrimSpace(c)
					if line != "" {
						t.AddLog("info", line)
					}
				}
				acc = acc[:0]
				lastFlush = time.Now()
			}
			for {
				n, err := r.Read(buf)
				if n > 0 {
					acc = append(acc, buf[:n]...)
					// 遇到换行立即flush
					if bytes.Contains(buf[:n], []byte{'\n'}) || bytes.Contains(buf[:n], []byte{'\r'}) {
						flush(false)
					} else {
						// 节流：每2秒刷新一次
						if time.Since(lastFlush) > 2*time.Second {
							flush(false)
						}
					}
				}
				if err != nil {
					// EOF或错误时，强制flush一次
					flush(true)
					return
				}
			}
		}

		go streamPipe(stdout)
		go streamPipe(stderr)

		// 等待命令完成
		if err := cmd.Wait(); err != nil {
			t.AddLog("error", fmt.Sprintf("部署命令执行失败: %v", err))

			// 部署失败，清理资源和项目目录
			t.AddLog("info", "正在清理失败的部署资源...")
			cleanupCmd := exec.Command("docker", "compose", "down")
			cleanupCmd.Dir = composeDir
			output, downErr := cleanupCmd.CombinedOutput()
			if downErr != nil {
				t.AddLog("warning", fmt.Sprintf("清理资源失败: %v, output: %s", downErr, string(output)))
			}

			os.RemoveAll(composeDir)
			t.Finish(task.StatusFailed, nil, err.Error())
			return
		}

		t.AddLog("success", fmt.Sprintf("应用 %s 部署成功！", app.Name))

		// 部署成功后，记录使用的端口到数据库
		if len(deployReq.Config) > 0 {
			var usedPorts []int
			for _, item := range deployReq.Config {
				if item.ParamType == "port" && item.Name != "" {
					if p, err := strconv.Atoi(item.Name); err == nil {
						usedPorts = append(usedPorts, p)
					}
				}
			}
			if len(usedPorts) > 0 {
				t.AddLog("info", fmt.Sprintf("正在登记端口使用情况: %v", usedPorts))
				owners := map[string][]int{}

				if cli, err := docker.NewDockerClient(); err == nil {
					defer cli.Close()

					containers, cerr := cli.ContainerList(context.Background(), types.ContainerListOptions{
						All: true,
						Filters: filters.NewArgs(
							filters.Arg("label", "com.docker.compose.project="+projectName),
						),
					})
					if cerr == nil && len(containers) == 0 {
						containers, _ = cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
						var filtered []types.Container
						for _, ctr := range containers {
							if ctr.Labels["com.docker.compose.project"] == projectName {
								filtered = append(filtered, ctr)
								continue
							}
							if wd := strings.TrimSpace(ctr.Labels["com.docker.compose.project.working_dir"]); wd != "" {
								if filepath.Base(wd) == projectName {
									filtered = append(filtered, ctr)
								}
							}
						}
						containers = filtered
					}

					portToContainer := mapHostPortsToContainerIDs(containers)
					for _, p := range usedPorts {
						if cid := strings.TrimSpace(portToContainer[p]); cid != "" {
							owners[cid] = append(owners[cid], p)
						} else {
							owners[projectName] = append(owners[projectName], p)
						}
					}
				} else {
					for _, p := range usedPorts {
						owners[projectName] = append(owners[projectName], p)
					}
				}

				tx, err := database.GetDB().Begin()
				if err != nil {
					t.AddLog("warning", "无法开启数据库事务进行端口登记")
				} else {
					ok := true
					for owner, ports := range owners {
						if rerr := database.ReservePortsTx(tx, ports, owner, "TCP", "App"); rerr != nil {
							ok = false
							t.AddLog("warning", fmt.Sprintf("端口登记失败: %v", rerr))
							break
						}
					}
					if !ok {
						_ = tx.Rollback()
					} else if cerr := tx.Commit(); cerr != nil {
						t.AddLog("warning", fmt.Sprintf("端口登记提交失败: %v", cerr))
					} else {
						t.AddLog("info", "端口登记完成")
					}
				}
			}
		}

		t.Finish(task.StatusSuccess, gin.H{"app_id": app.ID}, "")
	}(t.ID, id, req)

	// 立即返回 TaskID
	c.JSON(http.StatusOK, gin.H{
		"message": "部署任务已提交",
		"taskId":  t.ID,
	})
}

// applyConfigToYaml 根据配置数组重构 YAML
func applyConfigToYaml(content string, config []Variable) (string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}

	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no services found or invalid format")
	}

	// Group config by service
	configByService := make(map[string][]Variable)
	for _, item := range config {
		if item.ServiceName != "" {
			configByService[item.ServiceName] = append(configByService[item.ServiceName], item)
		}
	}

	for serviceName, svcConfig := range configByService {
		service, ok := services[serviceName]
		if !ok {
			if settings.IsDebugEnabled() {
				log.Printf("[Debug] Service %s not found in YAML", serviceName)
			}
			continue // Service not found in YAML, skip
		}
		svcMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		if settings.IsDebugEnabled() {
			log.Printf("[Debug] Processing service: %s, config items: %d", serviceName, len(svcConfig))
		}

		// Reset lists to rebuild them from config
		var newPorts []string
		var newVolumes []string
		envTouched := false
		newEnv := make(map[string]string)
		hasExistingEnv := false

		if existingEnv, ok := svcMap["environment"].(map[string]interface{}); ok {
			hasExistingEnv = true
			for k, v := range existingEnv {
				newEnv[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
			}
		} else if existingEnvList, ok := svcMap["environment"].([]interface{}); ok {
			hasExistingEnv = true
			for _, item := range existingEnvList {
				s := strings.TrimSpace(fmt.Sprintf("%v", item))
				if s == "" {
					continue
				}
				parts := strings.SplitN(s, "=", 2)
				key := strings.TrimSpace(parts[0])
				if key == "" {
					continue
				}
				val := ""
				if len(parts) == 2 {
					val = strings.TrimSpace(parts[1])
				}
				if _, exists := newEnv[key]; !exists {
					newEnv[key] = val
				}
			}
		}

		for _, item := range svcConfig {
			// Name is Host/Left, Default is Container/Right
			// Note: Front-end sends the *modified* value in 'Default' field (based on Variable struct definition)
			left := item.Name
			right := item.Default

			switch item.ParamType {
			case "port":
				// Format: "host:container" -> "name:default"
				if left != "" && right != "" {
					portStr := fmt.Sprintf("%s:%s", left, right)
					newPorts = append(newPorts, portStr)
					if settings.IsDebugEnabled() {
						log.Printf("[Debug] Adding port: %s", portStr)
					}
				}
			case "path", "volume": // Handle both 'path' and 'volume' types
				// Format: "host:container" -> "name:default"
				if left != "" && right != "" {
					newVolumes = append(newVolumes, fmt.Sprintf("%s:%s", left, right))
				}
			case "env", "environment":
				// Format: "key=value" -> "name=default"
				if left != "" {
					if isSelfEnvPlaceholder(left, right) {
						if _, exists := newEnv[left]; exists {
							delete(newEnv, left)
							envTouched = true
						}
						continue
					}
					if strings.TrimSpace(right) == "" {
						if _, exists := newEnv[left]; exists {
							delete(newEnv, left)
							envTouched = true
						}
						continue
					}
					newEnv[left] = right
					envTouched = true
				}
			}
		}

		// Apply updates
		if len(newPorts) > 0 {
			svcMap["ports"] = newPorts
		}
		if len(newVolumes) > 0 {
			svcMap["volumes"] = newVolumes
		}
		if envTouched || (!hasExistingEnv && len(newEnv) > 0) {
			svcMap["environment"] = newEnv
		}
	}

	out, err := marshalComposeYAMLOrdered(data)
	if err != nil {
		return "", err
	}
	return out, nil
}

// SSE 任务日志流
func taskEvents(c *gin.Context) {
	taskId := c.Param("id")
	tm := task.GetManager()
	t := tm.GetTask(taskId)

	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	setSSEHeaders(c)

	logChan, closeChan := t.Subscribe()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	// 获取已有的日志
	logs := t.GetLogs()

	// 获取 Header 中的 Last-Event-ID，如果存在，则只发送之后的日志
	lastEventIdRaw := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	lastEventId := int64(0)
	if lastEventIdRaw != "" {
		if v, err := strconv.ParseInt(lastEventIdRaw, 10, 64); err == nil && v > 0 {
			lastEventId = v
		}
	}

	startIndex := 0
	if lastEventId > 0 {
		if lastEventId >= int64(len(logs)) {
			startIndex = len(logs)
		} else {
			startIndex = int(lastEventId)
		}
	}

	nextID := int64(len(logs)) + 1
	if lastEventId > 0 && (lastEventId+1) > nextID {
		nextID = lastEventId + 1
	}

	// 发送历史日志
	for i := startIndex; i < len(logs); i++ {
		data, _ := json.Marshal(logs[i])
		fmt.Fprintf(c.Writer, "id: %d\ndata: %s\n\n", int64(i)+1, string(data))
	}
	c.Writer.Flush()

	// 监听新日志
	for {
		select {
		case log, ok := <-logChan:
			if !ok {
				return
			}
			data, _ := json.Marshal(log)
			fmt.Fprintf(c.Writer, "id: %d\ndata: %s\n\n", nextID, string(data))
			nextID++
			c.Writer.Flush()
		case <-keepalive.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		case <-closeChan:
			// 任务结束，发送最终状态
			resultData, _ := json.Marshal(gin.H{
				"type":    "result",
				"status":  t.Status,
				"message": "任务结束",
			})
			fmt.Fprintf(c.Writer, "id: %d\ndata: %s\n\n", nextID, string(resultData))
			nextID++
			c.Writer.Flush()
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

// 获取应用状态
func getAppStatus(c *gin.Context) {
	id := c.Param("id")

	// 获取应用详情以得到正确的项目名称
	app, err := getAppFromCacheOrServer(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取应用信息失败: " + err.Error()})
		return
	}

	// 创建Docker客户端
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败"})
		return
	}
	defer cli.Close()

	// 查询与应用相关的容器
	// 使用 app.Name 作为项目名称查询
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: "com.docker.compose.project=" + app.Name,
		}),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取容器列表失败"})
		return
	}

	// 统计容器状态
	total := len(containers)
	running := 0
	for _, container := range containers {
		if container.State == "running" {
			running++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       id,
		"name":     app.Name,
		"total":    total,
		"running":  running,
		"deployed": total > 0,
		"healthy":  total > 0 && running == total,
	})
}

// injectEnvToYaml 将环境变量注入到 Compose 文件的所有服务中
func injectEnvToYaml(content string, env map[string]string) (string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}

	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no services found or invalid format")
	}

	for _, service := range services {
		svcMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle environment
		envData, hasEnv := svcMap["environment"]
		if !hasEnv {
			// Create new map
			newEnv := make(map[string]string)
			for k, v := range env {
				newEnv[k] = v
			}
			svcMap["environment"] = newEnv
		} else {
			// Merge
			switch e := envData.(type) {
			case map[string]interface{}:
				for k, v := range env {
					e[k] = v
				}
			case []interface{}:
				// Append "KEY=VAL" strings
				for k, v := range env {
					// Check if already exists? Hard to check in list.
					// Just append.
					e = append(e, fmt.Sprintf("%s=%s", k, v))
				}
				svcMap["environment"] = e
			}
		}
	}

	out, err := marshalComposeYAMLOrdered(data)
	if err != nil {
		return "", err
	}
	return out, nil
}
