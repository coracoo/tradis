package database

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// InitDB 初始化数据库连接
func InitDB(dbPath string) error {
	// 确保数据目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	log.Printf("正在打开数据库: %s", dbPath)

	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// 测试数据库连接
	if err := db.Ping(); err != nil {
		return err
	}

	// 创建表
	return createTables()
}

// createTables 创建必要的数据库表
func createTables() error {
	var err error
	// 创建用户表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT NOT NULL UNIQUE,
        password TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `)
	if err != nil {
		return err
	}

	// 初始化管理员账户
	if err = initAdminUser(); err != nil {
		log.Printf("警告: 初始化管理员账户失败: %v", err)
	}
	_ = maybeResetAdminPassword()

	// 创建注册表配置表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS registries (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        url TEXT NOT NULL,
        username TEXT,
        password TEXT,
        is_default INTEGER DEFAULT 0,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `)
	if err != nil {
		return err
	}

	// 创建 Docker 代理配置表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS docker_proxy (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        enabled INTEGER DEFAULT 0,
        http_proxy TEXT,
        https_proxy TEXT,
        no_proxy TEXT,
        registry_mirrors TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )`)
	if err != nil {
		log.Printf("创建 docker_proxy 表失败: %v", err)
		return err
	}

	// 创建代理历史记录表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS proxy_history (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        enabled INTEGER DEFAULT 0,
        http_proxy TEXT,
        https_proxy TEXT,
        no_proxy TEXT,
        registry_mirrors TEXT,
        change_type TEXT,
        changed_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )`)
	if err != nil {
		return err
	}

	// 创建应用商店表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS applications (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        description TEXT,
        icon_url TEXT,
        category TEXT,
        version TEXT,
        image_name TEXT NOT NULL,
        port_mappings TEXT,
        environment_vars TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `)
	if err != nil {
		return err
	}

	// 创建导航项表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS navigation_items (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        url TEXT,
        lan_url TEXT,
        wan_url TEXT,
        icon TEXT,
        category TEXT,
        is_auto INTEGER DEFAULT 0,
        is_deleted INTEGER DEFAULT 0,
        container_id TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `)
	if err != nil {
		return err
	}

	// 尝试添加 is_deleted 列（为了兼容旧数据库）
	_, _ = db.Exec(`ALTER TABLE navigation_items ADD COLUMN is_deleted INTEGER DEFAULT 0`)
	// 尝试添加 icon_path 列（保存本地图标的绝对路径，便于维护）
	_, _ = db.Exec(`ALTER TABLE navigation_items ADD COLUMN icon_path TEXT`)

	// 创建全局设置表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS global_settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	if err != nil {
		return err
	}

	// 确保 server_url 有默认值
	var val string
	err = db.QueryRow("SELECT value FROM global_settings WHERE key = 'server_url'").Scan(&val)
	if err == sql.ErrNoRows {
		_, _ = db.Exec("INSERT INTO global_settings (key, value) VALUES ('server_url', 'http://localhost')")
	}

	// 创建部署表
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS deployments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        app_id INTEGER,
        container_id TEXT,
        status TEXT,
        port_mappings TEXT,
        environment_vars TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (app_id) REFERENCES applications(id)
    )`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS port_settings (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        range_start INTEGER NOT NULL,
	        range_end INTEGER NOT NULL,
	        protocol TEXT NOT NULL,
	        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS port_notes (
	        port INTEGER NOT NULL,
	        type TEXT NOT NULL,
	        protocol TEXT NOT NULL,
	        note TEXT,
	        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	        PRIMARY KEY (port, type, protocol)
	    );
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS port_reservations (
	        port INTEGER PRIMARY KEY,
	        reserved_by TEXT,
	        protocol TEXT,
	        type TEXT,
	        reserved_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS notifications (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        type TEXT,
	        message TEXT,
	        read INTEGER DEFAULT 0,
	        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS image_updates (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        repo_tag TEXT NOT NULL UNIQUE,
	        image_id TEXT,
	        local_digest TEXT,
	        remote_digest TEXT,
	        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}

	return nil
}

type Notification struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
	Read      bool   `json:"read"`
}

type ImageUpdate struct {
	ID           int64  `json:"id"`
	RepoTag      string `json:"repo_tag"`
	ImageID      string `json:"image_id"`
	LocalDigest  string `json:"local_digest"`
	RemoteDigest string `json:"remote_digest"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func SaveNotification(n *Notification) error {
	if n == nil {
		return nil
	}
	if n.Message == "" {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	read := boolToInt(n.Read)
	res, err := db.Exec(`
	    INSERT INTO notifications (type, message, read, created_at)
	    VALUES (?, ?, ?, ?)
	`, n.Type, n.Message, read, now)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		n.ID = id
		n.CreatedAt = now
	}
	return nil
}

