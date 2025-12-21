package api

import (
	"archive/tar"
	"context"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/system"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/gin-gonic/gin"
)

var remoteDigestErrorMu sync.Mutex
var remoteDigestErrorLast = make(map[string]time.Time)

func allowRemoteDigestErrorLog(repoTag string) bool {
	repoTag = strings.TrimSpace(repoTag)
	if repoTag == "" {
		return true
	}

	now := time.Now()
	window := 10 * time.Minute

	remoteDigestErrorMu.Lock()
	defer remoteDigestErrorMu.Unlock()

	if last, ok := remoteDigestErrorLast[repoTag]; ok {
		if now.Sub(last) < window {
			return false
		}
	}

	remoteDigestErrorLast[repoTag] = now

	if len(remoteDigestErrorLast) > 1000 {
		cutoff := now.Add(-30 * time.Minute)
		for k, ts := range remoteDigestErrorLast {
			if ts.Before(cutoff) {
				delete(remoteDigestErrorLast, k)
			}
		}
	}

	return true
}

func RegisterImageRoutes(r *gin.RouterGroup) {
	group := r.Group("/images")
	{
		group.GET("", listImages)
		group.DELETE("/:id", removeImage)
		group.GET("/updates", checkImageUpdates)
		group.GET("/updates/status", listStoredImageUpdates)
		group.POST("/updates/clear", clearImageUpdate)
		group.POST("/updates/apply", applyImageUpdates)
		group.POST("/pull", pullImage)
		group.GET("/pull/progress", pullImageProgress)
		group.GET("/proxy", getDockerProxy)
		group.GET("/proxy/history", getDockerProxyHistory)
		group.POST("/proxy", updateDockerProxy)
		group.POST("/tag", tagImage)
		group.GET("/export/:id", exportImage)
		group.POST("/import", importImage)
		group.POST("/prune", pruneImages)
	}
}

// 清理未使用的镜像
func pruneImages(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	report, err := cli.ImagesPrune(context.Background(), filters.Args{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "已清理未使用的镜像",
		"report":  report,
	})
}

// 导入镜像
func importImage(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取上传文件失败: " + err.Error()})
		return
	}

	// 创建临时文件
	tempFile, err := os.CreateTemp("", "docker-image-*.tar")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败: " + err.Error()})
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 保存上传的文件到临时文件
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打开上传文件失败: " + err.Error()})
		return
	}
	defer src.Close()

	if _, err = io.Copy(tempFile, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败: " + err.Error()})
		return
	}

	// 关闭临时文件
	tempFile.Close()

	// 从tar文件中解析镜像信息
	imageInfo, err := extractImageInfoFromTar(tempFile.Name())
	if err != nil {
		log.Printf("从tar文件解析镜像信息失败: %v", err)
	} else {
		log.Printf("从tar文件解析的镜像信息: %+v", imageInfo)
	}

	// 创建Docker客户端
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接Docker失败: " + err.Error()})
		return
	}
	defer cli.Close()

	// 打开临时文件用于导入
	importFile, err := os.Open(tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取临时文件失败: " + err.Error()})
		return
	}
	defer importFile.Close()

	// 导入镜像
	response, err := cli.ImageLoad(context.Background(), importFile, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导入镜像失败: " + err.Error()})
		return
	}
	defer response.Body.Close()

	// 读取响应
	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取导入响应失败: " + err.Error()})
		return
	}

	// 返回结果，优先使用从tar文件解析的信息
	if imageInfo != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":   "镜像导入成功",
			"details":   string(body),
			"imageInfo": imageInfo,
		})
	} else {
		// 如果无法从tar文件解析，则返回基本信息
		c.JSON(http.StatusOK, gin.H{
			"message": "镜像导入成功",
			"details": string(body),
		})
	}
}

