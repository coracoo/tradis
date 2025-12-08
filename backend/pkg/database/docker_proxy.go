package database

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// DockerProxy 结构体表示 Docker 代理配置
type DockerProxy struct {
	ID              int64  `json:"id"`
	Enabled         bool   `json:"enabled"`
	HTTPProxy       string `json:"http_proxy"`
	HTTPSProxy      string `json:"https_proxy"`
	NoProxy         string `json:"no_proxy"`
	RegistryMirrors string `json:"registry_mirrors"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// GetDockerProxy 获取 Docker 代理配置
func GetDockerProxy() (*DockerProxy, error) {
	var proxy DockerProxy

	// 查询最新的配置
	row := db.QueryRow(`
        SELECT id, enabled, http_proxy, https_proxy, no_proxy, registry_mirrors, 
               created_at, updated_at 
        FROM docker_proxy 
        ORDER BY id DESC LIMIT 1
    `)

	var enabled int
	err := row.Scan(
		&proxy.ID,
		&enabled,
		&proxy.HTTPProxy,
		&proxy.HTTPSProxy,
		&proxy.NoProxy,
		&proxy.RegistryMirrors,
		&proxy.CreatedAt,
		&proxy.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// 如果没有配置，创建一个默认配置
			return createDefaultDockerProxy()
		}
		return nil, err
	}

	proxy.Enabled = enabled == 1
	return &proxy, nil
}

// SaveDockerProxy 保存 Docker 代理配置
func SaveDockerProxy(proxy *DockerProxy) error {
	// 检查是否已存在配置
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM docker_proxy").Scan(&count)
	if err != nil {
		return err
	}

	var enabled int
	if proxy.Enabled {
		enabled = 1
	} else {
		enabled = 0
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	if count > 0 {
		// 更新现有配置
		_, err = db.Exec(`
            UPDATE docker_proxy 
            SET enabled = ?, http_proxy = ?, https_proxy = ?, no_proxy = ?, 
                registry_mirrors = ?, updated_at = ?
            WHERE id = (SELECT id FROM docker_proxy ORDER BY id DESC LIMIT 1)
        `, enabled, proxy.HTTPProxy, proxy.HTTPSProxy, proxy.NoProxy,
			proxy.RegistryMirrors, now)
	} else {
		// 插入新配置
		_, err = db.Exec(`
            INSERT INTO docker_proxy 
            (enabled, http_proxy, https_proxy, no_proxy, registry_mirrors, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `, enabled, proxy.HTTPProxy, proxy.HTTPSProxy, proxy.NoProxy,
			proxy.RegistryMirrors, now, now)
	}

	return err
}

// createDefaultDockerProxy 创建默认的 Docker 代理配置
func createDefaultDockerProxy() (*DockerProxy, error) {
	proxy := &DockerProxy{
		Enabled:         false,
		HTTPProxy:       "",
		HTTPSProxy:      "",
		NoProxy:         "",
		RegistryMirrors: "",
	}

	err := SaveDockerProxy(proxy)
	if err != nil {
		return nil, err
	}

	return GetDockerProxy()
}

// MarshalRegistryMirrors 将镜像加速器列表转换为 JSON 字符串
func MarshalRegistryMirrors(mirrors []string) string {
	if len(mirrors) == 0 {
		return ""
	}

	data, err := json.Marshal(mirrors)
	if err != nil {
		log.Printf("序列化镜像加速器列表失败: %v", err)
		return ""
	}

	return string(data)
}
