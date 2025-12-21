package api

import (
	"context"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"dockerpanel/backend/pkg/system"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

func RegisterSystemRoutes(r *gin.RouterGroup) {
	group := r.Group("/system")
	{
		group.GET("/info", getSystemInfo)
		group.GET("/stats", getSystemStats)
		group.GET("/events", getSystemEvents)
		group.POST("/notifications", addNotification)
		group.GET("/notifications", getNotifications)
		group.DELETE("/notifications/:id", deleteNotification)
		group.POST("/notifications/read", markNotificationsRead)
		group.POST("/navigation/rebuild", rebuildNavigation)
	}
}

func rebuildNavigation(c *gin.Context) {
	containerID := strings.TrimSpace(c.Query("container_id"))
	projectName := strings.TrimSpace(c.Query("project"))

	if containerID != "" {
		system.RebuildAutoNavigationForContainer(containerID)
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}
	if projectName != "" {
		system.RebuildAutoNavigationForComposeProject(projectName)
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}

	system.RebuildAutoNavigationAll()
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func parseDockerMajorVersion(raw string) int {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseAPIVersion(raw string) (major int, minor int, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || maj < 0 {
		return 0, 0, false
	}
	min, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || min < 0 {
		return 0, 0, false
	}
	return maj, min, true
}

func isAPIVersionGE(raw string, wantMajor int, wantMinor int) bool {
	maj, min, ok := parseAPIVersion(raw)
	if !ok {
		return false
	}
	if maj != wantMajor {
		return maj > wantMajor
	}
	return min >= wantMinor
}

func shouldApplyMinAPIVersionFix(engineVersion string, apiVersion string) bool {
	if parseDockerMajorVersion(engineVersion) >= 29 {
		return true
	}
	return isAPIVersionGE(apiVersion, 1, 52)
}

// 获取系统信息
func getSystemInfo(c *gin.Context) {
	// 创建Docker客户端
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败: " + err.Error()})
		return
	}
	defer cli.Close()

	info, err := cli.Info(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取Docker信息失败: " + err.Error()})
		return
	}

	versionInfo, err := cli.ServerVersion(context.Background())
	if err != nil {
		versionInfo = types.Version{}
	}

	engineVersion := strings.TrimSpace(versionInfo.Version)
	if engineVersion == "" {
		engineVersion = strings.TrimSpace(info.ServerVersion)
	}

	daemonMinAPIVersion := ""
	daemonCfg, daemonErr := docker.GetDaemonConfig()
	if daemonErr == nil && daemonCfg != nil {
		daemonMinAPIVersion = strings.TrimSpace(daemonCfg.MinAPIVersion)
	}

	minAPIVersionFixTarget := "1.43"
	minAPIVersionFixNeeded := shouldApplyMinAPIVersionFix(engineVersion, versionInfo.APIVersion)
	minAPIVersionFixApplied := false
	minAPIVersionFixError := ""
	if minAPIVersionFixNeeded && daemonMinAPIVersion == "" {
		if err := docker.UpdateDaemonConfig(&docker.DaemonConfig{MinAPIVersion: minAPIVersionFixTarget}); err != nil {
			minAPIVersionFixError = err.Error()
		} else {
			minAPIVersionFixApplied = true
			daemonMinAPIVersion = minAPIVersionFixTarget
		}
	}

	// 获取数据卷列表以统计数量
	volumeList, err := cli.VolumeList(context.Background(), volume.ListOptions{})
	volumeCount := 0
	if err == nil {
		volumeCount = len(volumeList.Volumes)
	}

	// 获取网络列表以统计数量
	networkList, err := cli.NetworkList(context.Background(), types.NetworkListOptions{})
	networkCount := 0
	if err == nil {
		networkCount = len(networkList)
	}

	// 获取系统内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取内存信息失败: " + err.Error()})
		return
	}

	// 获取CPU信息
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取CPU信息失败: " + err.Error()})
		return
	}

	// 获取磁盘信息
	diskInfo, err := disk.Usage("/")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取磁盘信息失败: " + err.Error()})
		return
	}

	// 计算Docker运行时间
	startTime, err := time.Parse(time.RFC3339, info.SystemTime)
	if err != nil {
		startTime = time.Now()
	}
	uptime := int64(time.Since(startTime).Seconds())

	// 构建响应
	response := gin.H{
		"ServerVersion":           info.ServerVersion,
		"DockerVersion":           versionInfo.Version,
		"DockerAPIVersion":        versionInfo.APIVersion,
		"DaemonMinAPIVersion":     daemonMinAPIVersion,
		"MinAPIVersionFixNeeded":  minAPIVersionFixNeeded,
		"MinAPIVersionFixApplied": minAPIVersionFixApplied,
		"MinAPIVersionFixError":   minAPIVersionFixError,
		"MinAPIVersionFixTarget":  minAPIVersionFixTarget,
		"DaemonConfigReadable":    daemonErr == nil,
		"NCPU":                    info.NCPU,
		"MemTotal":                memInfo.Total,
		"MemUsage":                memInfo.Used,
		"DiskTotal":               diskInfo.Total,
		"DiskUsage":               diskInfo.Used,
		"CpuUsage":                cpuPercent[0],
		"SystemTime":              info.SystemTime,
		"SystemUptime":            uptime,
		"OS":                      runtime.GOOS,
		"Arch":                    runtime.GOARCH,
		"Containers":              info.Containers,
		"ContainersRunning":       info.ContainersRunning,
		"ContainersPaused":        info.ContainersPaused,
		"ContainersStopped":       info.ContainersStopped,
		"Images":                  info.Images,
		"Volumes":                 volumeCount,
		"Networks":                networkCount,
	}

	c.JSON(http.StatusOK, response)
}