// 从tar文件中提取镜像信息
func extractImageInfoFromTar(tarPath string) (map[string]interface{}, error) {
	// 打开tar文件
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, fmt.Errorf("打开tar文件失败: %v", err)
	}
	defer f.Close()

	// 创建tar读取器
	tr := tar.NewReader(f)

	// 查找manifest.json文件
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取tar文件条目失败: %v", err)
		}

		// 检查是否是manifest.json文件
		if filepath.Base(header.Name) == "manifest.json" {
			// 读取manifest.json内容
			manifestData, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("读取manifest.json失败: %v", err)
			}

			// 解析manifest.json
			var manifests []struct {
				Config   string   `json:"Config"`
				RepoTags []string `json:"RepoTags"`
				Layers   []string `json:"Layers"`
			}
			if err := json.Unmarshal(manifestData, &manifests); err != nil {
				return nil, fmt.Errorf("解析manifest.json失败: %v", err)
			}

			// 如果找到了manifest信息
			if len(manifests) > 0 {
				imageID := ""
				if manifests[0].Config != "" {
					// 从Config文件名中提取镜像ID
					imageID = strings.TrimSuffix(manifests[0].Config, ".json")
				}

				repoTags := manifests[0].RepoTags
				if len(repoTags) == 0 {
					repoTags = []string{"<none>:<none>"}
				}

				return map[string]interface{}{
					"id":       imageID,
					"repoTags": repoTags,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("未在tar文件中找到manifest.json或有效的镜像信息")
}

// Docker代理配置结构
type DockerConfig struct {
	Enabled         bool                       `json:"enabled"`
	HTTPProxy       string                     `json:"HTTP Proxy"`
	HTTPSProxy      string                     `json:"HTTPS Proxy"`
	NoProxy         string                     `json:"No Proxy"`
	RegistryMirrors []string                   `json:"registry-mirrors"`
	Registries      map[string]docker.Registry `json:"registries"`
}

// 类型转换函数
func convertRegistryToDocker(r *database.Registry) docker.Registry {
	return docker.Registry{
		Name:     r.Name,
		URL:      r.URL,
		Username: r.Username,
		Password: r.Password,
	}
}

func convertRegistryToDatabase(r docker.Registry) *database.Registry {
	return &database.Registry{
		Name:     r.Name,
		URL:      r.URL,
		Username: r.Username,
		Password: r.Password,
	}
}

// 获取 Docker 代理配置
func getDockerProxy(c *gin.Context) {
	// 不从运行时覆盖用户设置，避免删除后又被覆盖

	// 读取 daemon.json（如果容器有挂载 /etc/docker/daemon.json 则优先）
	daemonConfig, err := docker.GetDaemonConfig()
	if err != nil {
		log.Printf("获取 daemon.json 失败: %v", err)
		daemonConfig = &docker.DaemonConfig{}
	}

	// 从数据库获取代理配置作为备用
	proxy, err := database.GetDockerProxy()
	if err != nil {
		log.Printf("获取数据库代理配置失败: %v", err)
		proxy = &database.DockerProxy{}
	}

	// 获取注册表配置
	dbRegistries, err := database.GetAllRegistries()
	if err != nil {
		log.Printf("获取注册表配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取注册表配置失败"})
		return
	}

	// 转换为 docker.Registry 类型
	registries := make(map[string]docker.Registry)
	for k, v := range dbRegistries {
		registries[k] = convertRegistryToDocker(v)
	}

	// 优先返回数据库中的值，不在 GET 接口中写回数据库，避免覆盖用户禁用设置
	finalMirrors := []string{}
	if proxy.RegistryMirrors != "" {
		var mirrors []string
		_ = json.Unmarshal([]byte(proxy.RegistryMirrors), &mirrors)
		finalMirrors = mirrors
	} else if len(daemonConfig.RegistryMirrors) > 0 {
		finalMirrors = daemonConfig.RegistryMirrors
	}

	config := DockerConfig{
		Enabled: proxy.Enabled,
		HTTPProxy: func() string {
			if proxy.Enabled {
				return proxy.HTTPProxy
			}
			return ""
		}(),
		HTTPSProxy: func() string {
			if proxy.Enabled {
				return proxy.HTTPSProxy
			}
			return ""
		}(),
		NoProxy: func() string {
			if proxy.Enabled {
				return proxy.NoProxy
			}
			return ""
		}(),
		RegistryMirrors: finalMirrors,
		Registries:      registries,
	}

	c.JSON(http.StatusOK, config)
}

