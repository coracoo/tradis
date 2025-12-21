package api

import (
	"bufio"
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

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(out), nil
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
		c.JSON(http.StatusForbidden, gin.H{"error": "容器化部署模式下，禁止管理自身容器"})
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
		c.JSON(http.StatusForbidden, gin.H{"error": "容器化部署模式下，禁止管理自身容器"})
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
	c.JSON(http.StatusForbidden, gin.H{"error": "容器化部署模式下，禁止管理自身项目"})
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
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Compose    string    `json:"compose"`
	AutoStart  bool      `json:"autoStart"`
	Containers int       `json:"containers"`
	Status     string    `json:"status"`
	CreateTime time.Time `json:"createTime"`
	IsSelf     bool      `json:"isSelf"`
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

func runComposeStreamLines(ctx context.Context, projectDir string, args []string, onLine func(string)) error {
	env := []string{"COMPOSE_PROGRESS=plain", "COMPOSE_NO_COLOR=1"}
	return runCommandStreamLines(ctx, projectDir, env, args, onLine)
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
		group.POST("/:name/start", startProject)
		group.GET("/:name/start/events", startProjectEvents)
		group.POST("/:name/stop", stopProject)
		group.GET("/:name/stop/events", stopProjectEvents)
		group.POST("/:name/restart", restartProject) // 添加重启路由
		group.GET("/:name/restart/events", restartProjectEvents)
		group.POST("/:name/build", buildProject)             // 保留 POST 构建路由用于兼容
		group.GET("/:name/build/events", buildProjectEvents) // 添加 SSE 构建路由
		group.GET("/:name/status", getStackStatus)
		group.DELETE("/:name/down", downProject)     // 添加清除(down)路由
		group.DELETE("/remove/:name", removeProject) // 修改为匹配当前请求格式
		group.GET("/:name/logs", getComposeLogs)     // 确保这个路由已添加
		group.GET("/:name/yaml", getProjectYaml)     // 添加获取 YAML 路由
		group.POST("/:name/yaml", saveProjectYaml)   // 添加保存 YAML 路由
	}
}

// startProject 启动项目
func startProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 异步执行启动命令
	go func() {
		// 使用 docker compose up 命令启动项目
		cmd := exec.Command("docker", "compose", "up", "-d")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 异步执行停止命令
	go func() {
		// 使用 docker compose stop 命令停止项目，添加 -t 2 缩短超时
		cmd := exec.Command("docker", "compose", "stop", "-t", "2")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 异步执行重启命令
	go func() {
		// 使用 docker compose restart 命令重启项目，添加 -t 2 缩短超时
		cmd := exec.Command("docker", "compose", "restart", "-t", "2")
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

// buildProject 构建项目
func buildProject(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 使用 docker compose build 命令构建项目
	// 可以添加 --pull 选项确保拉取最新基础镜像，但这可能会慢
	cmd := exec.Command("docker", "compose", "build")
	cmd.Dir = projectDir

	if output, err := cmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("构建失败: %s\n%s", err.Error(), string(output)),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目构建完成"})
}

// listProjects 获取项目列表
func listProjects(c *gin.Context) {
	// 创建 Docker 客户端
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 用于存储项目信息的 map
	projects := make(map[string]*ComposeProject)

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

	// 转换为数组
	result := make([]*ComposeProject, 0, len(projects))
	// projectRoot := settings.GetProjectRoot() // 不再使用 projectRoot 进行相对路径计算，而是使用 CWD

	cwd, _ := os.Getwd()

	for _, project := range projects {
		if composePath, err := findComposeFile(project.Path); err == nil {
			if data, err := os.ReadFile(composePath); err == nil {
				project.Compose = string(data)
			}
		}

		// 将绝对路径转换为相对路径 (相对于程序运行目录)
		// 这样可以保留 project/ 前缀，方便前端识别
		if relPath, err := filepath.Rel(cwd, project.Path); err == nil {
			project.Path = filepath.ToSlash(relPath)
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

	if projectNameRaw == "" || compose == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名称和配置内容不能为空"})
		return
	}

	projectName, ok := validateComposeProjectName(projectNameRaw)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
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

		if err := os.WriteFile(composePath, []byte(composeToWrite), 0644); err != nil {
			sendMessage("error", "保存配置文件失败: "+err.Error())
			return
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		cmd := exec.Command("docker", "compose", "down")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}

	if err := cleanProjectResources(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	// 清理资源
	if err := cleanProjectResources(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除项目目录失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目已删除"})
}

// 添加获取 compose 日志的处理函数
func getComposeLogs(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	projectDir := filepath.Join(getProjectsBaseDir(), name)
	if _, err := os.Stat(projectDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目目录不存在"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	yamlPath, err := findComposeFile(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到可用的 compose 配置文件，支持: *.yaml, *.yml, docker-compose.yaml, docker-compose.yml"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描配置文件失败: " + err.Error()})
		return
	}

	// 读取 YAML 文件
	content, err := os.ReadFile(yamlPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配置文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
	})
}

// 添加保存 YAML 配置的处理函数
func saveProjectYaml(c *gin.Context) {
	name, ok := validateComposeProjectName(c.Param("name"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名不合法：仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头"})
		return
	}
	if forbidIfSelfProject(c, name) {
		return
	}
	var data struct {
		Content string `json:"content"`
	}

	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	yamlPath, err := findComposeFile(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果不存在任何 YAML 文件，则默认写入 docker-compose.yml
			yamlPath = filepath.Join(projectDir, "docker-compose.yml")
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描配置文件失败: " + err.Error()})
			return
		}
	}

	// 保存 YAML 文件
	if err := os.WriteFile(yamlPath, []byte(data.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已保存"})
}

// 移除底部重复的 RegisterComposeRoutes
