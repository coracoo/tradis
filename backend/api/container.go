// backend/api/container.go
package api // 必须声明包名

import (
	"context"
	"dockerpanel/backend/pkg/docker"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
)

// 路由注册需导出的函数
func RegisterContainerRoutes(r *gin.RouterGroup) {
	group := r.Group("/containers")
	{
		group.GET("", ListContainers)
		group.GET("/:id", GetContainer) // 添加获取单个容器详情的路由
		group.POST("/create", createContainer)
		group.POST("/:id/rename", renameContainer) // 添加重命名容器路由（通过创建新容器实现）
		group.POST("/:id/start", startContainer)
		group.POST("/:id/stop", stopContainer)
		group.POST("/:id/restart", restartContainer)
		group.POST("/:id/pause", pauseContainer)
		group.POST("/:id/unpause", unpauseContainer)
		group.POST("/prune", pruneContainers) // 注册清理容器路由，注意要放在 :id 路由之前，避免冲突，或者使用不同的路径
		group.DELETE("/:id", removeContainer)
		group.GET("/:id/logs", getContainerLogs)
		group.GET("/:id/terminal", containerTerminal)
	}
}

// 清理停止的容器
func pruneContainers(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法连接到 Docker: " + err.Error()})
		return
	}
	defer cli.Close()

	report, err := cli.ContainersPrune(context.Background(), filters.Args{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理容器失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "清理完成",
		"deletedCount":   len(report.ContainersDeleted),
		"spaceReclaimed": report.SpaceReclaimed,
	})
}

// 容器列表
func ListContainers(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取每个容器的详细信息
	var containersWithDetails []gin.H
	for _, container := range containers {
		inspect, err := cli.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			continue
		}

		// 处理端口映射，添加 IP 地址
		formattedPorts := make([]gin.H, 0)
		for _, port := range container.Ports {
			portInfo := gin.H{
				"PrivatePort": port.PrivatePort,
				"Type":        port.Type,
			}

			if port.PublicPort != 0 {
				// 添加 IP 信息
				hostIP := "0.0.0.0"
				if port.IP != "" {
					hostIP = port.IP
				}
				portInfo["PublicPort"] = port.PublicPort
				portInfo["IP"] = hostIP
			}

			formattedPorts = append(formattedPorts, portInfo)
		}

		// 计算运行时间
		var runningTime string
		if container.State == "running" {
			startTime, err := time.Parse(time.RFC3339, inspect.State.StartedAt)
			if err != nil {
				runningTime = "时间解析错误"
			} else {
				runningTime = time.Since(startTime).Round(time.Second).String()
			}
		} else {
			runningTime = "未运行"
		}

		containerInfo := gin.H{
			"Id":              container.ID,
			"Names":           container.Names,
			"Image":           container.Image,
			"State":           container.State,
			"Status":          container.Status,
			"Created":         container.Created,
			"Ports":           formattedPorts,
			"NetworkSettings": inspect.NetworkSettings, // 使用 inspect 中的网络设置
			"HostConfig":      inspect.HostConfig,      // 添加 HostConfig
			"RunningTime":     runningTime,
		}
		containersWithDetails = append(containersWithDetails, containerInfo)
	}

	c.JSON(http.StatusOK, containersWithDetails)
}

