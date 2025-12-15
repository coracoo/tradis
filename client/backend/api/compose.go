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
    "gopkg.in/yaml.v3"

    "dockerpanel/backend/pkg/settings"
    "dockerpanel/backend/pkg/database"
)

// getProjectsBaseDir 获取项目根目录
func getProjectsBaseDir() string {
	return filepath.Join(settings.GetDataDir(), "project")
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
}

// RegisterComposeRoutes 注册路由
func RegisterComposeRoutes(r *gin.RouterGroup) {
	group := r.Group("/compose")
	{
		group.GET("/list", listProjects)
		group.GET("/deploy/events", deployEvents)
		group.POST("/:name/start", startProject)
		group.POST("/:name/stop", stopProject)
		group.POST("/:name/restart", restartProject)         // 添加重启路由
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
	name := c.Param("name")
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
	name := c.Param("name")
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
	name := c.Param("name")
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

// buildProjectEvents 构建项目并推送 SSE 事件
func buildProjectEvents(c *gin.Context) {
	name := c.Param("name")
	pull := c.Query("pull") == "true"
	projectDir := filepath.Join(getProjectsBaseDir(), name)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string)

	go func() {
		defer close(messageChan)

		// 准备构建命令
		args := []string{"compose", "build"}
		if pull {
			args = append(args, "--pull")
		}

		cmd := exec.Command("docker", args...)
		cmd.Dir = projectDir

		// 获取输出管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			messageChan <- fmt.Sprintf("error: 创建输出管道失败: %s", err.Error())
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			messageChan <- fmt.Sprintf("error: 创建错误管道失败: %s", err.Error())
			return
		}

		if err := cmd.Start(); err != nil {
			messageChan <- fmt.Sprintf("error: 启动构建失败: %s", err.Error())
			return
		}

		// 读取输出
		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for scanner.Scan() {
			messageChan <- scanner.Text()
		}

		if err := cmd.Wait(); err != nil {
			messageChan <- fmt.Sprintf("error: 构建失败: %s", err.Error())
		} else {
			messageChan <- "success: 构建完成"
		}
	}()

	// 发送事件
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return false
			}
			// 简单的日志行作为 data 发送
			c.SSEvent("log", msg)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// buildProject 构建项目
func buildProject(c *gin.Context) {
	name := c.Param("name")
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
			// 这里我们信任 data/project 下的目录结构
			if _, err := os.Stat(projectPath); err == nil {
				// 目录存在，确认为本项目
			} else if workingDir := container.Labels["com.docker.compose.project.working_dir"]; workingDir != "" {
				// 外部项目
				projectPath = workingDir
			} else {
				// 既不在 data/project，也没有 working_dir label，跳过或者标记为外部
				// continue
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

	// 补充扫描 data/project 目录下的项目
	// 即使没有运行容器，也应该显示在列表中
	projectBaseDir := getProjectsBaseDir()
	entries, err := os.ReadDir(projectBaseDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			projectName := entry.Name()
			if _, exists := projects[projectName]; !exists {
				// 检查是否存在 docker-compose.yml
				composePath := filepath.Join(projectBaseDir, projectName, "docker-compose.yml")
				if _, err := os.Stat(composePath); err == nil {
					// 只有存在 compose 文件的才认为是有效项目
					info, _ := entry.Info()
					projects[projectName] = &ComposeProject{
						Name:       projectName,
						Path:       filepath.Join(projectBaseDir, projectName),
						Containers: 0,
						Status:     "已停止",
						CreateTime: info.ModTime(), // 使用目录修改时间作为创建时间
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
		// 尝试读取 compose 文件
		composePath := filepath.Join(project.Path, "docker-compose.yml")
		if data, err := os.ReadFile(composePath); err == nil {
			project.Compose = string(data)
		}

		// 将绝对路径转换为相对路径 (相对于程序运行目录)
		// 这样可以保留 data/project/ 前缀，方便前端识别
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
	name := c.Param("name")

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
    name := c.Param("name")
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
	projectDir := filepath.Join(getProjectsBaseDir(), name)
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

	projectDir := filepath.Join(getProjectsBaseDir(), name)
	yamlPath := filepath.Join(projectDir, "docker-compose.yml")

	// 保存 YAML 文件
	if err := os.WriteFile(yamlPath, []byte(data.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已保存"})
}

// 移除底部重复的 RegisterComposeRoutes
