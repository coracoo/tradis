package settings

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"encoding/json"
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
	LanUrl                      string   `json:"lanUrl"`
	WanUrl                      string   `json:"wanUrl"`
	AppStoreServerUrl           string   `json:"appStoreServerUrl"`
	AdvancedMode                bool     `json:"advancedMode"`
	AllocPortStart              int      `json:"allocPortStart"`
	AllocPortEnd                int      `json:"allocPortEnd"`
	AllowAutoAllocPort          bool     `json:"allowAutoAllocPort"`
	ImageUpdateIntervalMinutes  int      `json:"imageUpdateIntervalMinutes"`
	AiEnabled                   bool     `json:"aiEnabled"`
	AiBaseUrl                   string   `json:"aiBaseUrl"`
	AiApiKey                    string   `json:"aiApiKey,omitempty"`
	AiApiKeySet                 bool     `json:"aiApiKeySet"`
	AiModel                     string   `json:"aiModel"`
	AiTemperature               float64  `json:"aiTemperature"`
	AiPrompt                    string   `json:"aiPrompt"`
	VolumeBackupEnabled         bool     `json:"volumeBackupEnabled"`
	VolumeBackupImage           string   `json:"volumeBackupImage"`
	VolumeBackupEnv             string   `json:"volumeBackupEnv"`
	VolumeBackupCronExpression  string   `json:"volumeBackupCronExpression"`
	VolumeBackupVolumes         []string `json:"volumeBackupVolumes"`
	VolumeBackupArchiveDir      string   `json:"volumeBackupArchiveDir"`
	VolumeBackupMountDockerSock bool     `json:"volumeBackupMountDockerSock"`
}

type ImageRemoteDigestBackoffPolicy struct {
	FirstFailBackoff   time.Duration
	SecondFailBackoff  time.Duration
	MaxConsecutiveFail int
}

