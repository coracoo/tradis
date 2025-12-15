// cmd/main.go
package main

import (
	"context"
	"dockerpanel/backend/api"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/system"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 设置数据目录
	dataDir := settings.GetDataDir()
	log.Printf("数据目录: %s", dataDir)

	// 初始化数据库
	log.Printf("正在初始化数据库...")
	dbPath := filepath.Join(dataDir, "data.db") // 指定数据库文件路径
	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	log.Println("数据库初始化成功")
	defer database.Close()

	// 初始化全局设置默认值
	if err := settings.InitSettingsTable(); err != nil {
		log.Printf("初始化全局设置失败: %v", err)
	}

	// 同步宿主机 Docker 代理与镜像加速配置到数据库
	func() {
		// 优先通过 Docker API 读取运行时信息
		cli, err := docker.NewDockerClient()
		if err == nil {
			defer cli.Close()
			if info, err := cli.Info(context.Background()); err == nil {
				mirrors := []string{}
				if info.RegistryConfig != nil {
					mirrors = info.RegistryConfig.Mirrors
				}
				proxy := &database.DockerProxy{
					Enabled:         (info.HTTPProxy != "" || info.HTTPSProxy != ""),
					HTTPProxy:       info.HTTPProxy,
					HTTPSProxy:      info.HTTPSProxy,
					NoProxy:         info.NoProxy,
					RegistryMirrors: database.MarshalRegistryMirrors(mirrors),
				}
				if err := database.SaveDockerProxy(proxy); err != nil {
					log.Printf("保存宿主机 Docker 代理配置失败: %v", err)
				} else {
					log.Printf("已同步宿主机 Docker 运行时代理与镜像加速配置")
				}
				return
			}
		}
		// 回退：读取 daemon.json
		cfg, err := docker.GetDaemonConfig()
		if err != nil {
			log.Printf("读取宿主机 Docker 配置失败: %v", err)
			return
		}
		proxy := &database.DockerProxy{
			Enabled: cfg.Proxies != nil,
			HTTPProxy: func() string {
				if cfg.Proxies != nil {
					return cfg.Proxies.HTTPProxy
				}
				return ""
			}(),
			HTTPSProxy: func() string {
				if cfg.Proxies != nil {
					return cfg.Proxies.HTTPSProxy
				}
				return ""
			}(),
			NoProxy: func() string {
				if cfg.Proxies != nil {
					return cfg.Proxies.NoProxy
				}
				return ""
			}(),
			RegistryMirrors: database.MarshalRegistryMirrors(cfg.RegistryMirrors),
		}
		if err := database.SaveDockerProxy(proxy); err != nil {
			log.Printf("保存宿主机 Docker 代理配置失败: %v", err)
		} else {
			log.Printf("已同步宿主机 Docker 代理与镜像加速配置 (daemon.json)")
		}
	}()

	// 启动 Docker 事件日志记录器
	system.StartEventLogger()

	r := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	r.Use(cors.New(config))

	// Register API routes
	api.RegisterAuthRoutes(r) // 注册认证路由

	// 创建一个 API 组，用于需要认证的路由
	// 注意：WebSocket 连接可能需要特殊的认证处理（Query Param），这里暂时通过 Header 认证
	// 如果 WebSocket 客户端无法发送 Header，可能需要将 RegisterTerminalRoutes 移出 protected 组，
	// 并在内部实现 Token 验证 (例如通过 Query String)
	protected := r.Group("/api")
	protected.Use(api.AuthMiddleware())

	api.RegisterContainerRoutes(protected)
	api.RegisterImageRoutes(protected)
	api.RegisterVolumeRoutes(protected)
	api.RegisterNetworkRoutes(protected)
	api.RegisterComposeRoutes(protected)
	api.RegisterImageRegistryRoutes(protected)
	api.RegisterSystemRoutes(protected)
	api.RegisterNavigationRoutes(protected) // 注册导航路由
	api.RegisterSettingsRoutes(protected)   // 注册设置路由
	api.RegisterPortRoutes(protected)       // 注册端口管理路由

	// 注册应用商店路由
	// 公开路由 (列表、详情)
	api.RegisterAppStoreRoutes(r)
	// 保护路由 (部署、状态)
	api.RegisterAppStoreProtectedRoutes(protected)

	// 注册容器终端与命令执行路由（WebSocket + CLI）
	api.RegisterTerminalRoutes(protected)

	// 启动容器自动发现服务
	go system.ProcessContainerDiscovery()
	// 启动容器事件监听
	go system.WatchContainerEvents()

	// 静态文件服务
	// 1. 静态资源 (assets) - 对应 dist/assets 目录
	r.Static("/assets", "./dist/assets")

	// 2. 根路径路由
	r.GET("/", func(c *gin.Context) {
		c.File("./dist/index.html")
	})

	// 3. SPA 回退逻辑 (NoRoute)
	// 处理所有未匹配的路由，支持 History 模式路由和根目录静态文件
	r.NoRoute(func(c *gin.Context) {
		// 避免 API 请求被错误返回 index.html
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(404, gin.H{"error": "API not found"})
			return
		}

		// 检查文件是否存在于 dist 根目录中 (例如 favicon.ico, vite.svg)
		// 注意：这里需要防止目录遍历攻击，但 filepath.Join 和 Clean 通常能处理
		path := filepath.Join("./dist", c.Request.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			c.File(path)
			return
		}

		// 默认返回 index.html (SPA 支持)
		c.File("./dist/index.html")
	})

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}
	bind := os.Getenv("BACKEND_BIND")
	if bind == "" {
		bind = fmt.Sprintf(":%s", port)
	}
	r.Run(bind)
}
