package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
)

// ComposeProject 定义项目结构
type ComposeProject struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Compose    string    `json:"compose"`
	AutoStart  bool      `json:"autoStart"`
	Containers int       `json:"containers"`
	Status     string    `json:"status"`
	CreateTime time.Time `json:"createTime"`
}

// RegisterComposeRoutes 注册路由
func RegisterComposeRoutes(r *gin.RouterGroup) {
	group := r.Group("/compose")
	{
		group.GET("/list", listProjects)
		group.GET("/deploy/events", deployEvents)
		group.POST("/:name/start", startProject)
		group.POST("/:name/stop", stopProject)
		group.GET("/:name/status", getStackStatus)
		group.DELETE("/remove/:name", removeProject) // 修改为匹配当前请求格式
		group.GET("/:name/logs", getComposeLogs)     // 确保这个路由已添加
		group.GET("/:name/yaml", getProjectYaml)     // 添加获取 YAML 路由
		group.POST("/:name/yaml", saveProjectYaml)   // 添加保存 YAML 路由
	}
}

// startProject 启动项目
func startProject(c *gin.Context) {
	name := c.Param("name")
	projectDir := filepath.Join("data", "project", name)

	// 使用 docker compose up 命令启动项目
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = projectDir

	if output, err := cmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("启动失败: %s\n%s", err.Error(), string(output)),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目已启动"})
}

// stopProject 停止项目
func stopProject(c *gin.Context) {
	name := c.Param("name")
	projectDir := filepath.Join("data", "project", name)

	// 使用 docker compose stop 命令停止项目
	cmd := exec.Command("docker", "compose", "stop")
	cmd.Dir = projectDir

	if output, err := cmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("停止失败: %s\n%s", err.Error(), string(output)),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目已停止"})
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
		projectName := container.Labels["com.docker.compose.project"]
		if projectName == "" {
			continue
		}

		if _, exists := projects[projectName]; !exists {
			// 默认路径
			projectPath := filepath.Join("data", "project", projectName)

			// 检查该路径是否存在，如果不存在则说明不是由本项目管理的
			// 尝试从 label 获取真实路径
			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				if workingDir := container.Labels["com.docker.compose.project.working_dir"]; workingDir != "" {
					projectPath = workingDir
				}
			}

			projects[projectName] = &ComposeProject{
				Name:       projectName,
				Path:       projectPath,
				Containers: 0,
				Status:     "已停止",
				CreateTime: time.Unix(container.Created, 0),
			}
		}

		// 更新容器数量
		projects[projectName].Containers++

		// 如果有任何容器在运行，则项目状态为运行中
		if container.State == "running" {
			projects[projectName].Status = "运行中"
		}
	}

	// 转换为数组
	result := make([]*ComposeProject, 0, len(projects))
	for _, project := range projects {
		// 尝试读取 compose 文件
		composePath := filepath.Join(project.Path, "docker-compose.yml")
		if data, err := os.ReadFile(composePath); err == nil {
			project.Compose = string(data)
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
	projectName := c.Query("name")
	compose := c.Query("compose")

	if projectName == "" || compose == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名称和配置内容不能为空"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	messageChan := make(chan map[string]interface{})
	doneChan := make(chan bool)

	go func() {
		defer close(messageChan)

		sendMessage := func(msgType, msg string) {
			select {
			case <-doneChan: // 检查是否已完成
				return
			default:
				messageChan <- map[string]interface{}{
					"type":    msgType,
					"message": msg,
				}
			}
		}

		projectDir := filepath.Join("data", "project", projectName)
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

		// 保存 compose 文件，使用从请求中获取的compose内容
		if err := os.WriteFile(composePath, []byte(compose), 0644); err != nil {
			sendMessage("error", "保存配置文件失败: "+err.Error())
			return
		}

		sendMessage("info", "正在启动服务...")

		// 使用 docker compose 命令
		cmd := exec.Command("docker", "compose", "up", "-d")
		cmd.Dir = projectDir

		// 获取命令的标准输出和错误输出管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			sendMessage("error", "创建输出管道失败: "+err.Error())
			return
		}
		stderr, stderrErr := cmd.StderrPipe()
		if stderrErr != nil {
			sendMessage("error", "创建错误输出管道失败: "+stderrErr.Error())
			return
		}

		// 启动命令
		if startErr := cmd.Start(); startErr != nil {
			sendMessage("error", "启动命令失败: "+startErr.Error())
			return
		}

		// 创建扫描器读取输出
		scannerDone := make(chan bool)
		go func() {
			defer close(scannerDone)
			scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
			for scanner.Scan() {
				line := scanner.Text()
				// 根据输出内容判断类型
				msgType := "info"
				if strings.Contains(line, "error") || strings.Contains(line, "Error") {
					msgType = "error"
				} else if strings.Contains(line, "Created") || strings.Contains(line, "Started") {
					msgType = "success"
				}

				select {
				case <-doneChan: // 检查是否已完成
					return
				default:
					sendMessage(msgType, line)
				}
			}
		}()

		// 等待命令完成
		err = cmd.Wait()
		<-scannerDone // 等待扫描器完成

		if err != nil {
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
			c.SSEvent("message", msg)
			return true
		case <-time.After(30 * time.Second): // 添加超时处理
			close(doneChan) // 标记为已完成
			return false
		}
	})
}

