// cmd/main.go
package main

import (
	"dockerpanel/backend/api"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/system"
	"log"
	"path/filepath"

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

	// 使用特定前缀处理静态文件
	r.Static("/static", "./dist")

	// 添加根路由重定向到静态文件
	r.GET("/", func(c *gin.Context) {
		c.Redirect(301, "/static/index.html")
	})

	r.Run(":8080")
}