// 更新 Docker 代理配置
func updateDockerProxy(c *gin.Context) {
	var config DockerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置格式: " + err.Error()})
		return
	}

	log.Printf("接收到代理配置: enabled=%v, HTTP=%s, HTTPS=%s, NoProxy=%s, Mirrors=%v",
		config.Enabled, config.HTTPProxy, config.HTTPSProxy, config.NoProxy, config.RegistryMirrors)

	// 更新 daemon.json 配置
	daemonConfig := &docker.DaemonConfig{
		RegistryMirrors: config.RegistryMirrors,
	}

	if config.Enabled {
		daemonConfig.Proxies = &docker.ProxyConfig{
			HTTPProxy:  config.HTTPProxy,
			HTTPSProxy: config.HTTPSProxy,
			NoProxy:    config.NoProxy,
		}
	} else {
		daemonConfig.ClearProxies = true
	}

	// 保存到 daemon.json
	if err := docker.UpdateDaemonConfig(daemonConfig); err != nil {
		log.Printf("更新 daemon.json 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}

	// 保存注册表配置到数据库
	for key, registry := range config.Registries {
		dbRegistry := &database.Registry{
			Name:      registry.Name,
			URL:       registry.URL,
			Username:  registry.Username,
			Password:  registry.Password,
			IsDefault: key == "docker.io", // docker.io 为默认注册表
		}

		// 确保 URL 不为空
		if dbRegistry.URL == "" {
			dbRegistry.URL = key
			log.Printf("注册表 URL 为空，使用键作为 URL: %s", key)
		}

		log.Printf("正在保存注册表配置: key=%s, name=%s, url=%s",
			key, dbRegistry.Name, dbRegistry.URL)

		if err := database.SaveRegistry(dbRegistry); err != nil {
			log.Printf("保存注册表失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存注册表配置失败: " + err.Error()})
			return
		}
	}

	// 保存到数据库作为备用配置
	if config.Enabled {
		proxy := &database.DockerProxy{
			Enabled:         true,
			HTTPProxy:       config.HTTPProxy,
			HTTPSProxy:      config.HTTPSProxy,
			NoProxy:         config.NoProxy,
			RegistryMirrors: database.MarshalRegistryMirrors(config.RegistryMirrors),
		}
		if err := database.SaveDockerProxy(proxy); err != nil {
			log.Printf("保存代理配置到数据库失败: %v", err)
		}
		_ = database.SaveProxyHistory(&database.ProxyHistory{
			Enabled:         true,
			HTTPProxy:       config.HTTPProxy,
			HTTPSProxy:      config.HTTPSProxy,
			NoProxy:         config.NoProxy,
			RegistryMirrors: database.MarshalRegistryMirrors(config.RegistryMirrors),
			ChangeType:      "enabled",
		})
	} else {
		// 禁用代理：清除数据库中的代理配置并记录历史
		if err := database.DeleteDockerProxy(); err != nil {
			log.Printf("删除数据库代理配置失败: %v", err)
		}
		_ = database.SaveProxyHistory(&database.ProxyHistory{
			Enabled:         false,
			HTTPProxy:       "",
			HTTPSProxy:      "",
			NoProxy:         "",
			RegistryMirrors: database.MarshalRegistryMirrors(config.RegistryMirrors),
			ChangeType:      "disabled",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Docker配置已更新，请运行以下命令重启 Docker 服务：\nsudo systemctl restart docker",
	})
}

// 获取代理历史记录
func getDockerProxyHistory(c *gin.Context) {
	list, err := database.GetProxyHistory(20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史记录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// 拉取进度监听
func pullImageProgress(c *gin.Context) {
	// 从查询参数获取镜像名称和注册表
	imageName := c.Query("name")
	registry := c.Query("registry")

	if imageName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "镜像名称不能为空"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "不支持流式输出"})
		return
	}

	ctx := c.Request.Context()

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.String(http.StatusInternalServerError, "data: %s\n\n", fmt.Sprintf(`{"error":%q}`, err.Error()))
		flusher.Flush()
		return
	}
	defer cli.Close()

	var options types.ImagePullOptions

	// 如果指定了仓库，使用仓库配置
	if registry != "" {
		registries, getErr := database.GetAllRegistries()
		if getErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取注册表配置失败: " + getErr.Error()})
			return
		}

		if reg, ok := registries[registry]; ok {
			// 如果不是 docker.io，则拼接注册表地址
			if registry != "docker.io" {
				imageName = reg.URL + "/" + imageName
			}

			if reg.Username != "" && reg.Password != "" {
				authConfig := types.AuthConfig{
					Username: reg.Username,
					Password: reg.Password,
				}
				encodedJSON, marshalErr := json.Marshal(authConfig)
				if marshalErr == nil {
					options.RegistryAuth = base64.URLEncoding.EncodeToString(encodedJSON)
				}
			}
		}
	}

	reader, err := cli.ImagePull(ctx, imageName, options)
	if err != nil {
		c.String(http.StatusInternalServerError, "data: %s\n\n", fmt.Sprintf(`{"error":%q}`, err.Error()))
		flusher.Flush()
		return
	}
	defer reader.Close()

	enc := json.NewEncoder(c.Writer)
	push := func(v any) {
		_, _ = c.Writer.Write([]byte("data: "))
		_ = enc.Encode(v)
		_, _ = c.Writer.Write([]byte("\n"))
		flusher.Flush()
	}

	dec := json.NewDecoder(reader)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var payload map[string]any
		if err := dec.Decode(&payload); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Printf("读取进度失败: %v", err)
			push(map[string]any{"error": err.Error()})
			return
		}

		push(payload)
		if _, hasErr := payload["error"]; hasErr {
			return
		}
	}

	push(map[string]any{"type": "done"})
}

