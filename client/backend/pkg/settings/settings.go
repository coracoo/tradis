package settings

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"log"
	"path/filepath"
	"strconv"
)

type Settings struct {
	LanUrl            string `json:"lanUrl"`
	WanUrl            string `json:"wanUrl"`
	AppStoreServerUrl string `json:"appStoreServerUrl"`
	AllocPortStart    int    `json:"allocPortStart"`
	AllocPortEnd      int    `json:"allocPortEnd"`
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
	if err := insertDefault("appstore_server_url", "https://template.cgakki.top:33333"); err != nil {
		return err
	}
	if err := insertDefault("alloc_port_start", "20000"); err != nil {
		return err
	}
	if err := insertDefault("alloc_port_end", "30000"); err != nil {
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
		s.AppStoreServerUrl = "https://template.cgakki.top:33333"
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

	return s, nil
}

// GetDataDir 获取数据目录
func GetDataDir() string {
	return "data"
}

// GetProjectRoot 获取项目根目录
func GetProjectRoot() string {
	return filepath.Join(GetDataDir(), "project")
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

	log.Printf("Updating settings: %+v", s)

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

	return nil
}
