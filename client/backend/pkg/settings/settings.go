package settings

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultAppStoreServerURL = "https://template.cgakki.top:33333"

var (
	appStoreRedactMu     sync.Mutex
	appStoreRedactCached struct {
		url      string
		host     string
		expires  time.Time
		initOnce bool
	}
)

type Settings struct {
	LanUrl                     string `json:"lanUrl"`
	WanUrl                     string `json:"wanUrl"`
	AppStoreServerUrl          string `json:"appStoreServerUrl"`
	AllocPortStart             int    `json:"allocPortStart"`
	AllocPortEnd               int    `json:"allocPortEnd"`
	ImageUpdateIntervalMinutes int    `json:"imageUpdateIntervalMinutes"`
}

// IsDebugEnabled 判断是否启用调试日志（通过环境变量控制）。
func IsDebugEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("DEBUG")))
	if v == "1" || v == "true" || v == "yes" || v == "on" {
		return true
	}
	lv := strings.TrimSpace(strings.ToLower(os.Getenv("LOG_LEVEL")))
	return lv == "debug"
}

// RedactAppStoreURL 对文本中的应用商店服务地址进行脱敏处理。
func RedactAppStoreURL(text string) string {
	if text == "" {
		return ""
	}

	candidates := []string{
		DefaultAppStoreServerURL,
		"template.cgakki.top:33333",
	}

	u, h := getCachedAppStoreURLForRedaction()
	if u != "" {
		candidates = append(candidates, u)
	}
	if h != "" {
		candidates = append(candidates, h)
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		text = strings.ReplaceAll(text, c, "<APPSTORE_API>")
	}
	return text
}

func getCachedAppStoreURLForRedaction() (string, string) {
	now := time.Now()
	appStoreRedactMu.Lock()
	defer appStoreRedactMu.Unlock()

	if appStoreRedactCached.initOnce && now.Before(appStoreRedactCached.expires) {
		return appStoreRedactCached.url, appStoreRedactCached.host
	}

	var raw string
	if s, err := GetSettings(); err == nil {
		raw = strings.TrimSpace(s.AppStoreServerUrl)
	}

	host := ""
	if raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			host = u.Host
		}
	}

	appStoreRedactCached.url = raw
	appStoreRedactCached.host = host
	appStoreRedactCached.expires = now.Add(30 * time.Second)
	appStoreRedactCached.initOnce = true
	return appStoreRedactCached.url, appStoreRedactCached.host
}

// InitSettingsTable 初始化设置表
func InitSettingsTable() error {
	db := database.GetDB()
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS global_settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	if err != nil {
		return err
	}

	// Helper to insert default if not exists
	insertDefault := func(key, value string) error {
		var val string
		err := db.QueryRow("SELECT value FROM global_settings WHERE key = ?", key).Scan(&val)
		if err == sql.ErrNoRows {
			_, err = db.Exec("INSERT INTO global_settings (key, value) VALUES (?, ?)", key, value)
			return err
		}
		return nil
	}

	if err := insertDefault("lan_url", "http://localhost"); err != nil {
		return err
	}
	if err := insertDefault("wan_url", ""); err != nil {
		return err
	}
	if err := insertDefault("appstore_server_url", DefaultAppStoreServerURL); err != nil {
		return err
	}
	if err := insertDefault("alloc_port_start", "20000"); err != nil {
		return err
	}
	if err := insertDefault("alloc_port_end", "30000"); err != nil {
		return err
	}
	if err := insertDefault("image_update_interval_minutes", "30"); err != nil {
		return err
	}

	return nil
}

func GetSettings() (Settings, error) {
	db := database.GetDB()
	var s Settings

	getValue := func(key string) string {
		var val string
		_ = db.QueryRow("SELECT value FROM global_settings WHERE key = ?", key).Scan(&val)
		return val
	}

	s.LanUrl = getValue("lan_url")
	s.WanUrl = getValue("wan_url")
	s.AppStoreServerUrl = getValue("appstore_server_url")
	if s.AppStoreServerUrl == "" {
		s.AppStoreServerUrl = DefaultAppStoreServerURL
	}

	parseInt := func(v string, def int) int {
		if v == "" {
			return def
		}
		i, err := strconv.Atoi(v)
		if err != nil {
			return def
		}
		return i
	}

	s.AllocPortStart = parseInt(getValue("alloc_port_start"), 20000)
	s.AllocPortEnd = parseInt(getValue("alloc_port_end"), 30000)
	s.ImageUpdateIntervalMinutes = parseInt(getValue("image_update_interval_minutes"), 30)

	return s, nil
}

// GetDataDir 获取数据目录
func GetDataDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		if exe, eerr := os.Executable(); eerr == nil {
			return filepath.Join(filepath.Dir(exe), "data")
		}
		return filepath.Join(".", "data")
	}
	return filepath.Join(cwd, "data")
}

// GetProjectRoot 获取项目根目录
func GetProjectRoot() string {
	parent := filepath.Dir(GetDataDir())
	return filepath.Join(parent, "project")
}

func GetHostProjectRoot() string {
	v := strings.TrimSpace(os.Getenv("PROJECT_ROOT"))
	v = strings.TrimRight(v, "/")
	return v
}

// GetAppStoreBasePath 获取应用商店基础路径
func GetAppStoreBasePath() string {
	return GetDataDir()
}

// GetLanUrl 获取局域网地址
func GetLanUrl() string {
	s, _ := GetSettings()
	return s.LanUrl
}

// GetWanUrl 获取外网地址
func GetWanUrl() string {
	s, _ := GetSettings()
	return s.WanUrl
}

func UpdateSettings(s Settings) error {
	db := database.GetDB()

	if IsDebugEnabled() {
		log.Printf(
			"Updating settings: lanUrl=%s wanUrl=%s appStoreServerUrl=%s allocPortStart=%d allocPortEnd=%d imageUpdateIntervalMinutes=%d",
			s.LanUrl,
			s.WanUrl,
			RedactAppStoreURL(s.AppStoreServerUrl),
			s.AllocPortStart,
			s.AllocPortEnd,
			s.ImageUpdateIntervalMinutes,
		)
	}

	// Helper to update
	update := func(key, value string) error {
		_, err := db.Exec("INSERT OR REPLACE INTO global_settings (key, value) VALUES (?, ?)", key, value)
		return err
	}

	if err := update("lan_url", s.LanUrl); err != nil {
		return err
	}
	if err := update("wan_url", s.WanUrl); err != nil {
		return err
	}
	if err := update("appstore_server_url", s.AppStoreServerUrl); err != nil {
		return err
	}
	if err := update("alloc_port_start", strconv.Itoa(s.AllocPortStart)); err != nil {
		return err
	}
	if err := update("alloc_port_end", strconv.Itoa(s.AllocPortEnd)); err != nil {
		return err
	}
	if err := update("image_update_interval_minutes", strconv.Itoa(s.ImageUpdateIntervalMinutes)); err != nil {
		return err
	}

	return nil
}

func GetValue(key string) (string, error) {
	db := database.GetDB()
	var val string
	err := db.QueryRow("SELECT value FROM global_settings WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func SetValue(key, value string) error {
	db := database.GetDB()
	_, err := db.Exec("INSERT OR REPLACE INTO global_settings (key, value) VALUES (?, ?)", key, value)
	return err
}
