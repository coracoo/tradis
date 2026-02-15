package main

import (
	"bytes"
	"dockerpanel/server/backend/handlers"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
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
		handlers.RespondError(c, http.StatusForbidden, "该接口仅允许指定 IP 访问", fmt.Errorf("ip=%s", ipStr))
		c.Abort()
	}
}

func ipAllowlistMiddleware(store *handlers.IPAllowlist) gin.HandlerFunc {
	return func(c *gin.Context) {
		ipStr := strings.TrimSpace(c.ClientIP())
		ip := net.ParseIP(ipStr)
		if store != nil && store.AllowsIP(ip) {
			c.Next()
			return
		}
		handlers.RespondError(c, http.StatusForbidden, "该接口仅允许指定 IP 访问", fmt.Errorf("ip=%s", ipStr))
		c.Abort()
	}
}

type ipFixedWindowLimiter struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	data   map[string]struct {
		start time.Time
		count int
	}
}

func newIPFixedWindowLimiter(limit int, window time.Duration) *ipFixedWindowLimiter {
	return &ipFixedWindowLimiter{
		window: window,
		limit:  limit,
		data: make(map[string]struct {
			start time.Time
			count int
		}),
	}
}

func (l *ipFixedWindowLimiter) Allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	it, ok := l.data[ip]
	if !ok || now.Sub(it.start) >= l.window {
		l.data[ip] = struct {
			start time.Time
			count int
		}{start: now, count: 1}
		return true
	}
	if it.count >= l.limit {
		return false
	}
	it.count++
	l.data[ip] = it
	return true
}

func rateLimitByIP(l *ipFixedWindowLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := strings.TrimSpace(c.ClientIP())
		if ip == "" {
			ip = "unknown"
		}
		if !l.Allow(ip) {
			handlers.RespondError(c, http.StatusTooManyRequests, "请求过于频繁，请稍后重试", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

type responseCacheEntry struct {
	expires     time.Time
	status      int
	contentType string
	body        []byte
}

type responseCache struct {
	mu    sync.Mutex
	items map[string]responseCacheEntry
}

func newResponseCache() *responseCache {
	return &responseCache{items: make(map[string]responseCacheEntry)}
}

type captureWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *captureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	_, _ = w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	_, _ = w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func cacheGETJSON(cache *responseCache, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		key := c.Request.Method + " " + c.Request.URL.Path + "?" + c.Request.URL.RawQuery
		now := time.Now()

		cache.mu.Lock()
		ce, ok := cache.items[key]
		cache.mu.Unlock()

		if ok && now.Before(ce.expires) && ce.status > 0 {
			if ce.contentType != "" {
				c.Header("Content-Type", ce.contentType)
			}
			c.Status(ce.status)
			_, _ = c.Writer.Write(ce.body)
			c.Abort()
			return
		}

		cw := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = cw
		c.Next()

		status := cw.status
		if status == 0 {
			status = c.Writer.Status()
		}
		if status != 200 {
			return
		}
		body := cw.body.Bytes()
		if len(body) == 0 {
			return
		}

		contentType := c.Writer.Header().Get("Content-Type")
		cache.mu.Lock()
		cache.items[key] = responseCacheEntry{
			expires:     time.Now().Add(ttl),
			status:      status,
			contentType: contentType,
			body:        append([]byte(nil), body...),
		}
		cache.mu.Unlock()
	}
}

type accessLogConfig struct {
	Enabled        bool
	LogDir         string
	BaseName       string
	MaxSizeMB      int
	MaxBackups     int
	MaxAgeDays     int
	Compress       bool
	MinStatus      int
	MinLatencyMS   int64
	IgnorePaths    map[string]struct{}
	IgnoreRoutes   map[string]struct{}
	IgnorePrefixes []string
}

func parseEnvBool(raw string, defaultVal bool) bool {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return defaultVal
	}
	switch s {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultVal
	}
}

func parseEnvInt(raw string, defaultVal int) int {
	s := strings.TrimSpace(raw)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func parseEnvInt64(raw string, defaultVal int64) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func parseCommaSet(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		out[s] = struct{}{}
	}
	return out
}

func parseCommaList(raw string) []string {
	items := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		items = append(items, s)
	}
	return items
}