// 拉取镜像
func pullImage(c *gin.Context) {
	var req struct {
		Image    string `json:"name" binding:"required"`
		Registry string `json:"registry"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("解析请求参数失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("开始拉取镜像: %s, 注册表: %s", req.Image, req.Registry)

	cli, err := docker.NewDockerClient()
	if err != nil {
		log.Printf("创建 Docker 客户端失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	var options types.ImagePullOptions
	imageName := req.Image

	// 如果指定了仓库，使用仓库配置
	if req.Registry != "" {
		registries, getErr := database.GetAllRegistries()
		if getErr != nil {
			log.Printf("获取注册表配置失败: %v", getErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取注册表配置失败: " + getErr.Error()})
			return
		}

		if registry, ok := registries[req.Registry]; ok {
			// 如果不是 docker.io，则拼接注册表地址
			if req.Registry != "docker.io" {
				imageName = registry.URL + "/" + req.Image
			}
			log.Printf("使用注册表 %s 拉取镜像，完整镜像名: %s", registry.Name, imageName)

			if registry.Username != "" && registry.Password != "" {
				authConfig := types.AuthConfig{
					Username: registry.Username,
					Password: registry.Password,
				}
				encodedJSON, marshalErr := json.Marshal(authConfig)
				if marshalErr == nil {
					options.RegistryAuth = base64.URLEncoding.EncodeToString(encodedJSON)
					log.Printf("使用认证信息拉取镜像")
				}
			}
		} else {
			log.Printf("未找到注册表配置: %s", req.Registry)
		}
	} else {
		log.Printf("使用默认注册表拉取镜像: %s", imageName)
	}

	// 获取 Docker 信息，检查代理设置
	info, err := cli.Info(context.Background())
	if err == nil && (info.HTTPProxy != "" || info.HTTPSProxy != "") {
		log.Printf("Docker 代理设置: HTTP=%s, HTTPS=%s, NoProxy=%s",
			info.HTTPProxy, info.HTTPSProxy, info.NoProxy)
	}

	log.Printf("开始拉取镜像: %s", imageName)
	reader, err := cli.ImagePull(context.Background(), imageName, options)
	if err != nil {
		log.Printf("拉取镜像失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	response, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("读取响应失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取响应失败: " + err.Error()})
		return
	}
	log.Printf("镜像拉取成功: %s", imageName)
	c.JSON(http.StatusOK, gin.H{"message": "镜像拉取成功", "details": string(response)})
}

// 展示镜像
func listImages(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, images)
}

// 删除镜像
func removeImage(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")
	repoTag := c.Query("repoTag")

	target := id
	if repoTag != "" {
		target = repoTag
	}

	_, err = cli.ImageRemove(context.Background(), target, types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "镜像已删除"})
}

// 标签处理
func tagImage(c *gin.Context) {
	var req struct {
		ID   string `json:"id"`
		Repo string `json:"repo"`
		Tag  string `json:"tag"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	newTag := fmt.Sprintf("%s:%s", req.Repo, req.Tag)

	err = cli.ImageTag(context.Background(), req.ID, newTag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("修改标签失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签修改成功"})
}

// 导出镜像
func exportImage(c *gin.Context) {
	imageID := c.Param("id")

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	inspect, _, err := cli.ImageInspectWithRaw(context.Background(), imageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取镜像信息失败: %v", err)})
		return
	}

	fileName := strings.TrimPrefix(imageID, "sha256:")[:12]
	if len(inspect.RepoTags) > 0 {
		fileName = strings.Replace(inspect.RepoTags[0], "/", "_", -1)
		fileName = strings.Replace(fileName, ":", "_", -1)
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.tar", fileName))
	c.Header("Content-Type", "application/x-tar")

	var names []string
	if len(inspect.RepoTags) > 0 {
		names = inspect.RepoTags
	} else {
		names = []string{imageID}
	}

	log.Printf("导出镜像: %v", names)

	reader, err := cli.ImageSave(context.Background(), names)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("导出镜像失败: %v", err)})
		return
	}
	defer reader.Close()

	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("写入响应失败: %v", err)})
		return
	}
}

