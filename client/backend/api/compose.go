package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/settings"
)

// getProjectsBaseDir 获取项目根目录
func getProjectsBaseDir() string {
	return settings.GetProjectRoot()
}

func removeDotenvEnvFileRefsFromCompose(composeContent string) (string, error) {
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

func dockerComposeCmd(projectDir string, args ...string) *exec.Cmd {
	base := []string{"compose"}
	envPath := filepath.Join(projectDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		base = append(base, "--env-file", envPath)
	}
	base = append(base, args...)
	cmd := exec.Command("docker", base...)
	cmd.Dir = projectDir
	return cmd
}

func normalizeComposeBindMountsForHost(composeContent string, hostProjectDir string) (string, error) {
	hostProjectDir = strings.TrimSpace(hostProjectDir)
	if hostProjectDir == "" {
		return composeContent, nil
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &root); err != nil {
		return "", err
	}

	servicesRaw, ok := root["services"]
	if !ok {
		return composeContent, nil
	}
	services, ok := servicesRaw.(map[string]interface{})
	if !ok {
		return composeContent, nil
	}

	rewriteSource := func(source string) (string, bool) {
		src := strings.TrimSpace(source)
		if src == "" {
			return source, false
		}
		if src == "." {
			return filepath.Clean(hostProjectDir), true
		}
		if strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../") {
			return filepath.Clean(filepath.Join(hostProjectDir, src)), true
		}
		return source, false
	}

	for _, svcRaw := range services {
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}
		volRaw, ok := svc["volumes"]
		if !ok {
			continue
		}
		vols, ok := volRaw.([]interface{})
		if !ok {
			continue
		}

		for i := range vols {
			switch v := vols[i].(type) {
			case string:
				parts := strings.Split(v, ":")
				if len(parts) < 2 {
					continue
				}
				newSrc, changed := rewriteSource(parts[0])
				if !changed {
					continue
				}
				parts[0] = newSrc
				vols[i] = strings.Join(parts, ":")
			case map[string]interface{}:
				srcVal, ok := v["source"]
				if !ok {
					continue
				}
				srcStr, ok := srcVal.(string)
				if !ok {
					continue
				}
				newSrc, changed := rewriteSource(srcStr)
				if !changed {
					continue
				}
				v["source"] = newSrc
				vols[i] = v
			}
		}

		svc["volumes"] = vols
	}

	out, err := marshalComposeYAMLOrdered(root)
	if err != nil {
		return "", err
	}
	return out, nil
}

// marshalComposeYAMLOrdered 将 Compose 数据结构序列化为 YAML，并统一字段顺序。
func marshalComposeYAMLOrdered(v any) (string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return "", err
	}
	if len(doc.Content) > 0 {
		reorderComposeRootNode(doc.Content[0])
	}

	if len(doc.Content) == 0 {
		return string(b), nil
	}

	out, err := yaml.Marshal(doc.Content[0])
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func reorderComposeRootNode(root *yaml.Node) {
	if root == nil {
		return
	}
	if root.Kind != yaml.MappingNode {
		return
	}

	reorderMappingNodeWithPreferredKeys(root, []string{"version", "name", "services", "networks", "volumes", "configs", "secrets"})

	services := mappingGetValue(root, "services")
	if services != nil {
		reorderComposeServicesNode(services)
	}
}

func reorderComposeServicesNode(services *yaml.Node) {
	if services == nil {
		return
	}
	if services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svcVal := services.Content[i+1]
		reorderComposeServiceNode(svcVal)
	}
}

func reorderComposeServiceNode(service *yaml.Node) {
	if service == nil {
		return
	}
	if service.Kind != yaml.MappingNode {
		return
	}

	firstKeys := map[string]bool{"image": true, "ports": true, "volumes": true, "env_file": true, "environment": true}
	lastKeys := map[string]bool{"healthcheck": true, "command": true, "cmd": true, "entrypoint": true}

	content := service.Content
	first := make([]*yaml.Node, 0, len(content))
	others := make([]*yaml.Node, 0, len(content))
	health := make([]*yaml.Node, 0, 2)
	cmd := make([]*yaml.Node, 0, 2)

	pushPair := func(dst *[]*yaml.Node, k *yaml.Node, v *yaml.Node) {
		*dst = append(*dst, k, v)
	}

	seen := map[string]bool{}
	getPair := func(name string) (*yaml.Node, *yaml.Node, bool) {
		for i := 0; i+1 < len(content); i += 2 {
			k := content[i]
			v := content[i+1]
			if k.Kind == yaml.ScalarNode && k.Value == name {
				return k, v, true
			}
		}
		return nil, nil, false
	}

	for _, kname := range []string{"image", "ports", "volumes", "env_file", "environment"} {
		k, v, ok := getPair(kname)
		if !ok {
			continue
		}
		pushPair(&first, k, v)
		seen[kname] = true
	}

	for i := 0; i+1 < len(content); i += 2 {
		k := content[i]
		v := content[i+1]
		name := ""
		if k.Kind == yaml.ScalarNode {
			name = k.Value
		}
		if name != "" {
			if seen[name] {
				continue
			}
			seen[name] = true
			if firstKeys[name] {
				continue
			}
			if lastKeys[name] {
				switch name {
				case "healthcheck":
					pushPair(&health, k, v)
				case "command", "cmd", "entrypoint":
					pushPair(&cmd, k, v)
				}
				continue
			}
		}
		pushPair(&others, k, v)
	}

	newContent := make([]*yaml.Node, 0, len(content))
	newContent = append(newContent, first...)
	newContent = append(newContent, others...)
	newContent = append(newContent, health...)
	newContent = append(newContent, cmd...)
	service.Content = newContent
}

func reorderMappingNodeWithPreferredKeys(node *yaml.Node, preferred []string) {
	if node == nil {
		return
	}
	if node.Kind != yaml.MappingNode {
		return
	}

	content := node.Content
	if len(content) < 2 {
		return
	}

	used := make([]bool, len(content))
	newContent := make([]*yaml.Node, 0, len(content))

	for _, key := range preferred {
		for i := 0; i+1 < len(content); i += 2 {
			k := content[i]
			if k.Kind == yaml.ScalarNode && k.Value == key {
				newContent = append(newContent, content[i], content[i+1])
				used[i] = true
				used[i+1] = true
				break
			}
		}
	}

	for i := 0; i+1 < len(content); i += 2 {
		if used[i] {
			continue
		}
		newContent = append(newContent, content[i], content[i+1])
	}

	node.Content = newContent
}

func mappingGetValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return v
		}
	}
	return nil
}

var selfIdentityOnce sync.Once
var selfContainerID string
var selfContainerName string
var selfComposeProject string
var selfComposeDirName string

const protectedComposeProjectName = "tradis"
const protectedImageRepo = "coracoo/tradis"

func isProtectedImage(image string) bool {
	v := strings.TrimSpace(image)
	if v == "" {
		return false
	}
	return v == protectedImageRepo || strings.HasPrefix(v, protectedImageRepo+":") || strings.HasPrefix(v, protectedImageRepo+"@")
}

