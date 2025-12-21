package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

// DaemonConfig 定义 Docker daemon.json 的配置结构
type DaemonConfig struct {
	RegistryMirrors []string            `json:"registry-mirrors,omitempty"`
	Proxies         *ProxyConfig        `json:"proxies,omitempty"`
	ClearProxies    bool                `json:"-"`
	Registries      map[string]Registry `json:"registries,omitempty"`
	IPv6            bool                `json:"ipv6,omitempty"`
	FixedCIDRv6     string              `json:"fixed-cidr-v6,omitempty"`
	MinAPIVersion   string              `json:"min-api-version,omitempty"`
}

// ProxyConfig 定义代理配置结构
type ProxyConfig struct {
	HTTPProxy  string `json:"http-proxy,omitempty"`
	HTTPSProxy string `json:"https-proxy,omitempty"`
	NoProxy    string `json:"no-proxy,omitempty"`
}

// UpdateDaemonConfig 更新 Docker daemon.json 配置
func UpdateDaemonConfig(config *DaemonConfig) error {
	configPath, err := GetDaemonConfigPath()
	if err != nil {
		return fmt.Errorf("获取配置路径失败: %v", err)
	}

	var existing map[string]interface{}
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		existing = make(map[string]interface{})
	} else {
		raw, readErr := ioutil.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("读取配置文件失败: %v", readErr)
		}
		if len(raw) == 0 {
			existing = make(map[string]interface{})
		} else {
			if unmarshalErr := json.Unmarshal(raw, &existing); unmarshalErr != nil {
				return fmt.Errorf("解析配置文件失败: %v", unmarshalErr)
			}
		}
	}

	if config.RegistryMirrors != nil {
		existing["registry-mirrors"] = config.RegistryMirrors
	}

	if config.IPv6 {
		existing["ipv6"] = true
	}
	if config.FixedCIDRv6 != "" {
		existing["fixed-cidr-v6"] = config.FixedCIDRv6
	}

	if config.Proxies != nil {
		proxies := map[string]string{}
		if config.Proxies.HTTPProxy != "" {
			proxies["http-proxy"] = config.Proxies.HTTPProxy
		}
		if config.Proxies.HTTPSProxy != "" {
			proxies["https-proxy"] = config.Proxies.HTTPSProxy
		}
		if config.Proxies.NoProxy != "" {
			proxies["no-proxy"] = config.Proxies.NoProxy
		}
		existing["proxies"] = proxies
	} else if config.ClearProxies {
		delete(existing, "proxies")
	}

	if config.Registries != nil {
		existing["registries"] = config.Registries
	}

	if config.MinAPIVersion != "" {
		existing["min-api-version"] = config.MinAPIVersion
	}

	data, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}
	return nil
}

type Registry struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// GetDaemonConfigPath 获取 daemon.json 文件路径
func GetDaemonConfigPath() (string, error) {
	var configPath string

	switch runtime.GOOS {
	case "windows":
		configPath = filepath.Join(os.Getenv("ProgramData"), "Docker", "config", "daemon.json")
	case "linux":
		configPath = "/etc/docker/daemon.json"
	case "darwin":
		configPath = filepath.Join(os.Getenv("HOME"), "Library", "Containers", "com.docker.docker", "Data", "daemon.json")
	default:
		return "", fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	return configPath, nil
}

// GetDaemonConfig 读取 Docker daemon.json 配置
func GetDaemonConfig() (*DaemonConfig, error) {
	configPath, err := GetDaemonConfigPath()
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 如果文件不存在，返回空配置
		return &DaemonConfig{}, nil
	}

	// 读取配置文件
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取 daemon.json 失败: %v", err)
	}

	// 如果文件为空，返回空配置
	if len(data) == 0 {
		return &DaemonConfig{}, nil
	}

	// 解析 JSON
	var config DaemonConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 daemon.json 失败: %v", err)
	}

	return &config, nil
}

func checkConfigPermissions(configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); err == nil {
		// 尝试打开文件进行写入测试
		f, err := os.OpenFile(configPath, os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("无写入权限: %v", err)
		}
		f.Close()
	}
	return nil
}