type imageUpdateInfo struct {
	RepoTag      string `json:"repoTag"`
	LocalDigest  string `json:"localDigest"`
	RemoteDigest string `json:"remoteDigest"`
	Notified     bool   `json:"notified"`
}

type imageUpdateCheckResult struct {
	Updates      []imageUpdateInfo
	TotalImages  int
	FoundUpdates int
	RemoteErrors int
	WriteErrors  int
	Duration     time.Duration
}

var imageUpdateLastRun int64

func formatImageUpdateMessage(repoTags []string) string {
	tags := make([]string, 0, len(repoTags))
	seen := make(map[string]struct{}, len(repoTags))
	for _, t := range repoTags {
		tt := strings.TrimSpace(t)
		if tt == "" {
			continue
		}
		if _, ok := seen[tt]; ok {
			continue
		}
		seen[tt] = struct{}{}
		tags = append(tags, tt)
	}
	if len(tags) == 0 {
		return ""
	}

	sort.Strings(tags)
	display := tags
	if len(display) > 3 {
		display = display[:3]
	}
	suffix := ""
	if len(tags) > 3 {
		suffix = fmt.Sprintf(" 等 %d 个", len(tags))
	}
	return fmt.Sprintf("%s%s 镜像有新版本，去及时查看", strings.Join(display, "、"), suffix)
}

