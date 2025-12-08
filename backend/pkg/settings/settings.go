package settings

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"log"
)

type Settings struct {
	LanUrl string `json:"lanUrl"`
	WanUrl string `json:"wanUrl"`
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

	// 为了兼容旧的 server_url，可以选择保留或者迁移。这里简单起见，如果存在 server_url，可以迁移到 lan_url
	// 但为了避免复杂逻辑，我们假设用户会重新设置

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

	return s, nil
}

func UpdateSettings(s Settings) error {
	db := database.GetDB()

	log.Printf("Updating settings: LanUrl=%s, WanUrl=%s", s.LanUrl, s.WanUrl)

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