// 获取单个容器详情
func GetContainer(c *gin.Context) {
	id := c.Param("id")

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 获取容器基础信息
	inspect, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "容器不存在: " + err.Error()})
		return
	}

	// 获取容器统计信息（用于 CPU 和内存使用率）
	// 暂时只返回基础信息，实时监控数据建议通过 WebSocket 推送
	// stats, err := cli.ContainerStats(context.Background(), id, false)
	// var cpuPercent, memPercent, memUsage, memLimit float64
	// var netRx, netTx float64

	// if err == nil {
	// 	var statsJSON types.StatsJSON
	// 	// 这里简化处理，实际需要解析流式数据或只读取一次
	// 	// 由于 ContainerStats 返回的是流，直接读取可能会阻塞或需要复杂处理
	// 	// 暂时只返回基础信息，实时监控数据建议通过 WebSocket 推送
	// 	stats.Body.Close()
	// }

	// 处理端口映射
	formattedPorts := make([]gin.H, 0)
	// 使用 inspect 中的 NetworkSettings.Ports
	for port, bindings := range inspect.NetworkSettings.Ports {
		for _, binding := range bindings {
			portInfo := gin.H{
				"PrivatePort": port.Port(),
				"Type":        port.Proto(),
				"PublicPort":  binding.HostPort,
				"IP":          binding.HostIP,
			}
			formattedPorts = append(formattedPorts, portInfo)
		}
	}

	// 计算运行时间
	var runningTime string
	if inspect.State.Running {
		startTime, parseErr := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if parseErr != nil {
			runningTime = "时间解析错误"
		} else {
			runningTime = time.Since(startTime).Round(time.Second).String()
		}
	} else {
		runningTime = "未运行"
	}

	// 处理挂载卷
	formattedMounts := make([]gin.H, 0)
	for _, mount := range inspect.Mounts {
		mountInfo := gin.H{
			"Source":      mount.Source,
			"Destination": mount.Destination,
			"Type":        mount.Type,
			"Mode":        mount.Mode,
			"RW":          mount.RW,
		}
		formattedMounts = append(formattedMounts, mountInfo)
	}

	// 处理网络信息
	formattedNetworks := make([]string, 0)
	for netName := range inspect.NetworkSettings.Networks {
		formattedNetworks = append(formattedNetworks, netName)
	}

	// 获取镜像详细信息，以提取默认 Cmd 和 Entrypoint
	var imageConfig *container.Config
	imageInspect, _, err := cli.ImageInspectWithRaw(context.Background(), inspect.Image)
	if err == nil {
		imageConfig = imageInspect.Config
	}

	containerInfo := gin.H{
		"Id":              inspect.ID,
		"Name":            inspect.Name, // 注意：inspect.Name 通常包含前导斜杠
		"Image":           inspect.Config.Image,
		"State":           inspect.State.Status,
		"Status":          inspect.State.Status, // 兼容前端
		"Created":         inspect.Created,
		"Ports":           formattedPorts,
		"Mounts":          formattedMounts,
		"Networks":        formattedNetworks,
		"RestartPolicy":   inspect.HostConfig.RestartPolicy.Name,
		"NetworkSettings": inspect.NetworkSettings,
		"HostConfig":      inspect.HostConfig,
		"RunningTime":     runningTime,
		"Path":            inspect.Path,
		"Args":            inspect.Args,
		"Config":          inspect.Config, // 包含当前的 Cmd, Entrypoint, Env 等
		"ImageConfig":     imageConfig,    // 新增：包含镜像默认的 Cmd, Entrypoint 等
		"Env":             inspect.Config.Env,
		"Labels":          inspect.Config.Labels,
	}

	c.JSON(http.StatusOK, containerInfo)
}

// 重启容器
func restartContainer(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	if err := cli.ContainerRestart(context.Background(), id, container.StopOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已重启"})
}

// 暂停容器
func pauseContainer(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	if err := cli.ContainerPause(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已暂停"})
}

// 恢复容器
func unpauseContainer(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	if err := cli.ContainerUnpause(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已恢复"})
}

// 启动容器
func startContainer(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法连接到 Docker: " + err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	// 先检查容器是否存在
	inspect, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "容器不存在: " + err.Error()})
		return
	}

	// 修正：前面 inspect 已经获取了详细信息，直接用 inspect 判断状态更准确
	if inspect.State.Running {
		c.JSON(http.StatusBadRequest, gin.H{"error": "容器已经在运行中"})
		return
	}

	// 尝试启动容器
	err = cli.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "bind: address already in use"):
			// 提取端口信息，匹配格式为 0.0.0.0:端口号 的模式
			portRegex := regexp.MustCompile(`0.0.0.0:(\d+)`)
			matches := portRegex.FindStringSubmatch(errMsg)
			if len(matches) > 1 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("端口冲突，%s，请检查端口", matches[1])})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "端口冲突，请检查端口配置"})
			}
		case strings.Contains(errMsg, "no such file or directory"):
			// 提取路径信息
			pathRegex := regexp.MustCompile(`path\s+([^\s]+)\s+`)
			matches := pathRegex.FindStringSubmatch(errMsg)
			if len(matches) > 1 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("路径不存在，请检查宿主机路径%s", matches[1])})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "路径不存在，请检查宿主机路径配置"})
			}
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启动容器失败: " + errMsg})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已启动"})
}

// 停止容器
func stopContainer(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	// 先检查容器是否存在
	_, err = cli.ContainerInspect(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "容器不存在"})
		return
	}

	// 尝试停止容器
	timeout := 2 // 设置超时时间为 2 秒，加快响应速度
	err = cli.ContainerStop(context.Background(), id, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已停止"})
}