func isProtectedLabels(labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	return strings.TrimSpace(labels["com.docker.compose.project"]) == protectedComposeProjectName
}

func isProtectedContainer(image string, labels map[string]string) bool {
	return isProtectedImage(image) || isProtectedLabels(labels)
}

func isSelfOrProtectedContainer(containerID string, containerName string, image string, labels map[string]string) bool {
	if containerID != "" && isSelfContainerID(containerID) {
		return true
	}
	if containerName != "" && isSelfContainerID(containerName) {
		return true
	}
	return isProtectedContainer(image, labels)
}

func getSelfIdentity() (containerID string, containerName string, composeProject string, composeDirName string) {
	selfIdentityOnce.Do(func() {
		id := detectSelfContainerID()
		if id == "" {
			return
		}

		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return
		}
		defer cli.Close()

		inspect, err := cli.ContainerInspect(context.Background(), id)
		if err != nil {
			if len(id) >= 8 {
				containers, listErr := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
				if listErr == nil {
					for _, ctr := range containers {
						if strings.HasPrefix(ctr.ID, id) {
							inspect, err = cli.ContainerInspect(context.Background(), ctr.ID)
							if err == nil {
								break
							}
						}
					}
				}
			}
		}
		if err != nil {
			return
		}

		selfContainerID = inspect.ID
		selfContainerName = strings.TrimPrefix(inspect.Name, "/")
		if inspect.Config != nil && inspect.Config.Labels != nil {
			selfComposeProject = strings.TrimSpace(inspect.Config.Labels["com.docker.compose.project"])
			workingDir := strings.TrimSpace(inspect.Config.Labels["com.docker.compose.project.working_dir"])
			if workingDir != "" {
				selfComposeDirName = filepath.Base(workingDir)
			} else {
				configFiles := strings.TrimSpace(inspect.Config.Labels["com.docker.compose.project.config_files"])
				if configFiles != "" {
					selfComposeDirName = filepath.Base(filepath.Dir(configFiles))
				}
			}
		}
	})
	return selfContainerID, selfContainerName, selfComposeProject, selfComposeDirName
}

func detectSelfContainerID() string {
	hostname, _ := os.Hostname()
	h := strings.TrimSpace(strings.ToLower(hostname))
	idRe := regexp.MustCompile(`^[0-9a-f]{12,64}$`)
	if idRe.MatchString(h) {
		return h
	}

	cgroupBytes, err := os.ReadFile("/proc/self/cgroup")
	if err == nil {
		lines := strings.Split(string(cgroupBytes), "\n")
		findRe := regexp.MustCompile(`([0-9a-f]{64}|[0-9a-f]{12})`)
		for _, line := range lines {
			l := strings.ToLower(line)
			if !strings.Contains(l, "docker") && !strings.Contains(l, "kubepods") && !strings.Contains(l, "containerd") {
				continue
			}
			m := findRe.FindStringSubmatch(l)
			if len(m) > 1 {
				return m[1]
			}
		}
	}

	return ""
}

func isSelfContainerID(id string) bool {
	selfID, selfName, _, _ := getSelfIdentity()
	if selfID == "" && selfName == "" {
		return false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(id, "/"))
	if raw == "" {
		return false
	}
	if selfName != "" && raw == selfName {
		return true
	}
	if selfID == "" {
		return false
	}
	return raw == selfID || strings.HasPrefix(selfID, raw) || strings.HasPrefix(raw, selfID)
}

func forbidIfSelfContainer(c *gin.Context, containerID string) bool {
	if isSelfContainerID(containerID) {
		respondError(c, http.StatusForbidden, "容器化部署模式下，禁止管理自身容器", nil)
		return true
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return false
	}

	if inspect.Config != nil && isProtectedContainer(inspect.Config.Image, inspect.Config.Labels) {
		respondError(c, http.StatusForbidden, "容器化部署模式下，禁止管理自身容器", nil)
		return true
	}

	return false
}

func isSelfProjectName(name string) bool {
	if strings.TrimSpace(name) == protectedComposeProjectName {
		return true
	}
	_, _, composeProject, composeDir := getSelfIdentity()
	if composeProject == "" && composeDir == "" {
		return false
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	return (composeProject != "" && n == composeProject) || (composeDir != "" && n == composeDir)
}

func forbidIfSelfProject(c *gin.Context, projectName string) bool {
	if !isSelfProjectName(projectName) {
		return false
	}
	respondError(c, http.StatusForbidden, "容器化部署模式下，禁止管理自身项目", nil)
	return true
}

var composeNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func validateComposeProjectName(raw string) (string, bool) {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return "", false
	}
	if !composeNameRe.MatchString(name) {
		return "", false
	}
	return name, true
}

// ComposeProject 定义项目结构
type ComposeProject struct {
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	Compose         string    `json:"compose"`
	AutoStart       bool      `json:"autoStart"`
	Containers      int       `json:"containers"`
	Status          string    `json:"status"`
	UpdateAvailable bool      `json:"updateAvailable"`
	UpdateCount     int       `json:"updateCount"`
	CreateTime      time.Time `json:"createTime"`
	IsSelf          bool      `json:"isSelf"`
}

func setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")
}

func sseNextIDFromLastEventID(c *gin.Context) int64 {
	raw := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	if raw == "" {
		return 1
	}
	last, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || last < 0 {
		return 1
	}
	return last + 1
}

func sseWriteStringEvent(c *gin.Context, id int64, event string, data string) {
	_, _ = fmt.Fprintf(c.Writer, "id: %d\n", id)
	if event != "" && event != "message" {
		_, _ = fmt.Fprintf(c.Writer, "event: %s\n", event)
	}

	payload := strings.ReplaceAll(string(data), "\r\n", "\n")
	payload = strings.TrimRight(payload, "\n")
	if payload == "" {
		_, _ = c.Writer.WriteString("data:\n\n")
		c.Writer.Flush()
		return
	}

	for _, line := range strings.Split(payload, "\n") {
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n", line)
	}
	_, _ = c.Writer.WriteString("\n")
	c.Writer.Flush()
}

func sseWriteJSONEvent(c *gin.Context, id int64, event string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		sseWriteStringEvent(c, id, event, fmt.Sprintf(`{"type":"error","message":"marshal failed: %s"}`, err.Error()))
		return
	}
	sseWriteStringEvent(c, id, event, string(b))
}