func runImageUpdateCheck(ctx context.Context) (imageUpdateCheckResult, error) {
	start := time.Now()
	result := imageUpdateCheckResult{}

	cli, err := docker.NewDockerClient()
	if err != nil {
		return result, err
	}
	defer cli.Close()

	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return result, err
	}
	result.TotalImages = len(images)

	// 获取已有的更新记录，避免重复轮询同一镜像标签
	existingUpdates, err := database.GetAllImageUpdates()
	if err != nil {
		return result, err
	}
	existingSet := make(map[string]bool, len(existingUpdates))
	for _, u := range existingUpdates {
		if u.RepoTag != "" {
			existingSet[u.RepoTag] = true
		}
	}

	var updates []imageUpdateInfo
	remoteErrors := 0
	writeErrors := 0

	for _, img := range images {
		localDigest := ""
		if len(img.RepoDigests) > 0 {
			parts := strings.SplitN(img.RepoDigests[0], "@", 2)
			if len(parts) == 2 {
				localDigest = parts[1]
			}
		}

		for _, tag := range img.RepoTags {
			if tag == "<none>:<none>" {
				continue
			}
			if localDigest == "" {
				continue
			}
			// 跳过已有记录的镜像标签，直到该记录被删除后再重新轮询
			if existingSet[tag] {
				continue
			}
			remoteDigest, derr := getDockerHubDigest(tag)
			if derr != nil {
				remoteErrors++
				if allowRemoteDigestErrorLog(tag) {
					log.Printf("获取远端镜像摘要失败 repoTag=%s: %v", tag, derr)
					system.LogSimpleEvent("error", fmt.Sprintf("镜像远端摘要获取失败: %s, 错误: %v", tag, derr))
				}
				continue
			}
			if remoteDigest == "" || remoteDigest == localDigest {
				continue
			}
			updates = append(updates, imageUpdateInfo{
				RepoTag:      tag,
				LocalDigest:  localDigest,
				RemoteDigest: remoteDigest,
			})

			if err := database.SaveImageUpdate(&database.ImageUpdate{
				RepoTag:      tag,
				ImageID:      img.ID,
				LocalDigest:  localDigest,
				RemoteDigest: remoteDigest,
				Notified:     false,
			}); err != nil {
				writeErrors++
				log.Printf("写入 image_updates 失败 repoTag=%s: %v", tag, err)
			}
		}
	}

	result.Updates = updates
	result.FoundUpdates = len(updates)
	result.RemoteErrors = remoteErrors
	result.WriteErrors = writeErrors
	result.Duration = time.Since(start)

	return result, nil
}

func StartImageUpdateScheduler() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s, err := settings.GetSettings()
			if err != nil {
				log.Printf("读取镜像更新设置失败: %v", err)
				continue
			}
			interval := s.ImageUpdateIntervalMinutes
			if interval <= 0 {
				interval = 30
			}

			last := time.Unix(atomic.LoadInt64(&imageUpdateLastRun), 0)
			if !last.IsZero() && time.Since(last) < time.Duration(interval)*time.Minute {
				continue
			}

			ctx := context.Background()
			result, cerr := runImageUpdateCheck(ctx)
			now := time.Now()
			atomic.StoreInt64(&imageUpdateLastRun, now.Unix())

			if cerr != nil {
				system.LogSimpleEvent("error", fmt.Sprintf("自动镜像更新检测失败: %v", cerr))
				continue
			}

			if result.WriteErrors > 0 {
				system.LogSimpleEvent("error", fmt.Sprintf(
					"自动镜像更新检测部分失败: 总镜像 %d, 可更新 %d, 写库失败 %d, 远端错误 %d, 耗时 %.1fs",
					result.TotalImages,
					result.FoundUpdates,
					result.WriteErrors,
					result.RemoteErrors,
					result.Duration.Seconds(),
				))
				continue
			}

			unnotified, err := database.GetUnnotifiedImageUpdates()
			if err != nil {
				system.LogSimpleEvent("error", fmt.Sprintf("读取待通知的镜像更新失败: %v", err))
				continue
			}
			if len(unnotified) > 0 {
				repoTags := make([]string, 0, len(unnotified))
				for _, u := range unnotified {
					repoTags = append(repoTags, u.RepoTag)
				}
				msg := formatImageUpdateMessage(repoTags)
				if msg != "" {
					system.LogSimpleEvent("info", msg)
					_ = database.SaveNotification(&database.Notification{Type: "info", Message: msg, Read: false})
				}
				_ = database.MarkImageUpdatesNotifiedByRepoTags(repoTags)
			} else if result.RemoteErrors > 0 {
				system.LogSimpleEvent("warning", fmt.Sprintf(
					"自动镜像更新检测存在远端错误: 总镜像 %d, 远端错误 %d, 耗时 %.1fs",
					result.TotalImages,
					result.RemoteErrors,
					result.Duration.Seconds(),
				))
			}
		}
	}()
}

