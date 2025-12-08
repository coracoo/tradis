package api

import (
	"context"
	"dockerpanel/backend/pkg/docker"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/gin-gonic/gin"
)

// 应用商城配置
const (
	AppStoreServerURL = "http://localhost:3001" // 应用商城服务器地址
	AppCacheDir       = "./data/apps"           // 应用缓存目录
)

// 应用结构
type App struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Version     string                 `json:"version"`
	Logo        string                 `json:"logo"`
	Author      string                 `json:"author"`
	Website     string                 `json:"website"`
	Tags        []string               `json:"tags"`
	Ports       []Port                 `json:"ports"`
	Volumes     []Volume               `json:"volumes"`
	Environment []EnvVar               `json:"environment"`
	Compose     map[string]interface{} `json:"compose"`
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
	group := r.Group("/api/appstore")
	{
		group.GET("/apps", listApps)
		group.GET("/apps/:id", getApp)
		group.POST("/deploy/:id", deployApp)
		group.GET("/status/:id", getAppStatus)
	}
}

// 获取应用列表
func listApps(c *gin.Context) {
	// 确保缓存目录存在
	if err := os.MkdirAll(AppCacheDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建缓存目录失败"})
		return
	}

	// 从应用商城服务器获取应用列表
	resp, err := http.Get(fmt.Sprintf("%s/api/apps", AppStoreServerURL))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接应用商城服务器失败"})
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
		appPath := filepath.Join(AppCacheDir, fmt.Sprintf("%s.json", app.ID))
		os.WriteFile(appPath, appData, 0644)
	}

	c.JSON(http.StatusOK, apps)
}

// 获取单个应用
func getApp(c *gin.Context) {
	id := c.Param("id")

	// 先尝试从缓存获取
	appPath := filepath.Join(AppCacheDir, fmt.Sprintf("%s.json", id))
	if _, err := os.Stat(appPath); err == nil {
		appData, err := os.ReadFile(appPath)
		if err == nil {
			var app App
			if err := json.Unmarshal(appData, &app); err == nil {
				c.JSON(http.StatusOK, app)
				return
			}
		}
	}

	// 从应用商城服务器获取
	resp, err := http.Get(fmt.Sprintf("%s/api/apps/%s", AppStoreServerURL, id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接应用商城服务器失败"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 读取响应
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取应用详情失败"})
		return
	}

	// 解析应用详情
	var app App
	if err := json.Unmarshal(body, &app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析应用详情失败"})
		return
	}

	// 缓存应用详情
	appData, marshalErr := json.Marshal(app)
	if marshalErr == nil {
		appPath := filepath.Join(AppCacheDir, fmt.Sprintf("%s.json", app.ID))
		os.WriteFile(appPath, appData, 0644)
	}

	c.JSON(http.StatusOK, app)
}

// 部署应用
func deployApp(c *gin.Context) {
	id := c.Param("id")

	// 获取应用详情
	appPath := filepath.Join(AppCacheDir, fmt.Sprintf("%s.json", id))
	appData, err := os.ReadFile(appPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	var app App
	if err := json.Unmarshal(appData, &app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析应用详情失败"})
		return
	}

	// 创建临时compose文件
	composeDir := filepath.Join("deployments", "apps", app.ID)
	if mkErr := os.MkdirAll(composeDir, 0755); mkErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建部署目录失败"})
		return
	}

	// 将compose配置转换为YAML
	composeData, marshalErr := json.Marshal(app.Compose)
	if marshalErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化Compose配置失败"})
		return
	}

	// 保存compose文件
	composeFile := filepath.Join(composeDir, "docker-compose.yaml")
	if writeErr := os.WriteFile(composeFile, composeData, 0644); writeErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存Compose文件失败"})
		return
	}

	// 创建Docker客户端
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败"})
		return
	}
	defer cli.Close()

	// 部署应用
	if err := cli.DeployCompose(context.Background(), composeFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "部署失败",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s 部署成功", app.Name)})
}

// 获取应用状态
func getAppStatus(c *gin.Context) {
	id := c.Param("id")

	// 创建Docker客户端
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败"})
		return
	}
	defer cli.Close()

	// 查询与应用相关的容器
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: "com.docker.compose.project=" + id,
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
		"total":    total,
		"running":  running,
		"deployed": total > 0,
		"healthy":  total > 0 && running == total,
	})
}
