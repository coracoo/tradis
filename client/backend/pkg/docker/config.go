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
    RegistryMirrors []string     `json:"registry-mirrors,omitempty"`
    Proxies        *ProxyConfig  `json:"proxies,omitempty"`
    Registries     map[string]Registry  `json:"registries,omitempty"`
    IPv6           bool          `json:"ipv6,omitempty"`
    FixedCIDRv6    string        `json:"fixed-cidr-v6,omitempty"`
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
    
    // 读取现有配置
    existingConfig, err := GetDaemonConfig()
    if err != nil {
        existingConfig = &DaemonConfig{}
    }

    // 合并配置
    if config.RegistryMirrors != nil {
        existingConfig.RegistryMirrors = config.RegistryMirrors
    }

    // 处理 IPv6 设置
    if config.IPv6 {
        existingConfig.IPv6 = true
    }
    if config.FixedCIDRv6 != "" {
        existingConfig.FixedCIDRv6 = config.FixedCIDRv6
    }

    // 处理代理配置
    if config.Proxies != nil {
        if existingConfig.Proxies == nil {
            existingConfig.Proxies = &ProxyConfig{}
        }
        if config.Proxies.HTTPProxy != "" {
            existingConfig.Proxies.HTTPProxy = config.Proxies.HTTPProxy
        }
        if config.Proxies.HTTPSProxy != "" {
            existingConfig.Proxies.HTTPSProxy = config.Proxies.HTTPSProxy
        }
        if config.Proxies.NoProxy != "" {
            existingConfig.Proxies.NoProxy = config.Proxies.NoProxy
        }
    } else {
        // 如果传入的代理配置为 nil，表示清除代理设置
        existingConfig.Proxies = nil
    }

    // 格式化 JSON 数据
    data, err := json.MarshalIndent(existingConfig, "", "    ")
    if err != nil {
        return err
    }

    // 写入文件
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
