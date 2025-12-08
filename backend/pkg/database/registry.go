package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

type Registry struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	IsDefault bool   `json:"is_default"`
	CreatedAt string `json:"created_at"` // 改为 string 类型
	UpdatedAt string `json:"updated_at"` // 改为 string 类型
}

// 添加清除注册表的函数
func ClearRegistries() error {
	_, err := db.Exec("DELETE FROM registries")
	return err
}

// 修改 SaveRegistry 函数，添加必填字段验证
// SaveRegistry 保存注册表配置
func SaveRegistry(registry *Registry) error {
	log.Printf("开始保存注册表: Name=%s, URL=%s", registry.Name, registry.URL)

	// 检查数据库连接
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}

	// 检查是否已存在相同 URL 的注册表
	var id int64
	err := db.QueryRow("SELECT id FROM registries WHERE url = ?", registry.URL).Scan(&id)

	now := time.Now().Format("2006-01-02 15:04:05")

	if err == nil {
		// 更新现有注册表
		log.Printf("更新现有注册表: %s (ID: %d, URL: %s)", registry.Name, id, registry.URL)
		_, err = db.Exec(`
            UPDATE registries 
            SET name = ?, username = ?, password = ?, is_default = ?, updated_at = ? 
            WHERE id = ?
        `, registry.Name, registry.Username, registry.Password,
			boolToInt(registry.IsDefault), now, id)
		return err
	} else if err == sql.ErrNoRows {
		// 插入新注册表
		log.Printf("插入新注册表: %s (URL: %s)", registry.Name, registry.URL)

		// 确保 URL 不为空
		if registry.URL == "" {
			log.Printf("注册表 URL 为空，无法保存: %s", registry.Name)
			return fmt.Errorf("注册表 URL 不能为空")
		}

		result, err := db.Exec(`
            INSERT INTO registries 
            (name, url, username, password, is_default, created_at, updated_at) 
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `, registry.Name, registry.URL, registry.Username, registry.Password,
			boolToInt(registry.IsDefault), now, now)

		if err != nil {
			log.Printf("插入注册表失败: %v", err)
			return err
		}

		// 获取新插入的 ID
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}

		log.Printf("注册表已保存，ID: %d, URL: %s", id, registry.URL)
		return nil
	}

	return err
}

// GetAllRegistries 获取所有注册表配置
func GetAllRegistries() (map[string]*Registry, error) {
	rows, err := db.Query(`
        SELECT id, name, url, username, password, is_default, created_at, updated_at 
        FROM registries
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	registries := make(map[string]*Registry)
	var count int

	for rows.Next() {
		var r Registry
		var isDefault int
		var createdAt, updatedAt string

		err := rows.Scan(&r.ID, &r.Name, &r.URL, &r.Username, &r.Password, &isDefault, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("扫描注册表行失败: %v", err)
			return nil, err
		}

		r.IsDefault = isDefault == 1

		// 使用 URL 作为键，但添加日志以便调试
		log.Printf("从数据库读取注册表: ID=%d, Name=%s, URL=%s", r.ID, r.Name, r.URL)

		// 确保 URL 不为空
		if r.URL != "" {
			registries[r.URL] = &r
			count++
		} else {
			log.Printf("警告: 跳过 URL 为空的注册表: ID=%d, Name=%s", r.ID, r.Name)
		}
	}

	log.Printf("从数据库读取到 %d 个注册表配置", count)

	// 确保 Docker Hub 存在
	if _, ok := registries["docker.io"]; !ok {
		log.Printf("添加默认的 Docker Hub 注册表")
		registries["docker.io"] = &Registry{
			Name:      "Docker Hub",
			URL:       "docker.io",
			IsDefault: true,
		}
	}

	// 打印所有注册表的键值，帮助调试
	for k, v := range registries {
		log.Printf("注册表配置: key=%s, name=%s, url=%s", k, v.Name, v.URL)
	}

	return registries, nil
}