// 获取系统实时监控数据
func getSystemStats(c *gin.Context) {
	// 获取CPU使用率
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取CPU信息失败: " + err.Error()})
		return
	}

	// 获取内存使用率
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取内存信息失败: " + err.Error()})
		return
	}

	// 获取磁盘使用率
	diskInfo, err := disk.Usage("/")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取磁盘信息失败: " + err.Error()})
		return
	}

	// 获取Docker容器资源使用情况
	containerStats, err := getContainersStats()
	if err != nil {
		fmt.Printf("获取容器统计信息失败: %v\n", err)
		// 继续执行，不返回错误
	}

	// 构建响应
	response := gin.H{
		"cpu_percent":     cpuPercent[0],
		"memory_percent":  memInfo.UsedPercent,
		"disk_percent":    diskInfo.UsedPercent,
		"container_stats": containerStats,
		"timestamp":       time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// 获取所有容器的资源使用情况
func getContainersStats() ([]gin.H, error) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	// 获取所有运行中的容器
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: false, // 只获取运行中的容器
	})
	if err != nil {
		return nil, err
	}

	var stats []gin.H
	for _, container := range containers {
		// 获取容器统计信息
		containerStats, err := cli.ContainerStats(context.Background(), container.ID, false)
		if err != nil {
			continue
		}
		defer containerStats.Body.Close()

		// 解析统计信息
		var statsJSON types.StatsJSON
		if err := json.NewDecoder(containerStats.Body).Decode(&statsJSON); err != nil {
			continue
		}

		// 计算CPU使用率
		cpuDelta := float64(statsJSON.CPUStats.CPUUsage.TotalUsage - statsJSON.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(statsJSON.CPUStats.SystemUsage - statsJSON.PreCPUStats.SystemUsage)
		cpuPercent := 0.0
		if systemDelta > 0 && cpuDelta > 0 {
			cpuPercent = (cpuDelta / systemDelta) * float64(len(statsJSON.CPUStats.CPUUsage.PercpuUsage)) * 100.0
		}

		// 计算内存使用率
		memoryUsage := float64(statsJSON.MemoryStats.Usage)
		memoryLimit := float64(statsJSON.MemoryStats.Limit)
		memoryPercent := 0.0
		if memoryLimit > 0 {
			memoryPercent = (memoryUsage / memoryLimit) * 100.0
		}

		// 添加到结果
		stats = append(stats, gin.H{
			"id":             container.ID[:12],
			"name":           strings.TrimPrefix(container.Names[0], "/"),
			"cpu_percent":    cpuPercent,
			"memory_percent": memoryPercent,
			"memory_usage":   memoryUsage,
			"memory_limit":   memoryLimit,
		})
	}

	return stats, nil
}

// 获取Docker版本信息
func getDockerVersion() (string, error) {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// 获取主机名
func getHostname() (string, error) {
	cmd := exec.Command("hostname")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// 获取系统负载
func getSystemLoad() (float64, float64, float64, error) {
	if runtime.GOOS == "windows" {
		// Windows不支持获取负载平均值，返回CPU使用率
		cpuPercent, err := cpu.Percent(0, false)
		if err != nil {
			return 0, 0, 0, err
		}
		return cpuPercent[0] / 100, 0, 0, nil
	}

	// Linux/Unix系统获取负载平均值
	cmd := exec.Command("cat", "/proc/loadavg")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}

	parts := strings.Fields(string(output))
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("无法解析负载平均值")
	}

	load1, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load5, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load15, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

// getSystemEvents 获取系统事件（改为从本地日志文件读取）
func getSystemEvents(c *gin.Context) {
	logs, err := system.GetRecentLogs(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read system logs: " + err.Error()})
		return
	}

	// 转换格式以匹配前端期望
	var eventList []gin.H
	for _, log := range logs {
		eventList = append(eventList, gin.H{
			"id":        log.ID,
			"type":      log.Type,
			"typeClass": log.TypeClass,
			"time":      log.Time,
			"message":   log.Message,
			"timestamp": log.Timestamp,
		})
	}

	c.JSON(http.StatusOK, eventList)
}

type notificationRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func addNotification(c *gin.Context) {
	var req notificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification"})
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	n := &database.Notification{
		Type:    req.Type,
		Message: message,
		Read:    false,
	}
	if err := database.SaveNotification(n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存通知失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

func getNotifications(c *gin.Context) {
	limitStr := c.Query("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	list, err := database.GetNotifications(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取通知失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func deleteNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := database.DeleteNotification(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除通知失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func markNotificationsRead(c *gin.Context) {
	if err := database.MarkAllNotificationsRead(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记通知已读失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
