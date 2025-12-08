package database

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
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
	if err := initAdminUser(); err != nil {
		log.Printf("警告: 初始化管理员账户失败: %v", err)
	}

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
        container_id TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `)
	if err != nil {
		return err
	}

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

	return nil
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

		// 这里暂时存储明文，实际应存储哈希
		// 为了简单演示，后续建议集成 bcrypt
		_, err = db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "admin", adminPassword)
		if err != nil {
			return err
		}
		log.Println("管理员账户已创建 (username: admin)")
	}
	return nil
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
