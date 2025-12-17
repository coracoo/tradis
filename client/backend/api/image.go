package api

import (
	"archive/tar"
	"context"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/gin-gonic/gin"
)

func RegisterImageRoutes(r *gin.RouterGroup) {
	group := r.Group("/images")
	{
		group.GET("", listImages)
		group.DELETE("/:id", removeImage)
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

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	reader, err := cli.ImagePull(context.Background(), imageName, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Access-Control-Allow-Origin", "*")

	// 读取并发送进度信息
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("读取进度失败: %v", err)
			continue
		}

		// 发送原始进度数据
		c.Writer.Write(buf[:n])
		c.Writer.Flush()
	}
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
	_, err = cli.ImageRemove(context.Background(), id, types.ImageRemoveOptions{
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