func GetImageRemoteDigestBackoffPolicy() ImageRemoteDigestBackoffPolicy {
	return ImageRemoteDigestBackoffPolicy{
		FirstFailBackoff:   24 * time.Hour,
		SecondFailBackoff:  48 * time.Hour,
		MaxConsecutiveFail: 3,
	}
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
	if err := insertDefault("advanced_mode", "false"); err != nil {
		return err
	}
	if err := insertDefault("alloc_port_start", "55500"); err != nil {
		return err
	}
	if err := insertDefault("alloc_port_end", "56000"); err != nil {
		return err
	}
	if err := insertDefault("allow_auto_alloc_port", "false"); err != nil {
		return err
	}
	if err := insertDefault("image_update_interval_minutes", "120"); err != nil {
		return err
	}
	if err := insertDefault("ai_enabled", "false"); err != nil {
		return err
	}
	if err := insertDefault("ai_base_url", "https://api.openai.com/v1"); err != nil {
		return err
	}
	if err := insertDefault("ai_api_key", ""); err != nil {
		return err
	}
	if err := insertDefault("ai_model", ""); err != nil {
		return err
	}
	if err := insertDefault("ai_temperature", "0.7"); err != nil {
		return err
	}
	defaultAIPrompt := "你是一个导航整理助手。你必须只输出严格 JSON：{\"title\":\"\",\"category\":\"\",\"icon\":\"\"}。title 与 category 必须非空。category 必须尽量给出具体中文分类；仅当完全无法判断时输出 未分类。icon 必须是 http(s) 图标 URL 或 mdi-docker。不要输出解释、推理过程、Markdown、代码块或额外字段。"
	if err := insertDefault("ai_prompt", defaultAIPrompt); err != nil {
		return err
	}
	{
		var cur string
		_ = db.QueryRow("SELECT value FROM global_settings WHERE key = ?", "ai_prompt").Scan(&cur)
		clean := strings.TrimSpace(cur)
		if clean != "" {
			isLegacy := strings.Contains(clean, "根据容器信息与端口探测结果") ||
				strings.Contains(clean, "从互联网搜索") ||
				strings.Contains(clean, "图标网络地址") ||
				strings.Contains(clean, "仅当完全无法判断时输出 默认") ||
				strings.Contains(clean, "输出 default")
			if isLegacy {
				_, _ = db.Exec("UPDATE global_settings SET value = ? WHERE key = ?", defaultAIPrompt, "ai_prompt")
			}
		}
	}
	if err := insertDefault("volume_backup_enabled", "false"); err != nil {
		return err
	}
	if err := insertDefault("volume_backup_image", "offen/docker-volume-backup:latest"); err != nil {
		return err
	}
	if err := insertDefault("volume_backup_env", ""); err != nil {
		return err
	}
	if err := insertDefault("volume_backup_cron_expression", "@daily"); err != nil {
		return err
	}
	if err := insertDefault("volume_backup_volumes", "[]"); err != nil {
		return err
	}
	if err := insertDefault("volume_backup_archive_dir", ""); err != nil {
		return err
	}
	if err := insertDefault("volume_backup_mount_docker_sock", "true"); err != nil {
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

	parseBool := func(v string, def bool) bool {
		t := strings.TrimSpace(strings.ToLower(v))
		if t == "" {
			return def
		}
		if t == "1" || t == "true" || t == "yes" || t == "on" {
			return true
		}
		if t == "0" || t == "false" || t == "no" || t == "off" {
			return false
		}
		return def
	}

	s.AdvancedMode = parseBool(getValue("advanced_mode"), false)
	s.AllocPortStart = parseInt(getValue("alloc_port_start"), 55500)
	s.AllocPortEnd = parseInt(getValue("alloc_port_end"), 56000)
	s.AllowAutoAllocPort = parseBool(getValue("allow_auto_alloc_port"), false)
	s.ImageUpdateIntervalMinutes = parseInt(getValue("image_update_interval_minutes"), 120)
	s.AiEnabled = parseBool(getValue("ai_enabled"), false)
	s.AiBaseUrl = strings.TrimSpace(getValue("ai_base_url"))
	s.AiModel = strings.TrimSpace(getValue("ai_model"))
	s.AiPrompt = getValue("ai_prompt")
	key := strings.TrimSpace(getValue("ai_api_key"))
	s.AiApiKeySet = key != ""
	s.AiApiKey = ""
	parseFloat := func(v string, def float64) float64 {
		if strings.TrimSpace(v) == "" {
			return def
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return def
		}
		return f
	}
	s.AiTemperature = parseFloat(getValue("ai_temperature"), 0.7)
	s.VolumeBackupEnabled = parseBool(getValue("volume_backup_enabled"), false)
	s.VolumeBackupImage = strings.TrimSpace(getValue("volume_backup_image"))
	if s.VolumeBackupImage == "" {
		s.VolumeBackupImage = "offen/docker-volume-backup:latest"
	}
	s.VolumeBackupEnv = getValue("volume_backup_env")
	s.VolumeBackupCronExpression = strings.TrimSpace(getValue("volume_backup_cron_expression"))
	if s.VolumeBackupCronExpression == "" {
		s.VolumeBackupCronExpression = "@daily"
	}
	s.VolumeBackupVolumes = parseStringSlice(getValue("volume_backup_volumes"))
	s.VolumeBackupArchiveDir = strings.TrimSpace(getValue("volume_backup_archive_dir"))
	s.VolumeBackupMountDockerSock = parseBool(getValue("volume_backup_mount_docker_sock"), true)

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
			"Updating settings: lanUrl=%s wanUrl=%s appStoreServerUrl=%s advancedMode=%t allocPortStart=%d allocPortEnd=%d allowAutoAllocPort=%t imageUpdateIntervalMinutes=%d aiEnabled=%t aiBaseUrl=%s aiModel=%s aiTemperature=%v volumeBackupEnabled=%t volumeBackupImage=%s",
			s.LanUrl,
			s.WanUrl,
			RedactAppStoreURL(s.AppStoreServerUrl),
			s.AdvancedMode,
			s.AllocPortStart,
			s.AllocPortEnd,
			s.AllowAutoAllocPort,
			s.ImageUpdateIntervalMinutes,
			s.AiEnabled,
			s.AiBaseUrl,
			s.AiModel,
			s.AiTemperature,
			s.VolumeBackupEnabled,
			s.VolumeBackupImage,
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
	if err := update("advanced_mode", strconv.FormatBool(s.AdvancedMode)); err != nil {
		return err
	}
	if err := update("alloc_port_start", strconv.Itoa(s.AllocPortStart)); err != nil {
		return err
	}
	if err := update("alloc_port_end", strconv.Itoa(s.AllocPortEnd)); err != nil {
		return err
	}
	if err := update("allow_auto_alloc_port", strconv.FormatBool(s.AllowAutoAllocPort)); err != nil {
		return err
	}
	if err := update("image_update_interval_minutes", strconv.Itoa(s.ImageUpdateIntervalMinutes)); err != nil {
		return err
	}
	if err := update("ai_enabled", strconv.FormatBool(s.AiEnabled)); err != nil {
		return err
	}
	if err := update("ai_base_url", s.AiBaseUrl); err != nil {
		return err
	}
	if err := update("ai_model", s.AiModel); err != nil {
		return err
	}
	if err := update("ai_temperature", strconv.FormatFloat(s.AiTemperature, 'f', -1, 64)); err != nil {
		return err
	}
	if err := update("ai_prompt", s.AiPrompt); err != nil {
		return err
	}
	if err := update("volume_backup_enabled", strconv.FormatBool(s.VolumeBackupEnabled)); err != nil {
		return err
	}
	if err := update("volume_backup_image", strings.TrimSpace(s.VolumeBackupImage)); err != nil {
		return err
	}
	if err := update("volume_backup_env", s.VolumeBackupEnv); err != nil {
		return err
	}
	if err := update("volume_backup_cron_expression", strings.TrimSpace(s.VolumeBackupCronExpression)); err != nil {
		return err
	}
	volumesJSON := "[]"
	if b, err := json.Marshal(normalizeStringSlice(s.VolumeBackupVolumes)); err == nil {
		volumesJSON = string(b)
	}
	if err := update("volume_backup_volumes", volumesJSON); err != nil {
		return err
	}
	if err := update("volume_backup_archive_dir", strings.TrimSpace(s.VolumeBackupArchiveDir)); err != nil {
		return err
	}
	if err := update("volume_backup_mount_docker_sock", strconv.FormatBool(s.VolumeBackupMountDockerSock)); err != nil {
		return err
	}

	return nil
}

func normalizeStringSlice(list []string) []string {
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, v := range list {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func parseStringSlice(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	var list []string
	if json.Unmarshal([]byte(s), &list) == nil {
		return normalizeStringSlice(list)
	}
	return normalizeStringSlice(strings.Split(s, ","))
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