func GetNotifications(limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`
	    SELECT id, type, message, created_at, read
	    FROM notifications
	    ORDER BY id DESC
	    LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Notification
	for rows.Next() {
		var n Notification
		var readInt int
		if err := rows.Scan(&n.ID, &n.Type, &n.Message, &n.CreatedAt, &readInt); err != nil {
			return nil, err
		}
		n.Read = readInt == 1
		list = append(list, n)
	}

	return list, nil
}

func DeleteNotification(id int64) error {
	_, err := db.Exec(`DELETE FROM notifications WHERE id = ?`, id)
	return err
}

func MarkAllNotificationsRead() error {
	_, err := db.Exec(`UPDATE notifications SET read = 1 WHERE read = 0`)
	return err
}

func ClearImageUpdates() error {
	_, err := db.Exec(`DELETE FROM image_updates`)
	return err
}

func DeleteImageUpdateByRepoTag(repoTag string) error {
	if repoTag == "" {
		return nil
	}
	_, err := db.Exec(`DELETE FROM image_updates WHERE repo_tag = ?`, repoTag)
	return err
}

func SaveImageUpdate(u *ImageUpdate) error {
	if u == nil {
		return nil
	}
	if u.RepoTag == "" {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
	    INSERT INTO image_updates (repo_tag, image_id, local_digest, remote_digest, created_at, updated_at)
	    VALUES (?, ?, ?, ?, ?, ?)
	    ON CONFLICT(repo_tag) DO UPDATE SET
	      image_id = excluded.image_id,
	      local_digest = excluded.local_digest,
	      remote_digest = excluded.remote_digest,
	      updated_at = excluded.updated_at
	`, u.RepoTag, u.ImageID, u.LocalDigest, u.RemoteDigest, now, now)
	if err != nil {
		return err
	}
	_ = db.QueryRow(`
	    SELECT id, created_at, updated_at
	    FROM image_updates
	    WHERE repo_tag = ?
	`, u.RepoTag).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	return nil
}

func GetAllImageUpdates() ([]ImageUpdate, error) {
	rows, err := db.Query(`
	    SELECT id, repo_tag, image_id, local_digest, remote_digest, created_at, updated_at
	    FROM image_updates
	    ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ImageUpdate
	for rows.Next() {
		var u ImageUpdate
		if err := rows.Scan(&u.ID, &u.RepoTag, &u.ImageID, &u.LocalDigest, &u.RemoteDigest, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, u)
	}

	return list, nil
}

// initAdminUser 初始化管理员账户
func initAdminUser() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "default_password" // 默认密码，生产环境应强制修改
			log.Println("警告: 未设置 ADMIN_PASSWORD 环境变量，使用默认密码: default_password")
		}

		// 使用 bcrypt 哈希存储管理员密码
		hash, herr := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		if herr != nil {
			return herr
		}
		_, err = db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "admin", string(hash))
		if err != nil {
			return err
		}
		log.Println("管理员账户已创建 (username: admin)")
	}
	return nil
}

func maybeResetAdminPassword() error {
	force := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_FORCE_RESET")))
	if force != "1" && force != "true" {
		return nil
	}
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "default_password"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE users SET password = ? WHERE username = 'admin'", string(hash))
	return err
}
func GetDB() *sql.DB {
	return db
}

func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
