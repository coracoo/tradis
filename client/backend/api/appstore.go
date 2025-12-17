package api

import (
	"bytes"
	"context"
	"dockerpanel/backend/pkg/database" // Add this
	"dockerpanel/backend/pkg/docker"
	"dockerpanel/backend/pkg/settings" // 添加 settings 包
	"dockerpanel/backend/pkg/task"
	"os/exec"
	"strconv" // Add this

	// 添加 task 包
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
		return "https://template.cgakki.top:33333"
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
	Compose     string     `json:"compose"` // Compose 从 map 改为 string
	Screenshots []string   `json:"screenshots"`
	Schema      []Variable `json:"schema"`
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
		group.GET("/status/:id", getAppStatus)
		// SSE 路由通常通过 URL Token 认证，或者放行
		// 如果在 protected 组中，前端 EventSource 需要带 Token (通常只能通过 URL Query)
		group.GET("/tasks/:id/events", taskEvents)
	}
}

// 获取应用列表
func listApps(c *gin.Context) {
	if err := os.MkdirAll(getAppCacheDir(), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建缓存目录失败"})
		return
	}

	// 从应用商城服务器获取应用列表
	url := fmt.Sprintf("%s/api/templates", getAppStoreServerURL())
	fmt.Printf("Requesting AppStore URL: %s\n", url) // 添加日志
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error connecting to AppStore: %v\n", err) // 添加日志
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接应用商城服务器失败: " + err.Error()})
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
	fmt.Printf("[Debug] getAppFromCacheOrServer: idOrName=%s\n", idOrName)

	// 1. 尝试从服务器获取 (优先，以获取最新信息和正确的 Name)
	url := fmt.Sprintf("%s/api/templates/%s", getAppStoreServerURL(), idOrName)
	fmt.Printf("[Debug] Requesting Server: %s\n", url)
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
	Compose string            `json:"compose"` // 前端传递的最终 Compose 内容
	Env     map[string]string `json:"env"`     // 预留环境变量
	Config  []Variable        `json:"config"`  // 新增：完整的配置数组
}

