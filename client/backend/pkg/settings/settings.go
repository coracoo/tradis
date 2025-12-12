package settings

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"log"
	"os"
	"path/filepath"
)

type Settings struct {
	LanUrl            string `json:"lanUrl"`
	WanUrl            string `json:"wanUrl"`
	AppStoreServerUrl string `json:"appStoreServerUrl"`
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

	// Insert default lan_url if not exists
	var val string
	err = db.QueryRow("SELECT value FROM global_settings WHERE key = 'lan_url'").Scan(&val)
	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO global_settings (key, value) VALUES ('lan_url', 'http://localhost')")
		if err != nil {
			return err
		}
	}

	// Insert default wan_url if not exists
	err = db.QueryRow("SELECT value FROM global_settings WHERE key = 'wan_url'").Scan(&val)
	if err == sql.ErrNoRows {
		// 默认外网地址为空或与内网一致，这里先给个空或者 localhost
		_, err = db.Exec("INSERT INTO global_settings (key, value) VALUES ('wan_url', '')")
		if err != nil {
			return err
		}
	}

	// Insert default appstore_server_url if not exists
	err = db.QueryRow("SELECT value FROM global_settings WHERE key = 'appstore_server_url'").Scan(&val)
	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO global_settings (key, value) VALUES ('appstore_server_url', 'http://localhost:3002')")
		if err != nil {
			return err
		}
	}

	return nil
}

func GetSettings() (Settings, error) {
	db := database.GetDB()
	var s Settings

	var val string
	err := db.QueryRow("SELECT value FROM global_settings WHERE key = 'lan_url'").Scan(&val)
	if err == nil {
		s.LanUrl = val
	}

	err = db.QueryRow("SELECT value FROM global_settings WHERE key = 'wan_url'").Scan(&val)
	if err == nil {
		s.WanUrl = val
	}

	err = db.QueryRow("SELECT value FROM global_settings WHERE key = 'appstore_server_url'").Scan(&val)
	if err == nil {
		s.AppStoreServerUrl = val
	} else {
		// 默认值
		s.AppStoreServerUrl = "http://localhost:3002"
	}

	return s, nil
}

func UpdateSettings(s Settings) error {
	db := database.GetDB()

	log.Printf("Updating settings: LanUrl=%s, WanUrl=%s, AppStoreServerUrl=%s", s.LanUrl, s.WanUrl, s.AppStoreServerUrl)

	_, err := db.Exec("INSERT OR REPLACE INTO global_settings (key, value) VALUES ('lan_url', ?)", s.LanUrl)
	if err != nil {
		log.Printf("Error updating lan_url: %v", err)
		return err
	}

	_, err = db.Exec("INSERT OR REPLACE INTO global_settings (key, value) VALUES ('wan_url', ?)", s.WanUrl)
	if err != nil {
		log.Printf("Error updating wan_url: %v", err)
		return err
	}

	_, err = db.Exec("INSERT OR REPLACE INTO global_settings (key, value) VALUES ('appstore_server_url', ?)", s.AppStoreServerUrl)
	if err != nil {
		log.Printf("Error updating appstore_server_url: %v", err)
		return err
	}

	return nil
}

// GetLanUrl 获取内网地址 (Helper)
func GetLanUrl() string {
	s, _ := GetSettings()
	if s.LanUrl == "" {
		return "http://localhost"
	}
	return s.LanUrl
}

// GetWanUrl 获取外网地址 (Helper)
func GetWanUrl() string {
	s, _ := GetSettings()
	return s.WanUrl
}

// GetAppStoreBasePath 获取 AppStore 相对路径根目录 (已弃用，请使用 GetDataDir)
func GetAppStoreBasePath() string {
	return GetDataDir()
}

// GetProjectRoot 获取项目根目录 (包含 go.mod 的目录)
func GetProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	// 1. 如果当前目录有 go.mod，则认为是根目录
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		return cwd
	}

	// 2. 如果是从项目根目录 (docker-manager) 运行，且存在 client/backend/go.mod
	if _, err := os.Stat(filepath.Join(cwd, "client", "backend", "go.mod")); err == nil {
		return filepath.Join(cwd, "client", "backend")
	}

	return cwd
}

// GetDataDir 获取统一的数据存储目录 (.../client/backend/data)
func GetDataDir() string {
	return filepath.Join(GetProjectRoot(), "data")
}
