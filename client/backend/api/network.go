package api

import (
	"context"
	"dockerpanel/backend/pkg/docker"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/gin-gonic/gin"
)

func RegisterNetworkRoutes(r *gin.RouterGroup) {
	group := r.Group("/networks")
	{
		group.GET("", listNetworks)
		group.POST("", createNetwork)
		group.PUT("/:id", updateNetwork)
		group.DELETE("/:id", removeNetwork)
		group.POST("/bridge/enable-ipv6", enableDefaultBridgeIPv6)
		group.POST("/prune", pruneNetworks)
	}
}

func pruneNetworks(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	report, err := cli.NetworksPrune(context.Background(), filters.Args{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "已清理未使用的网络",
		"deletedNetworks": report.NetworksDeleted,
	})
}

func listNetworks(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 获取详细的网络信息
	networks, err := cli.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取每个网络的详细信息
	for i, network := range networks {
		networkDetail, err := cli.NetworkInspect(context.Background(), network.ID, types.NetworkInspectOptions{})
		if err != nil {
			continue
		}
		networks[i] = networkDetail
	}

	c.JSON(http.StatusOK, networks)
}

func createNetwork(c *gin.Context) {
	var req struct {
		Name        string            `json:"name" binding:"required"`
		Driver      string            `json:"driver" binding:"required"`
		IPv4Subnet  string            `json:"ipv4Subnet"`
		IPv4Gateway string            `json:"ipv4Gateway"`
		IPv6Subnet  string            `json:"ipv6Subnet"`
		IPv6Gateway string            `json:"ipv6Gateway"`
		EnableIPv6  bool              `json:"enableIPv6"`
		Options     map[string]string `json:"options"`
		Parent      string            `json:"parent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 重名检查（针对 bridge）
	if req.Driver == "bridge" {
		existing, _ := cli.NetworkList(context.Background(), types.NetworkListOptions{})
		for _, n := range existing {
			if n.Name == req.Name && n.Driver == "bridge" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "同名的 bridge 网络已存在"})
				return
			}
		}
	}

	ipamConfigs := []networktypes.IPAMConfig{}
	if req.IPv4Subnet != "" {
		ipamConfigs = append(ipamConfigs, networktypes.IPAMConfig{
			Subnet:  req.IPv4Subnet,
			Gateway: req.IPv4Gateway,
		})
	}
	if req.EnableIPv6 && req.IPv6Subnet != "" {
		ipamConfigs = append(ipamConfigs, networktypes.IPAMConfig{
			Subnet:  req.IPv6Subnet,
			Gateway: req.IPv6Gateway,
		})
	}

	ipam := &networktypes.IPAM{
		Driver: "default",
		Config: ipamConfigs,
	}

	createOpts := types.NetworkCreate{
		Driver:     req.Driver,
		EnableIPv6: req.EnableIPv6,
		IPAM:       ipam,
		Options:    map[string]string{},
	}

	// macvlan 特定配置
	if req.Driver == "macvlan" {
		if req.Parent != "" {
			createOpts.Options["parent"] = req.Parent
		}
		// 允许用户传入其他选项
		for k, v := range req.Options {
			createOpts.Options[k] = v
		}
	}

	resp, err := cli.NetworkCreate(context.Background(), req.Name, createOpts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func removeNetwork(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")

	// 获取网络信息
	network, err := cli.NetworkInspect(context.Background(), id, types.NetworkInspectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 检查是否为默认网络
	if network.Name == "bridge" || network.Name == "host" || network.Name == "none" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除默认网络"})
		return
	}

	if err := cli.NetworkRemove(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "网络已删除"})
}

func enableDefaultBridgeIPv6(c *gin.Context) {
	var req struct {
		FixedCIDRv6 string `json:"fixedCIDRv6"`
	}
	_ = c.ShouldBindJSON(&req)
	cfg := &docker.DaemonConfig{
		IPv6:        true,
		FixedCIDRv6: req.FixedCIDRv6,
	}
	if err := docker.UpdateDaemonConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已启用默认 bridge IPv6，请重启 Docker 服务"})
}

func updateNetwork(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		IPv4Subnet  string            `json:"ipv4Subnet"`
		IPv4Gateway string            `json:"ipv4Gateway"`
		IPv6Subnet  string            `json:"ipv6Subnet"`
		IPv6Gateway string            `json:"ipv6Gateway"`
		EnableIPv6  bool              `json:"enableIPv6"`
		Options     map[string]string `json:"options"`
		Parent      string            `json:"parent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	network, err := cli.NetworkInspect(context.Background(), id, types.NetworkInspectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 禁止修改默认网络
	if network.Name == "bridge" || network.Name == "host" || network.Name == "none" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改默认网络"})
		return
	}

	// 若网络仍连接容器，提示先断开
	if len(network.Containers) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该网络已连接容器，无法修改，请先断开所有容器"})
		return
	}

	// 删除旧网络
	if err := cli.NetworkRemove(context.Background(), network.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除旧网络失败: " + err.Error()})
		return
	}

	// 重新创建网络，保留原名称与驱动
	ipamConfigs := []networktypes.IPAMConfig{}
	if req.IPv4Subnet != "" {
		ipamConfigs = append(ipamConfigs, networktypes.IPAMConfig{
			Subnet:  req.IPv4Subnet,
			Gateway: req.IPv4Gateway,
		})
	}
	if req.EnableIPv6 && req.IPv6Subnet != "" {
		ipamConfigs = append(ipamConfigs, networktypes.IPAMConfig{
			Subnet:  req.IPv6Subnet,
			Gateway: req.IPv6Gateway,
		})
	}
	ipam := &networktypes.IPAM{
		Driver: "default",
		Config: ipamConfigs,
	}

	createOpts := types.NetworkCreate{
		Driver:     network.Driver,
		EnableIPv6: req.EnableIPv6,
		IPAM:       ipam,
		Options:    map[string]string{},
	}
	if network.Driver == "macvlan" {
		if req.Parent != "" {
			createOpts.Options["parent"] = req.Parent
		}
		for k, v := range req.Options {
			createOpts.Options[k] = v
		}
	}

	resp, err := cli.NetworkCreate(context.Background(), network.Name, createOpts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建新网络失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