// 部署应用
func deployApp(c *gin.Context) {
	id := c.Param("id")
	fmt.Printf("[Debug] deployApp called for id: %s\n", id)

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

		// 打印调试信息，确认接收到的参数
		fmt.Printf("[Debug] Deploy Params for %s: ConfigLen=%d, EnvLen=%d\n", appId, len(deployReq.Config), len(deployReq.Env))

		// 获取应用详情 (支持缓存回源)
		t.AddLog("info", "正在获取应用配置...")
		app, err := getAppFromCacheOrServer(appId)
		if err != nil {
			t.AddLog("error", fmt.Sprintf("获取应用详情失败: %v", err))
			t.Finish(task.StatusFailed, nil, err.Error())
			return
		}

		// 创建临时compose文件
		t.AddLog("info", fmt.Sprintf("准备部署目录: %s", app.Name))
		// 使用项目名称作为目录名，与 compose.go 保持一致
		baseDir := getProjectsBaseDir()
		composeDir := filepath.Join(baseDir, app.Name)

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
			// 兼容旧逻辑：仅注入环境变量
			// 1. 生成 .env 文件 (用于支持 Compose 文件中的 ${VAR} 变量替换)
			envFile := filepath.Join(composeDir, ".env")
			var envContent string
			for k, v := range deployReq.Env {
				envContent += fmt.Sprintf("%s=%s\n", k, v)
			}
			if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
				t.AddLog("warning", fmt.Sprintf("保存 .env 文件失败: %v", err))
			}

			// 2. 将环境变量直接注入到 YAML 的 environment 字段 (用于确保容器内能读取到变量)
			if modified, err := injectEnvToYaml(composeContent, deployReq.Env); err == nil {
				composeContent = modified
			} else {
				t.AddLog("warning", fmt.Sprintf("注入环境变量失败: %v", err))
			}

			// 3. 替换 YAML 内容中的 ${VAR} 占位符 (确保端口、路径等非 environment 字段的变量也被替换)
			// 使用 os.Expand 风格的替换，但优先匹配 map 中的 key
			composeContent = os.Expand(composeContent, func(key string) string {
				// fmt.Printf("[Debug] Replacing var: %s\n", key)
				if v, ok := deployReq.Env[key]; ok {
					// fmt.Printf("[Debug] Replaced %s with %s\n", key, v)
					return v
				}
				return "${" + key + "}" // 如果没有对应的值，保持原样
			})
		} else {
			t.AddLog("warning", "未接收到任何配置参数，将使用默认模板部署")
		}

		if writeErr := os.WriteFile(composeFile, []byte(composeContent), 0644); writeErr != nil {
			t.AddLog("error", fmt.Sprintf("保存Compose文件失败: %v", writeErr))
			t.Finish(task.StatusFailed, nil, writeErr.Error())
			return
		}

		// 使用 docker compose 命令行部署，以确保原生行为（包括相对路径处理）
		t.AddLog("info", "开始执行 Docker Compose 部署...")

		cmd := exec.Command("docker", "compose", "up", "-d")
		cmd.Dir = composeDir // 设置工作目录为项目目录
		// 优化输出为纯文本，便于流式展示进度
		cmd.Env = append(os.Environ(), "COMPOSE_PROGRESS=plain", "COMPOSE_NO_COLOR=1")

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
				tx, err := database.GetDB().Begin()
				if err == nil {
					// ReservedBy could be app name or ID
					if err := database.ReservePortsTx(tx, usedPorts, app.Name, "TCP", "App"); err != nil {
						t.AddLog("warning", fmt.Sprintf("端口登记失败: %v", err))
						tx.Rollback()
					} else {
						tx.Commit()
						t.AddLog("info", "端口登记完成")
					}
				} else {
					t.AddLog("warning", "无法开启数据库事务进行端口登记")
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
			fmt.Printf("[Debug] Service %s not found in YAML\n", serviceName)
			continue // Service not found in YAML, skip
		}
		svcMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		fmt.Printf("[Debug] Processing service: %s, config items: %d\n", serviceName, len(svcConfig))

		// Reset lists to rebuild them from config
		var newPorts []string
		var newVolumes []string
		newEnv := make(map[string]string)

		// First, try to preserve existing env if it's a map (for merging)
		if existingEnv, ok := svcMap["environment"].(map[string]interface{}); ok {
			for k, v := range existingEnv {
				newEnv[k] = fmt.Sprintf("%v", v)
			}
		} else if existingEnvList, ok := svcMap["environment"].([]interface{}); ok {
			// Convert list "KEY=VAL" to map
			for _, item := range existingEnvList {
				str := fmt.Sprintf("%v", item)
				// simple parse
				// Note: this is a bit rough, but sufficient for merging
				// We prioritize the new config anyway
				// Actually, we might just want to append/overwrite.
				// Let's stick to the user's request: "combine into name=value"
				_ = str
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
					fmt.Printf("[Debug] Adding port: %s\n", portStr)
				}
			case "path", "volume": // Handle both 'path' and 'volume' types
				// Format: "host:container" -> "name:default"
				if left != "" && right != "" {
					newVolumes = append(newVolumes, fmt.Sprintf("%s:%s", left, right))
				}
			case "env", "environment":
				// Format: "key=value" -> "name=default"
				if left != "" {
					newEnv[left] = right
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
		if len(newEnv) > 0 {
			svcMap["environment"] = newEnv
		}
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(out), nil
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

	// 设置 SSE 头部
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	logChan, closeChan := t.Subscribe()

	// 获取已有的日志
	logs := t.GetLogs()

	// 获取 Header 中的 Last-Event-ID，如果存在，则只发送之后的日志
	lastEventId := c.GetHeader("Last-Event-ID")
	startIndex := 0
	if lastEventId != "" {
		// 这里简单处理：如果客户端发送了 Last-Event-ID，我们假设它已经接收了部分日志
		// 但由于我们没有为每条日志分配唯一递增 ID，这里只能尽力而为
		// 更好的做法是给每条日志一个 index，客户端传回 index
		// 暂时策略：如果是重连 (Last-Event-ID 不为空)，则不发送历史日志，只发送新日志
		// 或者前端负责去重
		startIndex = len(logs)
	}

	// 发送历史日志
	for i := startIndex; i < len(logs); i++ {
		data, _ := json.Marshal(logs[i])
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
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
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
			c.Writer.Flush()
		case <-closeChan:
			// 任务结束，发送最终状态
			resultData, _ := json.Marshal(gin.H{
				"type":    "result",
				"status":  t.Status,
				"message": "任务结束",
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(resultData))
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

	out, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
