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

	// 同步宿主机 Docker 代理与镜像加速配置到数据库（仅在数据库为空时执行）
	func() {
		db := database.GetDB()
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM docker_proxy").Scan(&count); err != nil {
			log.Printf("读取 docker_proxy 计数失败: %v", err)
			return
		}
		if count > 0 {
			log.Printf("检测到已有 docker_proxy 配置，跳过宿主机同步")
			return
		}
		// 优先读取 daemon.json，如果读取失败或为空，再回退到 Docker 运行时 info（支持 systemd 环境代理）
		if cfg, err := docker.GetDaemonConfig(); err == nil && (cfg.Proxies != nil || len(cfg.RegistryMirrors) > 0) {
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
			return
		}
		// 回退：读取 Docker 运行时信息
		if cli, err := docker.NewDockerClient(); err == nil {
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
					log.Printf("保存宿主机 Docker 运行时代理配置失败: %v", err)
				} else {
					log.Printf("已同步宿主机 Docker 运行时代理与镜像加速配置")
				}
			}
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
	// 1.1 兼容旧路径：上传的导航图标
	r.Static("/uploads/icons", filepath.Join(settings.GetDataDir(), "icons"))
	// 1.2 新路径：统一静态图片目录 /data/pic
	r.Static("/data/pic", filepath.Join(settings.GetDataDir(), "pic"))

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

	port := strings.TrimSpace(os.Getenv("BACKEND_PORT"))
	log.Printf("[DEBUG] Env BACKEND_PORT: '%s'", port)

	if port == "" {
		port = "8080"
		log.Printf("[DEBUG] Using default port: 8080")
	}

	bind := os.Getenv("BACKEND_BIND")
	if bind == "" {
		bind = fmt.Sprintf(":%s", port)
	}

	log.Printf("[DEBUG] Starting server on %s", bind)
	r.Run(bind)
}
