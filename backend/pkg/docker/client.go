package docker

import (
    "context"
    "fmt"
    "io"
    "os"
    "strings"

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

type ComposeConfig struct {
    Version  string
    Services map[string]ServiceConfig
    Volumes  map[string]struct{}
    Networks map[string]struct{}
}

type ServiceConfig struct {
    Image         string
    Ports         []string
    Volumes       []string
    Environment   map[string]string
    Restart       string
    Networks      []string
    ContainerName string `yaml:"container_name"`
}

func (c *Client) DeployCompose(ctx context.Context, composePath string) error {
    // 读取YAML文件
    yamlFile, err := os.ReadFile(composePath)
    if err != nil {
        return fmt.Errorf("读取Compose文件失败: %w", err)
    }

    // 解析YAML
    var config ComposeConfig
    if err := yaml.Unmarshal(yamlFile, &config); err != nil {
        return fmt.Errorf("解析YAML失败: %w", err)
    }

    // 创建网络和卷
    if err := c.createNetworks(ctx, config.Networks); err != nil {
        return err
    }

    // 部署服务
    for name, service := range config.Services {
        if err := c.deployService(ctx, name, service); err != nil {
            return err
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
        _, err := c.NetworkCreate(ctx, name, types.NetworkCreate{})
        if err != nil && !client.IsErrNotFound(err) {
            return fmt.Errorf("创建网络%s失败: %w", name, err)
        }
    }
    return nil
}

// 部署服务
func (c *Client) deployService(ctx context.Context, name string, service ServiceConfig) error {
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
    }

    // 创建容器配置
    hostConfig := &container.HostConfig{
        RestartPolicy: container.RestartPolicy{
            Name: service.Restart,  // 直接使用字符串，不需要类型转换
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
    cli, err := client.NewClientWithOpts(
        client.WithHost("unix:///var/run/docker.sock"),
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, err
    }
    return &Client{cli}, nil
}

// 关闭Client
func (cli *Client) Close() error {
    return cli.Client.Close()
}