func checkImageUpdates(c *gin.Context) {
	result, err := runImageUpdateCheck(context.Background())
	if err != nil {
		system.LogSimpleEvent("error", fmt.Sprintf("手动镜像更新检测失败: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result.WriteErrors > 0 {
		system.LogSimpleEvent("error", fmt.Sprintf(
			"手动镜像更新检测部分失败: 总镜像 %d, 可更新 %d, 写库失败 %d, 远端错误 %d, 耗时 %.1fs",
			result.TotalImages,
			result.FoundUpdates,
			result.WriteErrors,
			result.RemoteErrors,
			result.Duration.Seconds(),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("写入镜像更新记录失败: %d 条", result.WriteErrors)})
		return
	}
	if result.FoundUpdates > 0 {
		repoTags := make([]string, 0, len(result.Updates))
		for _, u := range result.Updates {
			repoTags = append(repoTags, u.RepoTag)
		}
		msg := formatImageUpdateMessage(repoTags)
		if msg != "" {
			system.LogSimpleEvent("info", msg)
		}
	} else if result.RemoteErrors > 0 {
		system.LogSimpleEvent("warning", fmt.Sprintf(
			"手动镜像更新检测存在远端错误: 总镜像 %d, 远端错误 %d, 耗时 %.1fs",
			result.TotalImages,
			result.RemoteErrors,
			result.Duration.Seconds(),
		))
	}
	c.JSON(http.StatusOK, gin.H{"updates": result.Updates})
}

func listStoredImageUpdates(c *gin.Context) {
	items, err := database.GetAllImageUpdates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var updates []imageUpdateInfo
	for _, item := range items {
		updates = append(updates, imageUpdateInfo{
			RepoTag:      item.RepoTag,
			LocalDigest:  item.LocalDigest,
			RemoteDigest: item.RemoteDigest,
			Notified:     item.Notified,
		})
	}
	c.JSON(http.StatusOK, gin.H{"updates": updates})
}

func clearImageUpdate(c *gin.Context) {
	var req struct {
		RepoTag string `json:"repoTag"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RepoTag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repoTag is required"})
		return
	}
	if err := database.DeleteImageUpdateByRepoTag(req.RepoTag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func applyImageUpdates(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	items, err := database.GetAllImageUpdates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	usedImageIDs := make(map[string]bool)
	usedImageTags := make(map[string]bool)
	for _, ctr := range containers {
		if ctr.ImageID != "" {
			usedImageIDs[ctr.ImageID] = true
		}
		if ctr.Image != "" {
			usedImageTags[ctr.Image] = true
		}
	}

	total := len(items)
	attempted := 0
	success := 0
	failed := 0
	skippedUsed := 0
	var failedTags []string

	for _, item := range items {
		// 计算镜像是否正在使用中
		used := false
		if item.ImageID != "" && usedImageIDs[item.ImageID] {
			used = true
		}
		if usedImageTags[item.RepoTag] {
			used = true
		}
		if used {
			skippedUsed++
			system.LogSimpleEvent("warning", fmt.Sprintf("跳过正在使用中的镜像: %s", item.RepoTag))
			continue
		}

		attempted++

		// 为非 docker.io 的镜像设置仓库认证
		pullOpts := types.ImagePullOptions{}
		if name, _ := parseImageName(item.RepoTag); name != "" {
			if host := imageHost(name); host != "" {
				if regs, e := database.GetAllRegistries(); e == nil {
					if r := matchRegistry(regs, host); r != nil && (r.Username != "" || r.Password != "") {
						authCfg := types.AuthConfig{
							Username: r.Username,
							Password: r.Password,
						}
						if r.URL != "" {
							authCfg.ServerAddress = r.URL
						}
						if enc, mErr := json.Marshal(authCfg); mErr == nil {
							pullOpts.RegistryAuth = base64.URLEncoding.EncodeToString(enc)
						}
					}
				}
			}
		}

		reader, err := cli.ImagePull(context.Background(), item.RepoTag, pullOpts)
		if err != nil {
			failed++
			failedTags = append(failedTags, item.RepoTag)
			system.LogSimpleEvent("error", fmt.Sprintf("镜像拉取失败: %s, 错误: %v", item.RepoTag, err))
			continue
		}
		_, _ = io.Copy(io.Discard, reader)
		reader.Close()

		success++
		system.LogSimpleEvent("success", fmt.Sprintf("镜像拉取成功: %s", item.RepoTag))
		_ = database.DeleteImageUpdateByRepoTag(item.RepoTag)
	}

	if failed > 0 {
		system.LogSimpleEvent("warning", fmt.Sprintf(
			"批量镜像更新完成: 待更新 %d, 实际尝试 %d, 成功 %d, 失败 %d, 跳过使用中 %d",
			total,
			attempted,
			success,
			failed,
			skippedUsed,
		))
	} else {
		system.LogSimpleEvent("success", fmt.Sprintf(
			"批量镜像更新完成: 待更新 %d, 实际尝试 %d, 成功 %d, 跳过使用中 %d",
			total,
			attempted,
			success,
			skippedUsed,
		))
	}

	c.JSON(http.StatusOK, gin.H{
		"total":       total,
		"attempted":   attempted,
		"success":     success,
		"failed":      failed,
		"skippedUsed": skippedUsed,
		"failedTags":  failedTags,
	})
}

func getDockerHubDigest(repoTag string) (string, error) {
	name, tag := parseImageName(repoTag)
	if name == "" {
		return "", fmt.Errorf("invalid image")
	}

	host := imageHost(name)
	fullRef := ""
	if host == "" {
		path := name
		if !strings.Contains(name, "/") {
			path = "library/" + name
		}
		fullRef = "docker.io/" + path + ":" + tag
	} else {
		fullRef = name + ":" + tag
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		return "", err
	}
	defer cli.Close()

	encoded := ""
	if host != "" {
		if regs, e := database.GetAllRegistries(); e == nil {
			if r := matchRegistry(regs, host); r != nil {
				if r.Username != "" || r.Password != "" {
					auth := registrytypes.AuthConfig{
						Username:      r.Username,
						Password:      r.Password,
						ServerAddress: r.URL,
					}
					if s, ee := registrytypes.EncodeAuthConfig(auth); ee == nil {
						encoded = s
					}
				}
			}
		}
	}

	di, err := cli.DistributionInspect(context.Background(), fullRef, encoded)
	if err != nil {
		return "", err
	}

	d := string(di.Descriptor.Digest)
	if d == "" {
		return "", fmt.Errorf("empty digest")
	}
	return d, nil
}

func imageHost(name string) string {
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

func matchRegistry(regs map[string]*database.Registry, host string) *database.Registry {
	if regs == nil {
		return nil
	}
	if r, ok := regs[host]; ok {
		return r
	}
	if r, ok := regs["https://"+host]; ok {
		return r
	}
	if r, ok := regs["http://"+host]; ok {
		return r
	}
	for k, v := range regs {
		if strings.Contains(k, host) {
			return v
		}
	}
	return nil
}

func parseImageName(repoTag string) (string, string) {
	if repoTag == "" {
		return "", ""
	}
	parts := strings.SplitN(repoTag, ":", 2)
	if len(parts) == 1 {
		return parts[0], "latest"
	}
	if parts[1] == "" {
		return parts[0], "latest"
	}
	return parts[0], parts[1]
}

func isDockerHubImage(name string) bool {
	if name == "" {
		return false
	}
	segments := strings.Split(name, "/")
	if len(segments) == 1 {
		return true
	}
	host := segments[0]
	if strings.Contains(host, ".") || strings.Contains(host, ":") || host == "localhost" {
		return false
	}
	return true
}
