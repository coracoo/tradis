package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

type columnSpec struct {
	Name         string
	AddColumnSQL string
	BackfillSQL  []string
}

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

// ensureTableColumns 确保指定表包含必要列，兼容旧数据库升级。
func ensureTableColumns(table string, cols []columnSpec) error {
	if strings.TrimSpace(table) == "" {
		return nil
	}
	for _, r := range table {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("非法表名: %s", table)
	}

	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	exists := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		exists[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, c := range cols {
		if c.Name == "" || strings.TrimSpace(c.AddColumnSQL) == "" {
			continue
		}
		if exists[c.Name] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + c.AddColumnSQL); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return err
		}
		for _, bf := range c.BackfillSQL {
			if strings.TrimSpace(bf) == "" {
				continue
			}
			if _, err := db.Exec(bf); err != nil {
				return err
			}
		}
	}
	return nil
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
	if err := ensureTableColumns("registries", []columnSpec{
		{Name: "username", AddColumnSQL: "username TEXT"},
		{Name: "password", AddColumnSQL: "password TEXT"},
		{Name: "is_default", AddColumnSQL: "is_default INTEGER DEFAULT 0"},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME"},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME"},
	}); err != nil {
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
	if err := ensureTableColumns("docker_proxy", []columnSpec{
		{Name: "enabled", AddColumnSQL: "enabled INTEGER DEFAULT 0", BackfillSQL: []string{"UPDATE docker_proxy SET enabled = 0 WHERE enabled IS NULL"}},
		{Name: "http_proxy", AddColumnSQL: "http_proxy TEXT"},
		{Name: "https_proxy", AddColumnSQL: "https_proxy TEXT"},
		{Name: "no_proxy", AddColumnSQL: "no_proxy TEXT"},
		{Name: "registry_mirrors", AddColumnSQL: "registry_mirrors TEXT"},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE docker_proxy SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE docker_proxy SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
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
	if err := ensureTableColumns("proxy_history", []columnSpec{
		{Name: "enabled", AddColumnSQL: "enabled INTEGER DEFAULT 0", BackfillSQL: []string{"UPDATE proxy_history SET enabled = 0 WHERE enabled IS NULL"}},
		{Name: "http_proxy", AddColumnSQL: "http_proxy TEXT"},
		{Name: "https_proxy", AddColumnSQL: "https_proxy TEXT"},
		{Name: "no_proxy", AddColumnSQL: "no_proxy TEXT"},
		{Name: "registry_mirrors", AddColumnSQL: "registry_mirrors TEXT"},
		{Name: "change_type", AddColumnSQL: "change_type TEXT"},
		{Name: "changed_at", AddColumnSQL: "changed_at DATETIME", BackfillSQL: []string{"UPDATE proxy_history SET changed_at = CURRENT_TIMESTAMP WHERE changed_at IS NULL"}},
	}); err != nil {
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
	if err := ensureTableColumns("applications", []columnSpec{
		{Name: "description", AddColumnSQL: "description TEXT"},
		{Name: "icon_url", AddColumnSQL: "icon_url TEXT"},
		{Name: "category", AddColumnSQL: "category TEXT"},
		{Name: "version", AddColumnSQL: "version TEXT"},
		{Name: "image_name", AddColumnSQL: "image_name TEXT"},
		{Name: "port_mappings", AddColumnSQL: "port_mappings TEXT"},
		{Name: "environment_vars", AddColumnSQL: "environment_vars TEXT"},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE applications SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE applications SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
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

	if err := ensureTableColumns("navigation_items", []columnSpec{
		{Name: "is_deleted", AddColumnSQL: "is_deleted INTEGER DEFAULT 0"},
		{Name: "icon_path", AddColumnSQL: "icon_path TEXT"},
		{Name: "ai_generated", AddColumnSQL: "ai_generated INTEGER DEFAULT 0"},
	}); err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS ai_logs (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        scope TEXT,
	        level TEXT,
	        message TEXT,
	        details TEXT,
	        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
	if err := ensureTableColumns("deployments", []columnSpec{
		{Name: "app_id", AddColumnSQL: "app_id INTEGER"},
		{Name: "container_id", AddColumnSQL: "container_id TEXT"},
		{Name: "status", AddColumnSQL: "status TEXT"},
		{Name: "port_mappings", AddColumnSQL: "port_mappings TEXT"},
		{Name: "environment_vars", AddColumnSQL: "environment_vars TEXT"},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE deployments SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE deployments SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
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
	if err := ensureTableColumns("port_settings", []columnSpec{
		{Name: "range_start", AddColumnSQL: "range_start INTEGER"},
		{Name: "range_end", AddColumnSQL: "range_end INTEGER"},
		{Name: "protocol", AddColumnSQL: "protocol TEXT"},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE port_settings SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
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
	if err := ensureTableColumns("port_notes", []columnSpec{
		{Name: "note", AddColumnSQL: "note TEXT"},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE port_notes SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
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
	if err := ensureTableColumns("port_reservations", []columnSpec{
		{Name: "reserved_by", AddColumnSQL: "reserved_by TEXT"},
		{Name: "protocol", AddColumnSQL: "protocol TEXT"},
		{Name: "type", AddColumnSQL: "type TEXT"},
		{Name: "reserved_at", AddColumnSQL: "reserved_at DATETIME", BackfillSQL: []string{"UPDATE port_reservations SET reserved_at = CURRENT_TIMESTAMP WHERE reserved_at IS NULL"}},
	}); err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS tasks (
	        id TEXT PRIMARY KEY,
	        type TEXT NOT NULL,
	        status TEXT NOT NULL,
	        result_json TEXT,
	        error TEXT,
	        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}
	if err := ensureTableColumns("tasks", []columnSpec{
		{Name: "type", AddColumnSQL: "type TEXT"},
		{Name: "status", AddColumnSQL: "status TEXT"},
		{Name: "result_json", AddColumnSQL: "result_json TEXT"},
		{Name: "error", AddColumnSQL: "error TEXT"},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE tasks SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE tasks SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS task_logs (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        task_id TEXT NOT NULL,
	        seq INTEGER NOT NULL,
	        time DATETIME DEFAULT CURRENT_TIMESTAMP,
	        type TEXT,
	        message TEXT,
	        UNIQUE(task_id, seq)
	    );
	`)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_logs_task_seq ON task_logs(task_id, seq)`)

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
	if err := ensureTableColumns("notifications", []columnSpec{
		{Name: "type", AddColumnSQL: "type TEXT"},
		{Name: "message", AddColumnSQL: "message TEXT"},
		{Name: "read", AddColumnSQL: "read INTEGER DEFAULT 0", BackfillSQL: []string{"UPDATE notifications SET read = 0 WHERE read IS NULL"}},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE notifications SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
	}); err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS image_updates (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        repo_tag TEXT NOT NULL UNIQUE,
	        image_id TEXT,
	        local_digest TEXT,
	        remote_digest TEXT,
	        notified INTEGER DEFAULT 0,
	        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}
	if err := ensureTableColumns("image_updates", []columnSpec{
		{Name: "repo_tag", AddColumnSQL: "repo_tag TEXT"},
		{Name: "image_id", AddColumnSQL: "image_id TEXT"},
		{Name: "local_digest", AddColumnSQL: "local_digest TEXT"},
		{Name: "remote_digest", AddColumnSQL: "remote_digest TEXT"},
		{Name: "notified", AddColumnSQL: "notified INTEGER DEFAULT 0", BackfillSQL: []string{"UPDATE image_updates SET notified = 0 WHERE notified IS NULL"}},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE image_updates SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE image_updates SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
		return err
	}

	if err := ensureImageUpdatesNotifiedColumn(); err != nil {
		return err
	}

	_, err = db.Exec(`
	    CREATE TABLE IF NOT EXISTS image_remote_digest_status (
	        repo_tag TEXT PRIMARY KEY,
	        fail_count INTEGER DEFAULT 0,
	        unavailable INTEGER DEFAULT 0,
	        next_check_at DATETIME,
	        last_error TEXT,
	        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`)
	if err != nil {
		return err
	}
	if err := ensureTableColumns("image_remote_digest_status", []columnSpec{
		{Name: "repo_tag", AddColumnSQL: "repo_tag TEXT"},
		{Name: "fail_count", AddColumnSQL: "fail_count INTEGER DEFAULT 0", BackfillSQL: []string{"UPDATE image_remote_digest_status SET fail_count = 0 WHERE fail_count IS NULL"}},
		{Name: "unavailable", AddColumnSQL: "unavailable INTEGER DEFAULT 0", BackfillSQL: []string{"UPDATE image_remote_digest_status SET unavailable = 0 WHERE unavailable IS NULL"}},
		{Name: "next_check_at", AddColumnSQL: "next_check_at DATETIME"},
		{Name: "last_error", AddColumnSQL: "last_error TEXT"},
		{Name: "created_at", AddColumnSQL: "created_at DATETIME", BackfillSQL: []string{"UPDATE image_remote_digest_status SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL"}},
		{Name: "updated_at", AddColumnSQL: "updated_at DATETIME", BackfillSQL: []string{"UPDATE image_remote_digest_status SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL"}},
	}); err != nil {
		return err
	}

	return nil
}

// ensureImageUpdatesNotifiedColumn 确保 image_updates 表包含 notified 列，兼容旧数据库。
func ensureImageUpdatesNotifiedColumn() error {
	return ensureTableColumns("image_updates", []columnSpec{
		{
			Name:         "notified",
			AddColumnSQL: "notified INTEGER DEFAULT 0",
			BackfillSQL:  []string{`UPDATE image_updates SET notified = 1`},
		},
	})
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
	Notified     bool   `json:"notified"`
}

type ImageRemoteDigestStatus struct {
	RepoTag     string `json:"repoTag"`
	FailCount   int    `json:"failCount"`
	Unavailable bool   `json:"unavailable"`
	NextCheckAt string `json:"nextCheckAt"`
	LastError   string `json:"lastError"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func GetImageRemoteDigestStatus(repoTag string) (ImageRemoteDigestStatus, error) {
	var s ImageRemoteDigestStatus
	repoTag = strings.TrimSpace(repoTag)
	if repoTag == "" {
		return s, nil
	}

	var unavailable int
	err := db.QueryRow(`
	    SELECT repo_tag, fail_count, unavailable, COALESCE(next_check_at, ''), COALESCE(last_error, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
	    FROM image_remote_digest_status
	    WHERE repo_tag = ?
	`, repoTag).Scan(&s.RepoTag, &s.FailCount, &unavailable, &s.NextCheckAt, &s.LastError, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return ImageRemoteDigestStatus{RepoTag: repoTag}, nil
	}
	if err != nil {
		return s, err
	}
	s.Unavailable = unavailable == 1
	return s, nil
}

func ResetImageRemoteDigestStatus(repoTag string) error {
	repoTag = strings.TrimSpace(repoTag)
	if repoTag == "" {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
	    INSERT INTO image_remote_digest_status (repo_tag, fail_count, unavailable, next_check_at, last_error, created_at, updated_at)
	    VALUES (?, 0, 0, NULL, '', ?, ?)
	    ON CONFLICT(repo_tag) DO UPDATE SET
	      fail_count = 0,
	      unavailable = 0,
	      next_check_at = NULL,
	      last_error = '',
	      updated_at = excluded.updated_at
	`, repoTag, now, now)
	return err
}

func RecordImageRemoteDigestFailure(repoTag string, errMsg string, firstBackoff time.Duration, secondBackoff time.Duration, maxFail int) (ImageRemoteDigestStatus, error) {
	repoTag = strings.TrimSpace(repoTag)
	if repoTag == "" {
		return ImageRemoteDigestStatus{}, nil
	}

	errMsg = strings.TrimSpace(errMsg)
	if len(errMsg) > 800 {
		errMsg = errMsg[:800]
	}

	if maxFail <= 0 {
		maxFail = 3
	}
	if firstBackoff <= 0 {
		firstBackoff = 24 * time.Hour
	}
	if secondBackoff <= 0 {
		secondBackoff = 48 * time.Hour
	}

	existing, err := GetImageRemoteDigestStatus(repoTag)
	if err != nil {
		return ImageRemoteDigestStatus{}, err
	}

	newFail := existing.FailCount + 1
	if newFail > maxFail {
		newFail = maxFail
	}
	unavailable := newFail >= maxFail

	nowTime := time.Now()
	now := nowTime.Format("2006-01-02 15:04:05")

	nextCheckAt := ""
	if !unavailable {
		if newFail == 1 {
			nextCheckAt = nowTime.Add(firstBackoff).Format("2006-01-02 15:04:05")
		} else {
			nextCheckAt = nowTime.Add(secondBackoff).Format("2006-01-02 15:04:05")
		}
	}

	nextCheckAtSQL := sql.NullString{Valid: false}
	if nextCheckAt != "" {
		nextCheckAtSQL = sql.NullString{String: nextCheckAt, Valid: true}
	}

	_, err = db.Exec(`
	    INSERT INTO image_remote_digest_status (repo_tag, fail_count, unavailable, next_check_at, last_error, created_at, updated_at)
	    VALUES (?, ?, ?, ?, ?, ?, ?)
	    ON CONFLICT(repo_tag) DO UPDATE SET
	      fail_count = excluded.fail_count,
	      unavailable = excluded.unavailable,
	      next_check_at = excluded.next_check_at,
	      last_error = excluded.last_error,
	      updated_at = excluded.updated_at
	`, repoTag, newFail, boolToInt(unavailable), nextCheckAtSQL, errMsg, now, now)
	if err != nil {
		return ImageRemoteDigestStatus{}, err
	}
	return GetImageRemoteDigestStatus(repoTag)
}

func ParseSQLiteTime(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// TaskRecord 表示后台任务的数据库记录（用于断线续看/进度查询）。
type TaskRecord struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	ResultJSON string `json:"result_json"`
	Error      string `json:"error"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// TaskLogRecord 表示任务日志的数据库记录（seq 用作 SSE 的 id/游标）。
type TaskLogRecord struct {
	TaskID  string `json:"task_id"`
	Seq     int64  `json:"seq"`
	Time    string `json:"time"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// UpsertTask 任务落库：不存在则创建，存在则更新 type/status/updated_at。
func UpsertTask(id string, taskType string, status string) error {
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	id = strings.TrimSpace(id)
	taskType = strings.TrimSpace(taskType)
	status = strings.TrimSpace(status)
	if id == "" || taskType == "" || status == "" {
		return fmt.Errorf("任务参数不完整")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
	    INSERT INTO tasks (id, type, status, result_json, error, created_at, updated_at)
	    VALUES (?, ?, ?, '', '', ?, ?)
	    ON CONFLICT(id) DO UPDATE SET
	      type = excluded.type,
	      status = excluded.status,
	      updated_at = excluded.updated_at
	`, id, taskType, status, now, now)
	return err
}

// UpdateTaskStatus 更新任务状态（不改变 result_json）。
func UpdateTaskStatus(id string, status string) error {
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if id == "" || status == "" {
		return fmt.Errorf("任务参数不完整")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	return err
}

// FinishTask 结束任务并写入 result_json/error/status。
func FinishTask(id string, status string, result any, errStr string) error {
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if id == "" || status == "" {
		return fmt.Errorf("任务参数不完整")
	}

	resultJSON := ""
	if result != nil {
		if b, err := json.Marshal(result); err == nil {
			resultJSON = string(b)
		}
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
	    UPDATE tasks
	    SET status = ?, result_json = ?, error = ?, updated_at = ?
	    WHERE id = ?
	`, status, resultJSON, strings.TrimSpace(errStr), now, id)
	return err
}

// AppendTaskLogWithSeq 追加任务日志（seq 在业务侧生成，用于稳定的 SSE 断线续看）。
func AppendTaskLogWithSeq(taskID string, seq int64, at time.Time, logType string, message string) error {
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || seq <= 0 {
		return fmt.Errorf("日志参数不完整")
	}
	logType = strings.TrimSpace(logType)
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	now := at
	if now.IsZero() {
		now = time.Now()
	}
	nowStr := now.Format("2006-01-02 15:04:05")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
	    INSERT OR IGNORE INTO task_logs (task_id, seq, time, type, message)
	    VALUES (?, ?, ?, ?, ?)
	`, taskID, seq, nowStr, logType, message); err != nil {
		return err
	}
	_, _ = tx.Exec(`UPDATE tasks SET updated_at = ? WHERE id = ?`, nowStr, taskID)
	return tx.Commit()
}

// GetTask 获取任务详情（用于进度查询/断线续看）。
func GetTask(id string) (TaskRecord, error) {
	var t TaskRecord
	if db == nil {
		return t, fmt.Errorf("数据库连接未初始化")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return t, sql.ErrNoRows
	}
	err := db.QueryRow(`
	    SELECT id, type, status, COALESCE(result_json, ''), COALESCE(error, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
	    FROM tasks
	    WHERE id = ?
	`, id).Scan(&t.ID, &t.Type, &t.Status, &t.ResultJSON, &t.Error, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func buildInPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, "?")
	}
	return strings.Join(parts, ",")
}

// ListTasks 列出任务（支持按 type/status 过滤，默认按 updated_at 倒序）。
func ListTasks(taskTypes []string, statuses []string, limit int) ([]TaskRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接未初始化")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args := make([]any, 0)
	conds := make([]string, 0)

	if len(taskTypes) > 0 {
		ph := buildInPlaceholders(len(taskTypes))
		conds = append(conds, "type IN ("+ph+")")
		for _, v := range taskTypes {
			args = append(args, strings.TrimSpace(v))
		}
	}
	if len(statuses) > 0 {
		ph := buildInPlaceholders(len(statuses))
		conds = append(conds, "status IN ("+ph+")")
		for _, v := range statuses {
			args = append(args, strings.TrimSpace(v))
		}
	}

	query := `
	    SELECT id, type, status, COALESCE(result_json, ''), COALESCE(error, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
	    FROM tasks
	`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]TaskRecord, 0)
	for rows.Next() {
		var t TaskRecord
		if err := rows.Scan(&t.ID, &t.Type, &t.Status, &t.ResultJSON, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// GetTaskLogsAfter 按 seq 游标获取任务日志（用于 SSE 断线续看）。
func GetTaskLogsAfter(taskID string, afterSeq int64, limit int) ([]TaskLogRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接未初始化")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}
	if afterSeq < 0 {
		afterSeq = 0
	}

	rows, err := db.Query(`
	    SELECT task_id, seq, COALESCE(time, ''), COALESCE(type, ''), COALESCE(message, '')
	    FROM task_logs
	    WHERE task_id = ? AND seq > ?
	    ORDER BY seq ASC
	    LIMIT ?
	`, taskID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TaskLogRecord, 0)
	for rows.Next() {
		var r TaskLogRecord
		if err := rows.Scan(&r.TaskID, &r.Seq, &r.Time, &r.Type, &r.Message); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
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
	notified := boolToInt(u.Notified)
	_, err := db.Exec(`
	    INSERT INTO image_updates (repo_tag, image_id, local_digest, remote_digest, notified, created_at, updated_at)
	    VALUES (?, ?, ?, ?, ?, ?, ?)
	    ON CONFLICT(repo_tag) DO UPDATE SET
	      image_id = excluded.image_id,
	      local_digest = excluded.local_digest,
	      remote_digest = excluded.remote_digest,
	      updated_at = excluded.updated_at
	`, u.RepoTag, u.ImageID, u.LocalDigest, u.RemoteDigest, notified, now, now)
	if err != nil {
		return err
	}
	_ = db.QueryRow(`
	    SELECT id, created_at, updated_at, notified
	    FROM image_updates
	    WHERE repo_tag = ?
	`, u.RepoTag).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &notified)
	u.Notified = notified == 1
	return nil
}

func GetAllImageUpdates() ([]ImageUpdate, error) {
	rows, err := db.Query(`
	    SELECT id, repo_tag, image_id, local_digest, remote_digest, created_at, updated_at, notified
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
		var notified int
		if err := rows.Scan(&u.ID, &u.RepoTag, &u.ImageID, &u.LocalDigest, &u.RemoteDigest, &u.CreatedAt, &u.UpdatedAt, &notified); err != nil {
			return nil, err
		}
		u.Notified = notified == 1
		list = append(list, u)
	}

	return list, nil
}

func GetUnnotifiedImageUpdates() ([]ImageUpdate, error) {
	rows, err := db.Query(`
	    SELECT id, repo_tag, image_id, local_digest, remote_digest, created_at, updated_at, notified
	    FROM image_updates
	    WHERE notified = 0
	    ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ImageUpdate
	for rows.Next() {
		var u ImageUpdate
		var notified int
		if err := rows.Scan(&u.ID, &u.RepoTag, &u.ImageID, &u.LocalDigest, &u.RemoteDigest, &u.CreatedAt, &u.UpdatedAt, &notified); err != nil {
			return nil, err
		}
		u.Notified = notified == 1
		list = append(list, u)
	}

	return list, nil
}

func MarkImageUpdatesNotifiedByRepoTags(repoTags []string) error {
	if len(repoTags) == 0 {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	placeholders := make([]string, 0, len(repoTags))
	args := make([]any, 0, len(repoTags)+1)
	args = append(args, now)
	for _, t := range repoTags {
		placeholders = append(placeholders, "?")
		args = append(args, t)
	}
	query := `UPDATE image_updates SET notified = 1, updated_at = ? WHERE repo_tag IN (` + strings.Join(placeholders, ",") + `) AND notified = 0`
	_, err := db.Exec(query, args...)
	return err
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