// getStackStatus 获取堆栈状态
func getStackStatus(c *gin.Context) {
	name := c.Param("name")
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
		// 获取容器详细信息
		stats, err := cli.ContainerStats(context.Background(), container.ID, false)
		if err != nil {
			continue
		}
		defer stats.Body.Close()

		containerInfo := map[string]interface{}{
			"name":      strings.TrimPrefix(container.Names[0], "/"),
			"image":     container.Image,
			"status":    container.State,
			"state":     container.State,
			"cpu":       "0%",   // 这里可以通过解析 stats 获取实际值
			"memory":    "0 MB", // 这里可以通过解析 stats 获取实际值
			"networkRx": "0 B",
			"networkTx": "0 B",
		}
		containerList = append(containerList, containerInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"containers": containerList,
	})
}

// removeStack 函数
func removeProject(c *gin.Context) {
	name := c.Param("name") // 修改这里，使用 name 而不是 stack
	projectDir := filepath.Join("data", "project", name)

	// 使用 docker compose down 命令停止并删除容器
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = projectDir

	if output, err := cmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("删除失败: %s\n%s", err.Error(), string(output)),
		})
		return
	}

	// 删除项目目录
	if err := os.RemoveAll(projectDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除项目目录失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目已删除"})
}

// 添加获取 compose 日志的处理函数
func getComposeLogs(c *gin.Context) {
	name := c.Param("name")
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

	// 设置响应头，支持 SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建一个通道来接收所有容器的日志
	logsChan := make(chan string)
	done := make(chan bool)

	// 为每个容器启动一个 goroutine 来读取日志
	for _, container := range containers {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		go func(containerID, containerName string) {
			options := types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
				Timestamps: true,
				Tail:       "100",
			}

			logs, err := cli.ContainerLogs(context.Background(), containerID, options)
			if err != nil {
				logsChan <- fmt.Sprintf("error: 获取容器 %s 日志失败: %s", containerName, err.Error())
				return
			}
			defer logs.Close()

			reader := bufio.NewReader(logs)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						logsChan <- fmt.Sprintf("error: 读取容器 %s 日志失败: %s", containerName, err.Error())
					}
					break
				}
				logsChan <- fmt.Sprintf("data: [%s] %s", containerName, line)
			}
		}(container.ID, containerName)
	}

	// 监听客户端断开连接
	go func() {
		<-c.Request.Context().Done()
		close(done)
	}()

	// 发送日志到客户端
	c.Stream(func(w io.Writer) bool {
		select {
		case <-done:
			return false
		case msg := <-logsChan:
			c.SSEvent("message", msg)
			return true
		}
	})
}

// 添加获取 YAML 配置的处理函数
// getProjectYaml 获取项目 YAML 配置
func getProjectYaml(c *gin.Context) {
	name := c.Param("name")

	// 检查项目目录是否在根目录下
	// 这里假设所有项目都应该在 data/project 目录下
	projectDir := filepath.Join("data", "project", name)
	yamlPath := filepath.Join(projectDir, "docker-compose.yml")

	// 检查文件是否存在
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目文件不在根目录下，无法查看"})
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
	name := c.Param("name")
	var data struct {
		Content string `json:"content"`
	}

	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	projectDir := filepath.Join("data", "project", name)
	yamlPath := filepath.Join(projectDir, "docker-compose.yml")

	// 保存 YAML 文件
	if err := os.WriteFile(yamlPath, []byte(data.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已保存"})
}

// 移除底部重复的 RegisterComposeRoutes