func loadAccessLogConfig() accessLogConfig {
	cfg := accessLogConfig{
		Enabled:      parseEnvBool(os.Getenv("ACCESS_LOG_ENABLED"), true),
		LogDir:       strings.TrimSpace(os.Getenv("ACCESS_LOG_DIR")),
		BaseName:     strings.TrimSpace(os.Getenv("ACCESS_LOG_BASENAME")),
		MaxSizeMB:    parseEnvInt(os.Getenv("ACCESS_LOG_MAX_SIZE_MB"), 50),
		MaxBackups:   parseEnvInt(os.Getenv("ACCESS_LOG_MAX_BACKUPS"), 10),
		MaxAgeDays:   parseEnvInt(os.Getenv("ACCESS_LOG_MAX_AGE_DAYS"), 14),
		Compress:     parseEnvBool(os.Getenv("ACCESS_LOG_COMPRESS"), true),
		MinStatus:    parseEnvInt(os.Getenv("ACCESS_LOG_MIN_STATUS"), 0),
		MinLatencyMS: parseEnvInt64(os.Getenv("ACCESS_LOG_MIN_LATENCY_MS"), 0),
	}
	if cfg.LogDir == "" {
		if st, err := os.Stat(filepath.Join("backend", "data", "logs")); err == nil && st.IsDir() {
			cfg.LogDir = filepath.Join("backend", "data", "logs")
		} else {
			cfg.LogDir = filepath.Join("data", "logs")
		}
	}
	if cfg.BaseName == "" {
		cfg.BaseName = "api_access"
	}

	ignorePathsRaw := strings.TrimSpace(os.Getenv("ACCESS_LOG_IGNORE_PATHS"))
	if ignorePathsRaw == "" {
		ignorePathsRaw = "/api/version"
	}
	cfg.IgnorePaths = parseCommaSet(ignorePathsRaw)
	cfg.IgnoreRoutes = parseCommaSet(strings.TrimSpace(os.Getenv("ACCESS_LOG_IGNORE_ROUTES")))

	ignorePrefixesRaw := strings.TrimSpace(os.Getenv("ACCESS_LOG_IGNORE_PATH_PREFIXES"))
	if ignorePrefixesRaw == "" {
		ignorePrefixesRaw = "/uploads/"
	}
	cfg.IgnorePrefixes = parseCommaList(ignorePrefixesRaw)

	if cfg.MaxSizeMB < 1 {
		cfg.MaxSizeMB = 1
	}
	if cfg.MaxBackups < 0 {
		cfg.MaxBackups = 0
	}
	if cfg.MaxAgeDays < 1 {
		cfg.MaxAgeDays = 1
	}
	if cfg.MinStatus < 0 {
		cfg.MinStatus = 0
	}
	if cfg.MinLatencyMS < 0 {
		cfg.MinLatencyMS = 0
	}
	return cfg
}

type dailyRollingWriter struct {
	mu          sync.Mutex
	cfg         accessLogConfig
	currentDate string
	lj          *lumberjack.Logger
}

func newDailyRollingWriter(cfg accessLogConfig) (*dailyRollingWriter, error) {
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, err
	}
	w := &dailyRollingWriter{cfg: cfg}
	if err := w.switchDateLocked(time.Now()); err != nil {
		return nil, err
	}
	_ = w.cleanupLocked(time.Now())
	return w, nil
}

func (w *dailyRollingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lj != nil {
		return w.lj.Close()
	}
	return nil
}

func (w *dailyRollingWriter) Write(p []byte) (int, error) {
	now := time.Now()
	w.mu.Lock()
	if err := w.maybeSwitchDateLocked(now); err != nil {
		w.mu.Unlock()
		return 0, err
	}
	lj := w.lj
	w.mu.Unlock()
	if lj == nil {
		return 0, io.ErrClosedPipe
	}
	return lj.Write(p)
}

func (w *dailyRollingWriter) maybeSwitchDateLocked(now time.Time) error {
	date := now.Format("2006-01-02")
	if date == w.currentDate && w.lj != nil {
		return nil
	}
	return w.switchDateLocked(now)
}

func (w *dailyRollingWriter) switchDateLocked(now time.Time) error {
	if w.lj != nil {
		_ = w.lj.Close()
		w.lj = nil
	}

	date := now.Format("2006-01-02")
	filename := filepath.Join(w.cfg.LogDir, fmt.Sprintf("%s-%s.log", w.cfg.BaseName, date))
	if f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		_ = f.Close()
	}
	w.lj = &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    w.cfg.MaxSizeMB,
		MaxBackups: w.cfg.MaxBackups,
		MaxAge:     w.cfg.MaxAgeDays,
		Compress:   w.cfg.Compress,
	}
	w.currentDate = date

	_ = w.updateSymlinkLocked(filename)
	_ = w.cleanupLocked(now)
	return nil
}