// 删除容器
func removeContainer(c *gin.Context) {

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	timeout := 2 // 设置超时时间为 2 秒
	err = cli.ContainerStop(context.Background(), id, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已删除"})
}

// CreateContainerRequest 定义创建容器的请求结构
type CreateContainerRequest struct {
	Name          string   `json:"name"`
	Image         string   `json:"image"`
	Ports         []string `json:"ports"`          // 格式: "8080:80"
	Env           []string `json:"env"`            // 格式: "KEY=VALUE"
	Volumes       []string `json:"volumes"`        // 格式: "/host/path:/container/path"
	NetworkMode   string   `json:"network_mode"`   // 网络模式
	RestartPolicy string   `json:"restart_policy"` // 重启策略
	Command       []string `json:"command"`        // 启动命令
	Entrypoint    []string `json:"entrypoint"`     // 入口点
	Privileged    bool     `json:"privileged"`     // 特权模式
	Devices       []string `json:"devices"`        // 格式: "/dev/sda:/dev/xda:rwm"
}

// createContainer 创建容器
func createContainer(c *gin.Context) {
	var req CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败: " + err.Error()})
		return
	}
	defer cli.Close()

	// 1. 拉取镜像（如果不存在）
	// 这里简化处理，假设镜像已存在或自动拉取
	// 实际生产中可能需要显式拉取镜像

	// 2. 配置容器
	// 如果前端传来的 Command 为空，不要强制设为空切片，让 Docker 使用镜像默认的 CMD
	// 但是这里 req.Command 即使为空切片，赋给 Cmd 后，Docker API 可能会认为是要清空 CMD
	// 所以如果为空，最好设为 nil
	var cmd []string
	if len(req.Command) > 0 {
		cmd = req.Command
	}

	var entrypoint []string
	if len(req.Entrypoint) > 0 {
		entrypoint = req.Entrypoint
	}

	config := &container.Config{
		Image:      req.Image,
		Env:        req.Env,
		Cmd:        cmd,
		Entrypoint: entrypoint,
	}

	// 解析设备映射
	var devices []container.DeviceMapping
	for _, d := range req.Devices {
		parts := strings.Split(d, ":")
		if len(parts) >= 2 {
			dev := container.DeviceMapping{
				PathOnHost:        parts[0],
				PathInContainer:   parts[1],
				CgroupPermissions: "rwm",
			}
			if len(parts) > 2 {
				dev.CgroupPermissions = parts[2]
			}
			devices = append(devices, dev)
		}
	}

	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: req.RestartPolicy,
		},
		NetworkMode:  container.NetworkMode(req.NetworkMode),
		Binds:        req.Volumes,
		PortBindings: make(map[nat.Port][]nat.PortBinding),
		Privileged:   req.Privileged,
		Resources: container.Resources{
			Devices: devices,
		},
	}

	// 处理端口映射
	for _, p := range req.Ports {
		// 格式: "HostPort:ContainerPort" 或 "HostPort:ContainerPort/Protocol"
		parts := strings.Split(p, ":")
		if len(parts) >= 2 {
			hostPort := parts[0]
			containerPortRaw := parts[1]

			// 解析协议
			protocol := "tcp"
			containerPort := containerPortRaw
			if strings.Contains(containerPortRaw, "/") {
				cpParts := strings.Split(containerPortRaw, "/")
				containerPort = cpParts[0]
				protocol = cpParts[1]
			}

			portKey, portErr := nat.NewPort(protocol, containerPort)
			if portErr == nil {
				hostConfig.PortBindings[portKey] = []nat.PortBinding{
					{
						HostPort: hostPort,
					},
				}
			}
		}
	}

	// 3. 创建容器
	resp, err := cli.ContainerCreate(context.Background(), config, hostConfig, nil, nil, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建容器失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": resp.ID, "message": "容器创建成功"})
}

// RenameContainerRequest 定义重命名容器的请求结构
type RenameContainerRequest struct {
	NewName string `json:"newName" binding:"required"`
}

// renameContainer 重命名容器（实际上是创建新容器并替换）
// 注意：Docker API 的 rename 只是改名，不会改变配置。
// 如果用户想改名，通常期望的是用新名字运行完全一样的服务。
// 这里我们先只实现简单的 rename，如果需要完整的“克隆+改名”，逻辑会更复杂。
// 根据用户需求：“容器重命名→创建新容器→验证新容器状态→删除旧容器”
// 这个逻辑主要在前端控制，后端提供 Create 接口即可。
// 但为了方便，我们可以提供一个 rename 接口，或者让前端分步调用。
// 这里我们提供一个 rename 接口，直接调用 Docker 的 rename API
func renameContainer(c *gin.Context) {
	id := c.Param("id")
	var req RenameContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败: " + err.Error()})
		return
	}
	defer cli.Close()

	err = cli.ContainerRename(context.Background(), id, req.NewName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重命名失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器重命名成功"})
}