func runCommandStreamLines(ctx context.Context, dir string, env []string, args []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stderr)
	}()
	go func() {
		wg.Wait()
		_ = pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		onLine(line)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

// withComposeEnvFile 为 docker compose 命令自动补充 --env-file 参数（若项目目录存在 .env）
// 目的：确保通过 AppStore/模板部署时生成的合并 .env 能在后续 start/stop/restart 等管理操作中参与插值
func withComposeEnvFile(projectDir string, args []string) []string {
	if len(args) == 0 {
		return args
	}
	if args[0] != "compose" {
		return args
	}
	for _, a := range args {
		if a == "--env-file" {
			return args
		}
	}

	envFile := filepath.Join(projectDir, ".env")
	if _, err := os.Stat(envFile); err != nil {
		return args
	}

	out := make([]string, 0, len(args)+2)
	out = append(out, "compose", "--env-file", envFile)
	out = append(out, args[1:]...)
	return out
}

func runComposeStreamLines(ctx context.Context, projectDir string, args []string, onLine func(string)) error {
	env := []string{"COMPOSE_PROGRESS=plain", "COMPOSE_NO_COLOR=1"}
	return runCommandStreamLines(ctx, projectDir, env, withComposeEnvFile(projectDir, args), onLine)
}

// upsertDotenvKeyValue 在 dotenv 文本中更新/插入 KEY=VALUE（尽量保留原注释与格式）
func upsertDotenvKeyValue(dotenvText string, key string, val string) string {
	key = strings.TrimSpace(key)
	if !isLikelyEnvKey(key) {
		return dotenvText
	}
	lines := strings.Split(strings.ReplaceAll(dotenvText, "\r\n", "\n"), "\n")
	needle := key + "="
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

		if strings.HasPrefix(line, needle) || line == key {
			lines[i] = prefix + key + "=" + val
			return strings.Join(lines, "\n")
		}
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, key+"="+val)
	return strings.Join(lines, "\n")
}

// findComposeFile 在项目目录中查找可用的 compose 配置文件
func findComposeFile(projectDir string) (string, error) {
	// 优先匹配常见的 docker compose 文件名
	candidates := []string{
		"docker-compose.yaml",
		"docker-compose.yml",
		"compose.yml",
		"compose.yml",
	}

	for _, name := range candidates {
		path := filepath.Join(projectDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 兜底：在目录中查找任意 *.yaml / *.yml 文件
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", err
	}

	matched := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
			matched = append(matched, name)
		}
	}

	if len(matched) == 0 {
		return "", os.ErrNotExist
	}

	sort.Strings(matched)
	return filepath.Join(projectDir, matched[0]), nil
}

// RegisterComposeRoutes 注册路由
func RegisterComposeRoutes(r *gin.RouterGroup) {
	group := r.Group("/compose")
	{
		group.GET("/list", listProjects)
		group.GET("/deploy/events", deployEvents)
		group.POST("/deploy", deployComposeTask)
		group.GET("/tasks", listComposeTasks)
		group.GET("/tasks/:id", getComposeTask)
		group.GET("/tasks/:id/events", composeTaskEvents)
		group.POST("/:name/start", startProject)
		group.GET("/:name/start/events", startProjectEvents)
		group.POST("/:name/stop", stopProject)
		group.GET("/:name/stop/events", stopProjectEvents)
		group.POST("/:name/restart", restartProject) // 添加重启路由
		group.GET("/:name/restart/events", restartProjectEvents)
		group.GET("/:name/update/events", updateProjectEvents) // 添加 SSE 更新路由
		group.POST("/:name/build", buildProject)               // 保留 POST 构建路由用于兼容
		group.GET("/:name/build/events", buildProjectEvents)   // 添加 SSE 构建路由
		group.GET("/:name/status", getStackStatus)
		group.DELETE("/:name/down", downProject)     // 添加清除(down)路由
		group.DELETE("/remove/:name", removeProject) // 修改为匹配当前请求格式
		group.GET("/:name/logs", getComposeLogs)     // 确保这个路由已添加
		group.GET("/:name/yaml", getProjectYaml)     // 添加获取 YAML 路由
		group.GET("/:name/env", getProjectEnv)       // 添加获取 .env 路由
		group.POST("/:name/yaml", saveProjectYaml)   // 添加保存 YAML 路由
		group.POST("/:name/env", saveProjectEnv)     // 添加保存 .env 路由
	}
}

type composeDeployRequest struct {
	Name      string `json:"name"`
	Compose   string `json:"compose"`
	Dotenv    string `json:"dotenv"`
	Env       string `json:"env"`
	AutoStart *bool  `json:"autoStart"`
}

func deployComposeTask(c *gin.Context) {
	var req composeDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "无效的请求参数", err)
		return
	}

	projectNameRaw := strings.TrimSpace(req.Name)
	composeRaw := strings.TrimSpace(req.Compose)
	if projectNameRaw == "" || composeRaw == "" {
		respondError(c, http.StatusBadRequest, "项目名称和配置内容不能为空", nil)
		return
	}

	projectName, ok := validateComposeProjectName(projectNameRaw)
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, projectName) {
		return
	}

	autoStart := true
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}

	taskID := fmt.Sprintf("%d", time.Now().UnixNano())
	_ = database.UpsertTask(taskID, "compose_deploy", "pending")

	go runComposeDeployTask(taskID, projectName, req.Compose, req.Dotenv, req.Env, autoStart)

	c.JSON(http.StatusOK, gin.H{
		"message": "部署任务已提交",
		"taskId":  taskID,
	})
}

