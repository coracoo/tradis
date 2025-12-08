package api

import (
    "context"
    "dockerpanel/backend/pkg/docker"
    "net/http"

    "github.com/docker/docker/api/types"  // 保留需要的导入
    "github.com/gin-gonic/gin"
)

func RegisterNetworkRoutes(r *gin.RouterGroup) {
	group := r.Group("/networks")
	{
		group.GET("", listNetworks)
		group.POST("", createNetwork)
		group.DELETE("/:id", removeNetwork)
	}
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
		Name   string `json:"name" binding:"required"`
		Driver string `json:"driver" binding:"required"`
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

	resp, err := cli.NetworkCreate(context.Background(), req.Name, types.NetworkCreate{
		Driver: req.Driver,
	})
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