func (w *dailyRollingWriter) updateSymlinkLocked(todayFilename string) error {
	linkPath := filepath.Join(w.cfg.LogDir, w.cfg.BaseName+".log")
	if fi, err := os.Lstat(linkPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(linkPath)
		} else if fi.Mode().IsRegular() {
			migrated := filepath.Join(w.cfg.LogDir, fmt.Sprintf("%s-migrated-%s.log", w.cfg.BaseName, time.Now().Format("20060102-150405")))
			_ = os.Rename(linkPath, migrated)
		} else {
			return fmt.Errorf("无法处理旧日志文件: %s", linkPath)
		}
	}
	target := filepath.Base(todayFilename)
	return os.Symlink(target, linkPath)
}

func (w *dailyRollingWriter) cleanupLocked(now time.Time) error {
	cutoff := now.Add(-time.Duration(w.cfg.MaxAgeDays) * 24 * time.Hour)
	entries, err := os.ReadDir(w.cfg.LogDir)
	if err != nil {
		return err
	}

	prefix := w.cfg.BaseName + "-"
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if !strings.Contains(name, ".log") {
			continue
		}
		full := filepath.Join(w.cfg.LogDir, name)
		fi, err := ent.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			_ = os.Remove(full)
		}
	}
	return nil
}

func shouldSkipAccessLog(cfg accessLogConfig, c *gin.Context, status int, latencyMS int64) bool {
	if !cfg.Enabled {
		return true
	}
	if c == nil || c.Request == nil {
		return true
	}
	if cfg.MinStatus > 0 && status < cfg.MinStatus {
		return true
	}
	if cfg.MinLatencyMS > 0 && latencyMS < cfg.MinLatencyMS {
		return true
	}
	if strings.EqualFold(c.Request.Method, "OPTIONS") {
		return true
	}

	path := c.Request.URL.Path
	if _, ok := cfg.IgnorePaths[path]; ok {
		return true
	}
	route := c.FullPath()
	if route != "" {
		if _, ok := cfg.IgnoreRoutes[route]; ok {
			return true
		}
	}
	for _, p := range cfg.IgnorePrefixes {
		if p != "" && strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func main() {
	_ = os.MkdirAll("data/uploads", 0755)

	db, err := gorm.Open(sqlite.Open("data/templates.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	db.AutoMigrate(&handlers.Template{}, &handlers.ServerKV{}, &handlers.ApplicationRequest{})

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

	accessCfg := loadAccessLogConfig()
	if accessCfg.Enabled {
		accessWriter, aerr := newDailyRollingWriter(accessCfg)
		if aerr != nil {
			log.Printf("[API Access Log] 初始化失败 dir=%s err=%v", accessCfg.LogDir, aerr)
		} else {
			defer func() { _ = accessWriter.Close() }()
			r.Use(func(c *gin.Context) {
				start := time.Now()
				c.Next()

				status := c.Writer.Status()
				latencyMS := time.Since(start).Milliseconds()
				if shouldSkipAccessLog(accessCfg, c, status, latencyMS) {
					return
				}

				ua := strings.TrimSpace(c.Request.UserAgent())
				if len(ua) > 512 {
					ua = ua[:512]
				}
				item := map[string]any{
					"ts":         time.Now().Format(time.RFC3339Nano),
					"ip":         strings.TrimSpace(c.ClientIP()),
					"method":     c.Request.Method,
					"path":       c.Request.URL.Path,
					"route":      c.FullPath(),
					"status":     status,
					"latency_ms": latencyMS,
					"ua":         ua,
				}
				if b, err := json.Marshal(item); err == nil {
					if _, werr := accessWriter.Write(append(b, '\n')); werr != nil {
						log.Printf("[API Access Log] 写入失败 err=%v", werr)
					}
				}
			})
			log.Printf("[API Access Log] 已启用 dir=%s current=%s", accessCfg.LogDir, filepath.Join(accessCfg.LogDir, accessCfg.BaseName+".log"))
		}
	} else {
		log.Printf("[API Access Log] 已禁用（ACCESS_LOG_ENABLED=0）")
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))

	r.Static("/uploads", "./data/uploads")

	adminAllowlistEnv := strings.TrimSpace(os.Getenv("ADMIN_ALLOWLIST"))
	if adminAllowlistEnv == "" {
		adminAllowlistEnv = "127.0.0.1,::1"
	}
	adminAllowlist, notes := handlers.NewIPAllowlist(adminAllowlistEnv)
	for _, n := range notes {
		log.Printf("[IP Allowlist] %s", n)
	}
	if v, ok, err := handlers.GetKV(db, handlers.KVAdminAllowlist); err == nil && ok && strings.TrimSpace(v) != "" {
		for _, n := range adminAllowlist.Set(v) {
			log.Printf("[IP Allowlist] %s", n)
		}
	} else if err != nil {
		log.Printf("[IP Allowlist] 读取数据库配置失败: %v", err)
	}
	log.Printf("[IP Allowlist] 写接口允许来源: %s", adminAllowlist.Raw())

	mcpAllowlistEnv := strings.TrimSpace(os.Getenv("MCP_ALLOWLIST"))
	if mcpAllowlistEnv == "" {
		mcpAllowlistEnv = adminAllowlistEnv
	}
	mcpAllowlist, mcpNotes := handlers.NewIPAllowlist(mcpAllowlistEnv)
	for _, n := range mcpNotes {
		log.Printf("[MCP Allowlist] %s", n)
	}
	if v, ok, err := handlers.GetKV(db, handlers.KVMCPAllowlist); err == nil && ok && strings.TrimSpace(v) != "" {
		for _, n := range mcpAllowlist.Set(v) {
			log.Printf("[MCP Allowlist] %s", n)
		}
	} else if err != nil {
		log.Printf("[MCP Allowlist] 读取数据库配置失败: %v", err)
	}
	log.Printf("[MCP Allowlist] 当前允许来源: %s", mcpAllowlist.Raw())

	mcpToken := strings.TrimSpace(os.Getenv("MCP_TOKEN"))
	if v, ok, err := handlers.GetKV(db, handlers.KVMCPToken); err == nil && ok && strings.TrimSpace(v) != "" {
		mcpToken = strings.TrimSpace(v)
	} else if err != nil {
		log.Printf("[MCP Token] 读取数据库配置失败: %v", err)
	}

	api := r.Group("/api")
	templatesLimiter := newIPFixedWindowLimiter(240, time.Minute)
	templatesCache := newResponseCache()
	api.GET("/templates", rateLimitByIP(templatesLimiter), cacheGETJSON(templatesCache, 5*time.Second), handlers.ListTemplates(db))
	api.GET("/templates/:id", rateLimitByIP(templatesLimiter), cacheGETJSON(templatesCache, 5*time.Second), handlers.GetTemplate(db))
	api.POST("/templates/:id/deploy", handlers.IncrementTemplateDeploymentCount(db))
	api.GET("/templates/:id/vars", rateLimitByIP(templatesLimiter), cacheGETJSON(templatesCache, 5*time.Second), handlers.GetTemplateVars(db))
	api.POST("/applications", handlers.CreateApplicationRequest(db))
	api.GET("/version", handlers.GetServerVersion(db))

	admin := api.Group("")
	admin.Use(ipAllowlistMiddleware(adminAllowlist))
	admin.POST("/templates", handlers.CreateTemplate(db))
	admin.PUT("/templates/:id", handlers.UpdateTemplate(db))
	admin.POST("/templates/parse-vars", handlers.ParseTemplateVars())
	admin.POST("/templates/:id/enable", handlers.EnableTemplate(db))
	admin.POST("/templates/:id/disable", handlers.DisableTemplate(db))
	admin.DELETE("/templates/:id", handlers.DeleteTemplate(db))
	admin.POST("/templates/sync", handlers.SyncTemplatesToGithubHandler(db))
	admin.POST("/upload", handlers.UploadFile)
	admin.PUT("/version", handlers.UpdateServerVersion(db))

	admin.GET("/admin/allowlist", handlers.GetAllowlistHandler(db, handlers.KVAdminAllowlist, adminAllowlist, adminAllowlistEnv))
	admin.PUT("/admin/allowlist", handlers.UpdateAllowlistHandler(db, handlers.KVAdminAllowlist, adminAllowlist))
	admin.GET("/admin/mcp-allowlist", handlers.GetAllowlistHandler(db, handlers.KVMCPAllowlist, mcpAllowlist, mcpAllowlistEnv))
	admin.PUT("/admin/mcp-allowlist", handlers.UpdateAllowlistHandler(db, handlers.KVMCPAllowlist, mcpAllowlist))
	admin.GET("/admin/mcp-token", handlers.GetMCPTokenHandler(db))
	admin.PUT("/admin/mcp-token", handlers.UpdateMCPTokenHandler(db, func(next string) { mcpToken = strings.TrimSpace(next) }))

	mcp := api.Group("/mcp")
	mcp.Use(ipAllowlistMiddleware(mcpAllowlist))
	mcp.Use(func(c *gin.Context) {
		token := strings.TrimSpace(mcpToken)
		if token == "" {
			c.Next()
			return
		}
		got := strings.TrimSpace(c.GetHeader("X-MCP-Token"))
		if got == token {
			c.Next()
			return
		}
		handlers.RespondError(c, http.StatusForbidden, "MCP token 无效", fmt.Errorf("ip=%s", strings.TrimSpace(c.ClientIP())))
		c.Abort()
	})
	mcp.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
		c.Next()
	})
	mcp.POST("/templates/import", handlers.ImportTemplatesMCP(db))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}
	r.Run(":" + port)
}