func runComposeDeployTask(taskID string, projectName string, compose string, dotenvRaw string, envRaw string, autoStart bool) {
	seq := int64(0)
	appendLog := func(logType string, message string) {
		seq++
		_ = database.AppendTaskLogWithSeq(taskID, seq, time.Now(), logType, message)
	}

	done := false
	finish := func(status string, result any, errStr string) {
		if done {
			return
		}
		done = true
		_ = database.FinishTask(taskID, status, result, errStr)

		st := strings.ToLower(strings.TrimSpace(status))
		notifyType := "info"
		notifyMsg := fmt.Sprintf("Compose 项目 %s 部署任务结束", projectName)
		if st == "success" || st == "completed" {
			notifyType = "success"
			notifyMsg = fmt.Sprintf("Compose 项目 %s 部署成功", projectName)
		} else {
			notifyType = "error"
			errText := strings.TrimSpace(errStr)
			if errText == "" {
				errText = "未知错误"
			}
			notifyMsg = fmt.Sprintf("Compose 项目 %s 部署失败：%s", projectName, errText)
		}
		_ = database.SaveNotification(&database.Notification{
			Type:    notifyType,
			Message: notifyMsg,
			Read:    false,
		})
	}

	_ = database.UpsertTask(taskID, "compose_deploy", "running")
	appendLog("info", fmt.Sprintf("开始部署项目：%s", projectName))

	projectDir := filepath.Join(getProjectsBaseDir(), projectName)
	composePath := filepath.Join(projectDir, "docker-compose.yml")
	envPath := filepath.Join(projectDir, ".env")

	if _, err := os.Stat(projectDir); err == nil {
		appendLog("error", fmt.Sprintf("项目 '%s' 已存在，如需重新部署请先删除现有项目", projectName))
		finish("error", nil, "project exists")
		return
	} else if !os.IsNotExist(err) {
		appendLog("error", "检查项目目录失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		appendLog("error", "创建项目目录失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	composeToWrite := compose
	if hostRoot := settings.GetHostProjectRoot(); hostRoot != "" {
		hostProjectDir := filepath.Join(hostRoot, projectName)
		if normalized, nerr := normalizeComposeBindMountsForHost(composeToWrite, hostProjectDir); nerr == nil {
			composeToWrite = normalized
		} else {
			appendLog("error", "处理相对路径失败: "+nerr.Error())
			finish("error", nil, nerr.Error())
			return
		}
	}

	if err := os.WriteFile(composePath, []byte(composeToWrite), 0644); err != nil {
		appendLog("error", "保存配置文件失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	envMap := make(map[string]string)
	if strings.TrimSpace(envRaw) != "" {
		if err := json.Unmarshal([]byte(envRaw), &envMap); err != nil {
			appendLog("warning", "解析 env 参数失败，将仅使用 dotenv: "+err.Error())
			envMap = make(map[string]string)
		}
	}

	dotenvText := strings.ReplaceAll(dotenvRaw, "\r\n", "\n")
	if strings.TrimSpace(dotenvText) == "" && len(envMap) > 0 {
		dotenvText = renderDotenvFromMap(envMap)
	} else if len(envMap) > 0 {
		for k, v := range envMap {
			dotenvText = upsertDotenvKeyValue(dotenvText, k, v)
		}
	}

	allowedKeys := extractComposeInterpolationKeys(composeToWrite)
	dotenvText = filterDotenvByAllowedKeys(dotenvText, allowedKeys)

	if err := os.WriteFile(envPath, []byte(dotenvText), 0644); err != nil {
		appendLog("error", "保存 .env 文件失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	appendLog("success", "配置已保存")
	if !autoStart {
		finish("success", gin.H{"project": projectName, "autoStart": false}, "")
		return
	}

	ctx := context.Background()
	appendLog("info", "开始拉取镜像...")
	if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "pull"}, func(line string) {
		msgType := "info"
		if strings.Contains(line, "error") || strings.Contains(line, "Error") {
			msgType = "error"
		}
		appendLog(msgType, line)
	}); err != nil {
		appendLog("error", "拉取镜像失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	appendLog("info", "正在启动服务...")
	if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "up", "-d"}, func(line string) {
		msgType := "info"
		if strings.Contains(line, "error") || strings.Contains(line, "Error") {
			msgType = "error"
		} else if strings.Contains(line, "Created") || strings.Contains(line, "Started") {
			msgType = "success"
		}
		appendLog(msgType, line)
	}); err != nil {
		appendLog("error", "部署失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		appendLog("error", "Docker客户端初始化失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project="+projectName),
		),
	})
	if err != nil {
		appendLog("error", "获取容器状态失败: "+err.Error())
		finish("error", nil, err.Error())
		return
	}

	allRunning := true
	for _, container := range containers {
		if container.State != "running" {
			allRunning = false
			break
		}
	}
	if allRunning {
		appendLog("success", "所有服务已成功启动")
		finish("success", gin.H{"project": projectName, "containers": len(containers)}, "")
		return
	}

	appendLog("warning", "部分服务可能未正常启动，请检查状态")
	finish("success", gin.H{"project": projectName, "containers": len(containers), "warning": true}, "")
}

func listComposeTasks(c *gin.Context) {
	typesRaw := strings.TrimSpace(c.Query("types"))
	statusesRaw := strings.TrimSpace(c.Query("statuses"))
	limitRaw := strings.TrimSpace(c.Query("limit"))

	var taskTypes []string
	if typesRaw != "" {
		for _, s := range strings.Split(typesRaw, ",") {
			if v := strings.TrimSpace(s); v != "" {
				taskTypes = append(taskTypes, v)
			}
		}
	}
	if len(taskTypes) == 0 {
		taskTypes = []string{"compose_deploy"}
	}

	var statuses []string
	if statusesRaw != "" {
		for _, s := range strings.Split(statusesRaw, ",") {
			if v := strings.TrimSpace(s); v != "" {
				statuses = append(statuses, v)
			}
		}
	}

	limit := 50
	if limitRaw != "" {
		if v, err := strconv.Atoi(limitRaw); err == nil && v > 0 {
			limit = v
		}
	}

	list, err := database.ListTasks(taskTypes, statuses, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取任务列表失败", err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func getComposeTask(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		respondError(c, http.StatusBadRequest, "任务不存在", nil)
		return
	}
	t, err := database.GetTask(id)
	if err != nil {
		respondError(c, http.StatusNotFound, "任务不存在", err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func composeTaskEvents(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		respondError(c, http.StatusBadRequest, "任务ID不能为空", nil)
		return
	}

	setSSEHeaders(c)
	ctx := c.Request.Context()

	lastEventIDRaw := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	afterSeq := int64(0)
	if lastEventIDRaw != "" {
		if v, err := strconv.ParseInt(lastEventIDRaw, 10, 64); err == nil && v > 0 {
			afterSeq = v
		}
	}

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	writeLine := func(seq int64, payload any) {
		sseWriteJSONEvent(c, seq, "message", payload)
		c.Writer.Flush()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		logs, err := database.GetTaskLogsAfter(taskID, afterSeq, 500)
		if err != nil {
			writeLine(afterSeq+1, gin.H{"type": "error", "message": "读取任务日志失败: " + err.Error(), "time": time.Now().Format(time.RFC3339)})
			return
		}
		for _, r := range logs {
			writeLine(r.Seq, gin.H{"type": strings.TrimSpace(r.Type), "message": r.Message, "time": r.Time})
			afterSeq = r.Seq
		}

		t, terr := database.GetTask(taskID)
		if terr == nil {
			st := strings.ToLower(strings.TrimSpace(t.Status))
			if st == "success" || st == "error" || st == "failed" || st == "completed" {
				writeLine(afterSeq+1, gin.H{"type": "result", "status": st, "taskId": taskID, "error": t.Error})
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		case <-time.After(800 * time.Millisecond):
		}
	}
}

// startProject 启动项目
func startProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 异步执行启动命令
	go func() {
		// 使用 docker compose up 命令启动项目
		args := withComposeEnvFile(projectDir, []string{"compose", "up", "-d"})
		cmd := exec.Command("docker", args...)
		cmd.Dir = projectDir

		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Error starting project %s: %s\nOutput: %s\n", name, err.Error(), string(output))
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "项目启动指令已发送"})
}

// stopProject 停止项目
func stopProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 异步执行停止命令
	go func() {
		// 使用 docker compose stop 命令停止项目，添加 -t 2 缩短超时
		args := withComposeEnvFile(projectDir, []string{"compose", "stop", "-t", "2"})
		cmd := exec.Command("docker", args...)
		cmd.Dir = projectDir

		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Error stopping project %s: %s\nOutput: %s\n", name, err.Error(), string(output))
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "项目停止指令已发送"})
}

// restartProject 重启项目
func restartProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 异步执行重启命令
	go func() {
		// 使用 docker compose restart 命令重启项目，添加 -t 2 缩短超时
		args := withComposeEnvFile(projectDir, []string{"compose", "restart", "-t", "2"})
		cmd := exec.Command("docker", args...)
		cmd.Dir = projectDir

		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Error restarting project %s: %s\nOutput: %s\n", name, err.Error(), string(output))
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "项目重启指令已发送"})
}

