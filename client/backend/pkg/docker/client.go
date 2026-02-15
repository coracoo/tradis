package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dockerpanel/backend/pkg/settings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"gopkg.in/yaml.v3"
)

// 新增Client结构体封装Docker客户端
type Client struct {
	*client.Client
}

// Environment 自定义类型以支持数组和Map格式
type Environment map[string]string

// UnmarshalYAML 实现自定义解析
func (e *Environment) UnmarshalYAML(value *yaml.Node) error {
	// 如果是 Map
	if value.Kind == yaml.MappingNode {
		m := make(map[string]string)
		if err := value.Decode(&m); err != nil {
			return err
		}
		*e = m
		return nil
	}

	// 如果是 Sequence (数组) ["KEY=VAL", ...]
	if value.Kind == yaml.SequenceNode {
		m := make(map[string]string)
		var s []string
		if err := value.Decode(&s); err != nil {
			return err
		}
		for _, item := range s {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) == 2 {
				m[parts[0]] = parts[1]
			} else {
				m[parts[0]] = ""
			}
		}
		*e = m
		return nil
	}

	return fmt.Errorf("unsupported type for environment")
}

type ComposeConfig struct {
	Version  string
	Services map[string]ServiceConfig `yaml:"services"`
	Volumes  map[string]struct{}
	Networks map[string]struct{}
}

type ServiceConfig struct {
	Image         string      `yaml:"image"`
	Ports         []string    `yaml:"ports"`
	Volumes       []string    `yaml:"volumes"`
	Environment   Environment `yaml:"environment"`
	Restart       string      `yaml:"restart"`
	Networks      []string    `yaml:"networks"`
	ContainerName string      `yaml:"container_name"`
}

func (c *Client) DeployCompose(ctx context.Context, composePath string, projectName string) error {
	// 读取YAML文件
	yamlFile, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("读取Compose文件失败: %w", err)
	}

	// 解析YAML
	var config ComposeConfig
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		// 如果解析 ComposeConfig 失败，但不一定是致命错误（比如格式不匹配），我们尝试下面的逻辑
		// 但 yaml.Unmarshal 通常只有在语法错误时才报错。结构体不匹配不报错。
		// 所以这里先打印日志，继续尝试
		fmt.Printf("Standard Compose parsing warning: %v\n", err)
	}

	// 如果 Services 为空，尝试作为 v1 格式（顶层即服务）解析
	if len(config.Services) == 0 {
		var services map[string]ServiceConfig
		if err := yaml.Unmarshal(yamlFile, &services); err == nil {
			// 过滤掉非服务的顶层键（如 version, networks, volumes）
			// 简单的做法是：如果有 image 字段，认为是服务
			validServices := make(map[string]ServiceConfig)
			for k, v := range services {
				if v.Image != "" {
					validServices[k] = v
				}
			}
			if len(validServices) > 0 {
				config.Services = validServices
				fmt.Printf("Detected v1 style compose with %d services\n", len(config.Services))
			}
		}
	}

	if len(config.Services) == 0 {
		return fmt.Errorf("未在Compose文件中找到任何服务")
	}

	// 获取 Compose 文件的绝对路径和目录，用于处理相对路径 volume
	// 基础根目录：使用 Compose 文件所在目录（原生逻辑）
	baseDir := filepath.Dir(composePath)

	// 创建网络和卷
	if err := c.createNetworks(ctx, config.Networks); err != nil {
		return err
	}

	// 部署服务
	for name, service := range config.Services {
		// 如果没有指定 ContainerName，使用服务名作为默认容器名
		if service.ContainerName == "" {
			service.ContainerName = name
		}

		// 处理 Volumes 中的相对路径
		for i, vol := range service.Volumes {
			parts := strings.Split(vol, ":")
			if len(parts) > 0 {
				hostPath := parts[0]
				if strings.HasPrefix(hostPath, ".") {
					absHostPath := filepath.Join(baseDir, hostPath)
					parts[0] = absHostPath
					service.Volumes[i] = strings.Join(parts, ":")
					fmt.Printf("Converted volume path: %s -> %s\n", hostPath, absHostPath)
				}
			}
		}

		if err := c.deployService(ctx, name, service, projectName); err != nil {
			return fmt.Errorf("部署服务 %s 失败: %w", name, err)
		}
	}

	return nil
}

// 添加卷清理方法
func (c *Client) PruneVolumes(ctx context.Context) (types.VolumesPruneReport, error) {
	// 使用原生的 Docker SDK 方法
	return c.Client.VolumesPrune(ctx, filters.NewArgs())
}

// 创建网络函数
func (c *Client) createNetworks(ctx context.Context, networks map[string]struct{}) error {
	for name := range networks {
		// 如果网络已存在则跳过，避免因重复创建失败
		_, inspectErr := c.NetworkInspect(ctx, name, types.NetworkInspectOptions{})
		if inspectErr == nil {
			continue
		}
		_, err := c.NetworkCreate(ctx, name, types.NetworkCreate{})
		if err != nil {
			// 已存在时忽略错误
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("创建网络%s失败: %w", name, err)
		}
	}
	return nil
}

// 部署服务
func (c *Client) deployService(ctx context.Context, name string, service ServiceConfig, projectName string) error {
	// 拉取镜像
	reader, err := c.ImagePull(ctx, service.Image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}
	defer reader.Close()
	io.Copy(os.Stdout, reader) // 显示进度

	// 创建容器配置
	config := &container.Config{
		Image: service.Image,
		Env:   convertEnvMap(service.Environment),
		Labels: map[string]string{
			"com.docker.compose.project":     projectName,
			"com.docker.compose.service":     name,
			"com.docker.compose.version":     "1.0", // 模拟版本
			"com.docker.compose.working_dir": filepath.Join(settings.GetAppStoreBasePath(), "project", projectName),
		},
	}

	// 创建容器配置
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: service.Restart, // 直接使用字符串，不需要类型转换
		},
		Binds:        service.Volumes,
		PortBindings: parsePorts(service.Ports),
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}
	for _, netName := range service.Networks {
		networkingConfig.EndpointsConfig[netName] = &network.EndpointSettings{}
	}

	// 创建容器
	resp, err := c.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, service.ContainerName)
	if err != nil {
		return fmt.Errorf("创建容器失败: %w", err)
	}

	// 启动容器
	if err := c.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("启动容器失败: %w", err)
	}

	return nil
}

// 转换环境变量映射为数组
func convertEnvMap(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// 解析端口映射
func parsePorts(ports []string) nat.PortMap {
	portMap := make(nat.PortMap)
	for _, binding := range ports {
		parts := strings.Split(binding, ":")
		if len(parts) == 2 {
			containerPort := parts[1]
			hostPort := parts[0]
			portMap[nat.Port(containerPort)] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			}
		}
	}
	return portMap
}

// 修改构造函数返回自定义Client
func NewDockerClient() (*Client, error) {
	dockerHost := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if dockerHost == "" {
		dockerSock := strings.TrimSpace(os.Getenv("DOCKER_SOCK"))
		if dockerSock == "" {
			dockerSock = "/var/run/docker.sock"
		}
		if strings.HasPrefix(dockerSock, "unix://") {
			dockerHost = dockerSock
		} else {
			dockerHost = "unix://" + dockerSock
		}
	}

	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if strings.TrimSpace(os.Getenv("DOCKER_HOST")) != "" {
		opts = append(opts, client.FromEnv)
	} else {
		opts = append(opts, client.WithHost(dockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	return &Client{cli}, nil
}

// 关闭Client
func (cli *Client) Close() error {
	return cli.Client.Close()
}
