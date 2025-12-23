package main

import (
	"dockerpanel/server/backend/handlers"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// parseIPAllowlist 将逗号分隔的 IP/CIDR 字符串解析为可匹配的网段列表
func parseIPAllowlist(raw string) ([]*net.IPNet, []string) {
	notes := make([]string, 0)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, notes
	}

	items := strings.Split(raw, ",")
	out := make([]*net.IPNet, 0, len(items))
	for _, item := range items {
		s := strings.TrimSpace(item)
		if s == "" {
			continue
		}
		if strings.Contains(s, "/") {
			_, n, err := net.ParseCIDR(s)
			if err != nil {
				notes = append(notes, fmt.Sprintf("忽略无效 CIDR: %s", s))
				continue
			}
			out = append(out, n)
			continue
		}

		ip := net.ParseIP(s)
		if ip == nil {
			notes = append(notes, fmt.Sprintf("忽略无效 IP: %s", s))
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			out = append(out, &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)})
		} else {
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)})
		}
	}
	return out, notes
}

// ipInAllowlist 判断 clientIP 是否命中任一允许网段
func ipInAllowlist(clientIP net.IP, allow []*net.IPNet) bool {
	if clientIP == nil || len(allow) == 0 {
		return false
	}
	for _, n := range allow {
		if n != nil && n.Contains(clientIP) {
			return true
		}
	}
	return false
}

// adminIPAllowlistMiddleware 限制写接口仅允许指定 IP/CIDR 访问
func adminIPAllowlistMiddleware(allow []*net.IPNet) gin.HandlerFunc {
	return func(c *gin.Context) {
		ipStr := strings.TrimSpace(c.ClientIP())
		ip := net.ParseIP(ipStr)
		if ipInAllowlist(ip, allow) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(403, gin.H{
			"error": "该接口仅允许指定 IP 访问",
			"ip":    ipStr,
		})
	}
}

func main() {
	_ = os.MkdirAll("data/uploads", 0755)

	db, err := gorm.Open(sqlite.Open("data/templates.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	db.AutoMigrate(&handlers.Template{})

	r := gin.Default()

	trustedProxiesRaw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if trustedProxiesRaw == "" {
		trustedProxiesRaw = "127.0.0.1,::1,172.16.0.0/12"
	}
	trustedProxies := make([]string, 0)
	for _, s := range strings.Split(trustedProxiesRaw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			trustedProxies = append(trustedProxies, s)
		}
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		log.Printf("[Trusted Proxies] 设置失败: %v", err)
	} else {
		log.Printf("[Trusted Proxies] 当前配置: %s", trustedProxiesRaw)
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))

	r.Static("/uploads", "./data/uploads")

	allowlistRaw := strings.TrimSpace(os.Getenv("ADMIN_ALLOWLIST"))
	if allowlistRaw == "" {
		allowlistRaw = "127.0.0.1,::1"
	}
	allowlist, notes := parseIPAllowlist(allowlistRaw)
	for _, n := range notes {
		log.Printf("[IP Allowlist] %s", n)
	}
	log.Printf("[IP Allowlist] 写接口允许来源: %s", allowlistRaw)

	api := r.Group("/api")
	api.GET("/templates", handlers.ListTemplates(db))
	api.GET("/templates/:id", handlers.GetTemplate(db))

	admin := api.Group("")
	admin.Use(adminIPAllowlistMiddleware(allowlist))
	admin.POST("/templates", handlers.CreateTemplate(db))
	admin.PUT("/templates/:id", handlers.UpdateTemplate(db))
	admin.DELETE("/templates/:id", handlers.DeleteTemplate(db))
	admin.POST("/upload", handlers.UploadFile)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}
	r.Run(":" + port)
}