// startProjectEvents 启动项目并推送 SSE 日志
func startProjectEvents(c *gin.Context) {
	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		sseWriteStringEvent(c, nextID, "log", "error: 项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头")
		return
	}
	if isSelfProjectName(name) {
		sseWriteStringEvent(c, nextID, "log", "error: 容器化部署模式下，禁止管理自身项目")
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	messageChan := make(chan string, 128)
	ctx := c.Request.Context()

	go func() {
		defer close(messageChan)

		send := func(line string) {
			line = strings.TrimSpace(line)
			if line == "" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case messageChan <- line:
				return
			}
		}

		if _, err := os.Stat(projectDir); err != nil {
			send("error: 项目目录不存在")
			return
		}

		send("info: 开始启动服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "up", "-d"}, send); err != nil {
			send(fmt.Sprintf("error: 启动失败: %s", err.Error()))
			return
		}

		send("success: 启动完成")
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return false
			}
			sseWriteStringEvent(c, nextID, "log", msg)
			nextID++
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// stopProjectEvents 停止项目并推送 SSE 日志
func stopProjectEvents(c *gin.Context) {
	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		sseWriteStringEvent(c, nextID, "log", "error: 项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头")
		return
	}
	if isSelfProjectName(name) {
		sseWriteStringEvent(c, nextID, "log", "error: 容器化部署模式下，禁止管理自身项目")
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	messageChan := make(chan string, 128)
	ctx := c.Request.Context()

	go func() {
		defer close(messageChan)

		send := func(line string) {
			line = strings.TrimSpace(line)
			if line == "" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case messageChan <- line:
				return
			}
		}

		if _, err := os.Stat(projectDir); err != nil {
			send("error: 项目目录不存在")
			return
		}

		send("info: 开始停止服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "stop", "-t", "2"}, send); err != nil {
			send(fmt.Sprintf("error: 停止失败: %s", err.Error()))
			return
		}

		send("success: 停止完成")
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return false
			}
			sseWriteStringEvent(c, nextID, "log", msg)
			nextID++
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// restartProjectEvents 重启项目并推送 SSE 日志
func restartProjectEvents(c *gin.Context) {
	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		sseWriteStringEvent(c, nextID, "log", "error: 项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头")
		return
	}
	if isSelfProjectName(name) {
		sseWriteStringEvent(c, nextID, "log", "error: 容器化部署模式下，禁止管理自身项目")
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	messageChan := make(chan string, 128)
	ctx := c.Request.Context()

	go func() {
		defer close(messageChan)

		send := func(line string) {
			line = strings.TrimSpace(line)
			if line == "" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case messageChan <- line:
				return
			}
		}

		if _, err := os.Stat(projectDir); err != nil {
			send("error: 项目目录不存在")
			return
		}

		send("info: 开始重启服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "restart", "-t", "2"}, send); err != nil {
			send(fmt.Sprintf("error: 重启失败: %s", err.Error()))
			return
		}

		send("success: 重启完成")
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return false
			}
			sseWriteStringEvent(c, nextID, "log", msg)
			nextID++
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// buildProjectEvents 构建项目并推送 SSE 事件
func buildProjectEvents(c *gin.Context) {
	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		sseWriteStringEvent(c, nextID, "log", "error: 项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头")
		return
	}
	if isSelfProjectName(name) {
		sseWriteStringEvent(c, nextID, "log", "error: 容器化部署模式下，禁止管理自身项目")
		return
	}
	pull := c.Query("pull") == "true"
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	messageChan := make(chan string, 128)
	ctx := c.Request.Context()

	go func() {
		defer close(messageChan)

		send := func(line string) {
			line = strings.TrimSpace(line)
			if line == "" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case messageChan <- line:
				return
			}
		}

		if _, err := os.Stat(projectDir); err != nil {
			send("error: 项目目录不存在")
			return
		}

		if pull {
			send("info: 开始拉取最新镜像...")
			if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "pull"}, send); err != nil {
				send(fmt.Sprintf("error: 拉取镜像失败: %s", err.Error()))
				return
			}
		}

		send("info: 开始构建服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "build"}, send); err != nil {
			send(fmt.Sprintf("error: 构建失败: %s", err.Error()))
			return
		}

		send("info: 开始重新创建服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "up", "-d", "--remove-orphans", "--force-recreate"}, send); err != nil {
			send(fmt.Sprintf("error: 重建失败: %s", err.Error()))
			return
		}

		send("success: 构建完成")
	}()

	// 发送事件
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return false
			}
			sseWriteStringEvent(c, nextID, "log", msg)
			nextID++
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// updateProjectEvents 更新项目（拉取镜像并重启）并推送 SSE 事件
func updateProjectEvents(c *gin.Context) {
	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		sseWriteStringEvent(c, nextID, "log", "error: 项目名不合法")
		return
	}
	if isSelfProjectName(name) {
		sseWriteStringEvent(c, nextID, "log", "error: 容器化部署模式下，禁止管理自身项目")
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)

	messageChan := make(chan string, 128)
	ctx := c.Request.Context()

	go func() {
		defer close(messageChan)

		send := func(line string) {
			line = strings.TrimSpace(line)
			if line == "" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case messageChan <- line:
				return
			}
		}

		if _, err := os.Stat(projectDir); err != nil {
			send("error: 项目目录不存在")
			return
		}

		send("info: 开始拉取最新镜像...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "pull"}, send); err != nil {
			send(fmt.Sprintf("error: 拉取镜像失败: %s", err.Error()))
			return
		}

		send("info: 开始重建并启动服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "up", "-d", "--remove-orphans"}, send); err != nil {
			send(fmt.Sprintf("error: 启动失败: %s", err.Error()))
			return
		}

		send("success: 项目更新完成")
	}()

	// 发送事件
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return false
			}
			sseWriteStringEvent(c, nextID, "log", msg)
			nextID++
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// buildProject 构建项目
func buildProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 使用 docker compose build 命令构建项目
	// 可以添加 --pull 选项确保拉取最新基础镜像，但这可能会慢
	args := withComposeEnvFile(projectDir, []string{"compose", "build"})
	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir

	if output, err := cmd.CombinedOutput(); err != nil {
		respondError(c, http.StatusInternalServerError, "构建失败", fmt.Errorf("%s\n%s", err.Error(), string(output)))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目构建完成"})
}

func splitImageRef(raw string) (string, string) {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return "", ""
	}
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		name := strings.TrimSpace(ref[:lastColon])
		tag := strings.TrimSpace(ref[lastColon+1:])
		if tag == "" {
			tag = "latest"
		}
		return name, tag
	}
	return ref, "latest"
}

func imageHostFromName(name string) string {
	segments := strings.Split(name, "/")
	if len(segments) == 0 {
		return ""
	}
	host := segments[0]
	if strings.Contains(host, ".") || strings.Contains(host, ":") || host == "localhost" {
		return host
	}
	return ""
}

func normalizeImageVariants(raw string) []string {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return nil
	}
	name, tag := splitImageRef(ref)
	if name == "" {
		return []string{ref}
	}
	if tag == "" {
		tag = "latest"
	}
	out := map[string]struct{}{}
	out[ref] = struct{}{}
	out[name+":"+tag] = struct{}{}

	if strings.HasPrefix(name, "docker.io/") {
		name = strings.TrimPrefix(name, "docker.io/")
		out[name+":"+tag] = struct{}{}
	}

	if strings.HasPrefix(name, "library/") {
		short := strings.TrimPrefix(name, "library/")
		out[short+":"+tag] = struct{}{}
		out["docker.io/library/"+short+":"+tag] = struct{}{}
	} else {
		out["docker.io/"+name+":"+tag] = struct{}{}
	}

	if imageHostFromName(name) == "" {
		if !strings.Contains(name, "/") {
			out["library/"+name+":"+tag] = struct{}{}
			out["docker.io/library/"+name+":"+tag] = struct{}{}
		} else {
			out["docker.io/"+name+":"+tag] = struct{}{}
		}
	}

	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	return keys
}

func hasImageUpdate(updateMap map[string]bool, image string) bool {
	if updateMap == nil {
		return false
	}
	for _, key := range normalizeImageVariants(image) {
		if updateMap[key] {
			return true
		}
	}
	return false
}

func extractComposeImagesFromProject(projectPath string) []string {
	composePath, err := findComposeFile(projectPath)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil
	}

	images := make([]string, 0)
	pushImage := func(v interface{}) {
		svc, ok := v.(map[string]interface{})
		if !ok {
			return
		}
		raw, ok := svc["image"]
		if !ok {
			return
		}
		image, ok := raw.(string)
		if !ok {
			return
		}
		image = strings.TrimSpace(image)
		if image == "" {
			return
		}
		images = append(images, image)
	}

	if servicesRaw, ok := root["services"]; ok {
		if services, ok := servicesRaw.(map[string]interface{}); ok {
			for _, svc := range services {
				pushImage(svc)
			}
			return images
		}
	}

	for _, svc := range root {
		pushImage(svc)
	}

	return images
}

// listProjects 获取项目列表
func listProjects(c *gin.Context) {
	// 创建 Docker 客户端
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "创建 Docker 客户端失败", err)
		return
	}
	defer cli.Close()

	// 获取所有带有 compose 标签的容器
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project"),
		),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取项目容器失败", err)
		return
	}

	// 用于存储项目信息的 map
	projects := make(map[string]*ComposeProject)

	// 获取所有镜像更新信息
	allUpdates, _ := database.GetAllImageUpdates()
	updateMap := make(map[string]bool)
	for _, u := range allUpdates {
		if u.LocalDigest != u.RemoteDigest {
			for _, key := range normalizeImageVariants(u.RepoTag) {
				updateMap[key] = true
			}
		}
	}

	// 遍历容器，按项目分组
	for _, container := range containers {
		// 优先使用 com.docker.compose.project 标签
		projectName := container.Labels["com.docker.compose.project"]

		// 如果没有标签，尝试从目录名推断 (兼容旧数据或非标准部署)
		if projectName == "" {
			if workingDir := container.Labels["com.docker.compose.project.working_dir"]; workingDir != "" {
				projectName = filepath.Base(workingDir)
			} else if configDir := container.Labels["com.docker.compose.project.config_files"]; configDir != "" {
				projectName = filepath.Base(filepath.Dir(configDir))
			}
		}

		if projectName == "" {
			continue
		}

		if _, exists := projects[projectName]; !exists {
			// 默认路径
			projectPath := filepath.Join(getProjectsBaseDir(), projectName)

			// 检查该路径是否存在，如果不存在则说明不是由本项目管理的
			// 尝试从 label 获取真实路径
			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				if workingDir := container.Labels["com.docker.compose.project.working_dir"]; workingDir != "" {
					projectPath = workingDir
				}
			}

			// 如果是 AppStore 部署的应用，虽然目录存在，但可能因为 compose label 问题没被识别
			// 这里我们信任 project 下的目录结构
			if _, err := os.Stat(projectPath); err == nil {
				// 目录存在，确认为本项目
			} else if workingDir := container.Labels["com.docker.compose.project.working_dir"]; workingDir != "" {
				// 外部项目
				projectPath = workingDir
			} else {
				// 既不在 project，也没有 working_dir label，跳过或者标记为外部
				// continue
			}

			// 显示名称：如果项目目录在受管项目根目录下，则优先使用目录名（兼容 AppStore 模板名）
			displayName := projectName
			projectRoot := getProjectsBaseDir()
			if rel, err := filepath.Rel(projectRoot, projectPath); err == nil && !strings.HasPrefix(rel, "..") {
				displayName = filepath.Base(projectPath)
			}

			projects[projectName] = &ComposeProject{
				Name:       displayName,
				Path:       projectPath,
				Containers: 0,
				Status:     "已停止",
				CreateTime: time.Unix(container.Created, 0),
				IsSelf:     isSelfProjectName(projectName) || isSelfProjectName(displayName),
			}
		}

		// 更新容器数量
		projects[projectName].Containers++

		// 检查镜像更新
		if hasImageUpdate(updateMap, container.Image) {
			projects[projectName].UpdateAvailable = true
			projects[projectName].UpdateCount++
		}

		// 如果有任何容器在运行，则项目状态为运行中
		if container.State == "running" {
			projects[projectName].Status = "运行中"
		}
	}

	// 补充扫描项目根目录下的项目，即使没有运行容器，也应该显示在列表中
	projectBaseDir := getProjectsBaseDir()
	entries, err := os.ReadDir(projectBaseDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			projectName := entry.Name()
			if projectName == "#recycle" {
				continue
			}
			projectDir := filepath.Join(projectBaseDir, projectName)

			// 如果该目录路径已经被某个项目使用（例如通过容器 label 推导出来），则不再重复创建项目
			alreadyUsed := false
			for _, p := range projects {
				if filepath.Clean(p.Path) == filepath.Clean(projectDir) {
					alreadyUsed = true
					break
				}
			}
			if alreadyUsed {
				continue
			}

			if _, exists := projects[projectName]; !exists {
				if _, err := findComposeFile(projectDir); err == nil {
					info, _ := entry.Info()
					projects[projectName] = &ComposeProject{
						Name:       projectName,
						Path:       projectDir,
						Containers: 0,
						Status:     "已停止",
						CreateTime: info.ModTime(), // 使用目录修改时间作为创建时间
						IsSelf:     isSelfProjectName(projectName),
					}
				}
			}
		}
	}

	for _, project := range projects {
		if project.UpdateAvailable {
			continue
		}
		images := extractComposeImagesFromProject(project.Path)
		if len(images) == 0 {
			continue
		}
		seen := make(map[string]struct{})
		for _, image := range images {
			if !hasImageUpdate(updateMap, image) {
				continue
			}
			if _, exists := seen[image]; exists {
				continue
			}
			seen[image] = struct{}{}
			project.UpdateAvailable = true
			project.UpdateCount++
		}
	}

	// 转换为数组
	result := make([]*ComposeProject, 0, len(projects))
	// projectRoot := settings.GetProjectRoot() // 不再使用 projectRoot 进行相对路径计算，而是使用 CWD
	projectRoot := getProjectsBaseDir()

	for _, project := range projects {
		if composePath, err := findComposeFile(project.Path); err == nil {
			if data, err := os.ReadFile(composePath); err == nil {
				project.Compose = string(data)
			}
		}

		if relPath, err := filepath.Rel(projectRoot, project.Path); err == nil && !strings.HasPrefix(relPath, "..") {
			if relPath == "." {
				project.Path = "project"
			} else {
				project.Path = filepath.ToSlash(filepath.Join("project", relPath))
			}
		}

		result = append(result, project)
	}

	// 按创建时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreateTime.After(result[j].CreateTime)
	})

	c.JSON(http.StatusOK, result)
}

// deployEvents 处理部署事件
func deployEvents(c *gin.Context) {
	projectNameRaw := c.Query("name")
	compose := c.Query("compose")
	dotenvRaw := c.Query("dotenv")
	envRaw := c.Query("env")

	if projectNameRaw == "" || compose == "" {
		respondError(c, http.StatusBadRequest, "项目名称和配置内容不能为空", nil)
		return
	}

	projectName, ok := validateComposeProjectName(projectNameRaw)
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, projectName) {
		return
	}

	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)

	messageChan := make(chan map[string]interface{}, 128)
	doneChan := make(chan bool)
	ctx := c.Request.Context()

	go func() {
		defer close(messageChan)

		sendMessage := func(msgType, msg string) {
			payload := map[string]interface{}{
				"type":    msgType,
				"message": msg,
			}
			select {
			case <-ctx.Done():
				return
			case <-doneChan: // 检查是否已完成
				return
			case messageChan <- payload:
				return
			}
		}

		projectDir := filepath.Join(getProjectsBaseDir(), projectName)
		composePath := filepath.Join(projectDir, "docker-compose.yml")
		envPath := filepath.Join(projectDir, ".env")

		// 检查项目目录是否已存在
		if _, err := os.Stat(projectDir); err == nil {
			// 目录已存在，提示用户并终止部署
			sendMessage("error", fmt.Sprintf("项目 '%s' 已存在，如需重新部署请先删除现有项目", projectName))
			return
		} else if !os.IsNotExist(err) {
			// 其他错误
			sendMessage("error", "检查项目目录失败: "+err.Error())
			return
		}

		// 创建项目目录（如果不存在）
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			sendMessage("error", "创建项目目录失败: "+err.Error())
			return
		}

		composeToWrite := compose
		if hostRoot := settings.GetHostProjectRoot(); hostRoot != "" {
			hostProjectDir := filepath.Join(hostRoot, projectName)
			if normalized, nerr := normalizeComposeBindMountsForHost(composeToWrite, hostProjectDir); nerr == nil {
				composeToWrite = normalized
			} else {
				sendMessage("error", "处理相对路径失败: "+nerr.Error())
				return
			}
		}

		envMap := make(map[string]string)
		if strings.TrimSpace(envRaw) != "" {
			if err := json.Unmarshal([]byte(envRaw), &envMap); err != nil {
				sendMessage("warning", "解析 env 参数失败，将仅使用 dotenv: "+err.Error())
				envMap = make(map[string]string)
			}
		}

		dotenvText := strings.ReplaceAll(dotenvRaw, "\r\n", "\n")
		if strings.TrimSpace(dotenvText) != "" && len(envMap) > 0 {
			for k, v := range envMap {
				dotenvText = upsertDotenvKeyValue(dotenvText, k, v)
			}
		}

		allowedKeys := extractComposeInterpolationKeys(composeToWrite)
		dotenvText = filterDotenvByAllowedKeys(dotenvText, allowedKeys)

		if strings.TrimSpace(dotenvText) == "" {
			if modified, err := removeDotenvEnvFileRefsFromCompose(composeToWrite); err == nil {
				composeToWrite = modified
			}
		}

		if err := os.WriteFile(composePath, []byte(composeToWrite), 0644); err != nil {
			sendMessage("error", "保存配置文件失败: "+err.Error())
			return
		}

		if strings.TrimSpace(dotenvText) != "" {
			if err := os.WriteFile(envPath, []byte(dotenvText), 0644); err != nil {
				sendMessage("error", "保存 .env 文件失败: "+err.Error())
				return
			}
		}

		sendMessage("info", "开始拉取镜像...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "pull"}, func(line string) {
			msgType := "info"
			if strings.Contains(line, "error") || strings.Contains(line, "Error") {
				msgType = "error"
			}
			sendMessage(msgType, line)
		}); err != nil {
			sendMessage("error", "拉取镜像失败: "+err.Error())
			return
		}

		sendMessage("info", "正在启动服务...")
		if err := runComposeStreamLines(ctx, projectDir, []string{"compose", "up", "-d"}, func(line string) {
			msgType := "info"
			if strings.Contains(line, "error") || strings.Contains(line, "Error") {
				msgType = "error"
			} else if strings.Contains(line, "Created") || strings.Contains(line, "Started") {
				msgType = "success"
			}
			sendMessage(msgType, line)
		}); err != nil {
			sendMessage("error", "部署失败: "+err.Error())
			return
		}

		// 检查容器状态
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			sendMessage("error", "Docker客户端初始化失败: "+err.Error())
			return
		}
		defer cli.Close()

		containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", "com.docker.compose.project="+projectName),
			),
		})
		if err != nil {
			sendMessage("error", "获取容器状态失败: "+err.Error())
			return
		}

		// 检查所有容器是否都在运行
		allRunning := true
		for _, container := range containers {
			if container.State != "running" {
				allRunning = false
				break
			}
		}

		if allRunning {
			sendMessage("success", "所有服务已成功启动")
		} else {
			sendMessage("warning", "部分服务可能未正常启动，请检查状态")
		}
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				close(doneChan) // 标记为已完成
				return false
			}
			sseWriteJSONEvent(c, nextID, "message", msg)
			nextID++
			return true
		case <-c.Request.Context().Done():
			close(doneChan)
			return false
		}
	})
}

// getStackStatus 获取堆栈状态
func getStackStatus(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "创建 Docker 客户端失败", err)
		return
	}
	defer cli.Close()

	// 获取项目的所有容器
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project="+name),
		),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取容器状态失败", err)
		return
	}

	// 转换容器信息为前端需要的格式
	containerList := make([]map[string]interface{}, 0)
	for _, container := range containers {
		// 移除 ContainerStats 调用以提高性能
		// stats, err := cli.ContainerStats(context.Background(), container.ID, false)

		containerInfo := map[string]interface{}{
			"name":      strings.TrimPrefix(container.Names[0], "/"),
			"image":     container.Image,
			"status":    container.State,
			"state":     container.State,
			"cpu":       "0%",   // 暂不采集实时数据以优化性能
			"memory":    "0 MB", // 暂不采集实时数据以优化性能
			"networkRx": "0 B",
			"networkTx": "0 B",
		}
		containerList = append(containerList, containerInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"containers": containerList,
		"isSelf":     isSelfProjectName(name),
	})
}

// cleanProjectResources 清理项目的容器和网络资源
// 1. 尝试使用 docker compose down
// 2. 扫描并强制删除所有带有 com.docker.compose.project=name 标签的残留容器
// 3. 清理关联网络
func cleanProjectResources(name string) error {
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 1. 尝试使用 docker compose down 命令停止并删除容器
	if _, err := os.Stat(projectDir); err == nil {
		args := withComposeEnvFile(projectDir, []string{"compose", "down"})
		cmd := exec.Command("docker", args...)
		cmd.Dir = projectDir

		if output, err := cmd.CombinedOutput(); err != nil {
			// 仅打印日志，不中断流程
			fmt.Printf("Warning: docker compose down failed for %s: %v\nOutput: %s\n", name, err, string(output))
		}
	}

	// 2. 使用 Docker SDK 手动清理残留容器
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("Failed to create docker client: %v", err)
	}
	defer cli.Close()

	// 查找属于该项目的所有容器
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project="+name),
		),
	})

	if err == nil {
		hostPortDedup := make(map[int]struct{})
		for _, container := range containers {
			for _, p := range container.Ports {
				if p.PublicPort == 0 {
					continue
				}
				hostPortDedup[int(p.PublicPort)] = struct{}{}
			}
		}

		for _, container := range containers {
			// 强制删除容器 (Force=true 会先停止容器)
			removeOpts := types.ContainerRemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			}
			if err := cli.ContainerRemove(context.Background(), container.ID, removeOpts); err != nil {
				fmt.Printf("Warning: Failed to force remove container %s (%s): %v\n", container.ID, container.Names, err)
			} else {
				fmt.Printf("Successfully removed container %s (%s)\n", container.ID, container.Names)
			}
		}

		if len(hostPortDedup) > 0 {
			var hostPorts []int
			for p := range hostPortDedup {
				hostPorts = append(hostPorts, p)
			}
			if tx, txErr := database.GetDB().Begin(); txErr == nil {
				if derr := database.DeleteReservedPortsByPortsTx(tx, hostPorts); derr != nil {
					_ = tx.Rollback()
				} else {
					_ = tx.Commit()
				}
			}
		}
	} else {
		fmt.Printf("Warning: Failed to list containers for project %s: %v\n", name, err)
	}

	// 3. 清理关联网络
	// 3.1 根据标签 com.docker.compose.project 清理
	if cli != nil {
		nets, nerr := cli.NetworkList(context.Background(), types.NetworkListOptions{
			Filters: filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+name)),
		})
		if nerr == nil {
			for _, net := range nets {
				_ = cli.NetworkRemove(context.Background(), net.ID)
			}
		}
		// 3.2 从 compose 文件中解析 networks 字段进行清理（若文件仍在）
		composePath := filepath.Join(projectDir, "docker-compose.yml")
		if data, rerr := os.ReadFile(composePath); rerr == nil {
			type ComposeNetworks struct {
				Networks map[string]struct{} `yaml:"networks"`
			}
			var cn ComposeNetworks
			if uerr := yaml.Unmarshal(data, &cn); uerr == nil {
				for netName := range cn.Networks {
					_ = cli.NetworkRemove(context.Background(), netName)
				}
			}
		}
	}
	return nil
}

// downProject 停止并移除项目容器和网络，但保留目录
func downProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}

	if err := cleanProjectResources(name); err != nil {
		respondError(c, http.StatusInternalServerError, "清理项目资源失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目容器和网络已清理"})
}

// removeProject 删除项目
// 功能：
// 1. 尝试使用 docker compose down 停止并删除容器
// 2. 扫描并强制删除所有带有 com.docker.compose.project=name 标签的残留容器
// 3. 删除项目文件目录
func removeProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 清理资源
	if err := cleanProjectResources(name); err != nil {
		respondError(c, http.StatusInternalServerError, "清理项目资源失败", err)
		return
	}

	if tx, err := database.GetDB().Begin(); err == nil {
		if derr := database.DeleteReservedPortsByOwnerTx(tx, name); derr != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}

	// 4. 删除项目目录
	if err := os.RemoveAll(projectDir); err != nil {
		respondError(c, http.StatusInternalServerError, "删除项目目录失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目已删除"})
}

// 添加获取 compose 日志的处理函数
func getComposeLogs(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)
	if _, err := os.Stat(projectDir); err != nil {
		respondError(c, http.StatusBadRequest, "项目目录不存在", err)
		return
	}

	setSSEHeaders(c)
	nextID := sseNextIDFromLastEventID(c)

	lines := make(chan string, 128)
	ctx := c.Request.Context()

	go func() {
		defer close(lines)
		_ = runComposeStreamLines(ctx, projectDir, []string{"compose", "logs", "-f", "--tail", "200"}, func(line string) {
			select {
			case <-ctx.Done():
				return
			case lines <- line:
				return
			}
		})
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		case msg, ok := <-lines:
			if !ok {
				return false
			}
			sseWriteStringEvent(c, nextID, "message", msg)
			nextID++
			return true
		}
	})
}

// 添加获取 YAML 配置的处理函数
// getProjectYaml 获取项目 YAML 配置
func getProjectYaml(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	yamlPath, err := findComposeFile(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			respondError(c, http.StatusBadRequest, "未找到可用的 compose 配置文件，支持: *.yaml, *.yml, docker-compose.yaml, docker-compose.yml", nil)
			return
		}
		respondError(c, http.StatusInternalServerError, "扫描配置文件失败", err)
		return
	}

	// 读取 YAML 文件
	content, err := os.ReadFile(yamlPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "读取配置文件失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
	})
}

// getProjectEnv 获取项目 .env 内容
func getProjectEnv(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	envPath := filepath.Join(projectDir, ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"content": ""})
			return
		}
		respondError(c, http.StatusInternalServerError, "读取 .env 文件失败", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(content)})
}

// saveProjectEnv 保存项目 .env 内容
func saveProjectEnv(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}

	var data struct {
		Content string `json:"content"`
	}
	if err := c.BindJSON(&data); err != nil {
		respondError(c, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	if _, err := os.Stat(projectDir); err != nil {
		respondError(c, http.StatusBadRequest, "项目目录不存在", err)
		return
	}

	envPath := filepath.Join(projectDir, ".env")
	content := strings.ReplaceAll(data.Content, "\r\n", "\n")
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		respondError(c, http.StatusInternalServerError, "保存 .env 文件失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ".env 已保存"})
}

// 添加保存 YAML 配置的处理函数
func saveProjectYaml(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		respondError(c, http.StatusBadRequest, "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头", nil)
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	var data struct {
		Content string `json:"content"`
	}

	if err := c.BindJSON(&data); err != nil {
		respondError(c, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	yamlPath, err := findComposeFile(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果不存在任何 YAML 文件，则默认写入 docker-compose.yml
			yamlPath = filepath.Join(projectDir, "docker-compose.yml")
		} else {
			respondError(c, http.StatusInternalServerError, "扫描配置文件失败", err)
			return
		}
	}

	// 保存 YAML 文件
	if err := os.WriteFile(yamlPath, []byte(data.Content), 0644); err != nil {
		respondError(c, http.StatusInternalServerError, "保存配置文件失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已保存"})
}

// 移除底部重复的 RegisterComposeRoutes
