package api

import (
	"context"
	"dockerpanel/backend/pkg/docker"
	"fmt"
	"net/http"
	"log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/gin-gonic/gin"
)

func RegisterVolumeRoutes(r *gin.RouterGroup) {
    group := r.Group("/volumes")
    {
        group.GET("", listVolumes)
        group.POST("", createVolume)
        group.DELETE("/:name", removeVolume)
        group.POST("/prune", pruneVolumes)  // 添加新路由
    }
}

// 添加清除无用卷的处理函数
func pruneVolumes(c *gin.Context) {
    cli, err := docker.NewDockerClient()
    if err != nil {
        log.Printf("创建 Docker 客户端失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cli.Close()

    log.Printf("开始清理无用卷")

    // 获取清理前的卷列表和使用状态
    beforeVolumes, err := cli.VolumeList(context.Background(), volume.ListOptions{})
    if err != nil {
        log.Printf("获取清理前卷列表失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取卷列表失败: %v", err)})
        return
    }

    // 获取容器列表以检查卷的使用状态
    containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
    if err != nil {
        log.Printf("获取容器列表失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取容器列表失败: %v", err)})
        return
    }

    log.Printf("当前容器数量: %d", len(containers))
    
    // 创建清理报告
    pruneReport := types.VolumesPruneReport{
        VolumesDeleted: []string{},
        SpaceReclaimed: 0,
    }
    
    // 手动检查并删除未使用的卷
    for _, vol := range beforeVolumes.Volumes {
        inUse := false
        for _, container := range containers {
            for _, mount := range container.Mounts {
                if mount.Type == "volume" && mount.Name == vol.Name {
                    inUse = true
                    log.Printf("卷 %s 正在被容器 %s 使用", vol.Name, container.Names[0])
                    break
                }
            }
            if inUse {
                break
            }
        }
        
        if !inUse {
            log.Printf("卷 %s 未被使用，尝试删除", vol.Name)
            if removeErr := cli.VolumeRemove(context.Background(), vol.Name, true); removeErr == nil {
                pruneReport.VolumesDeleted = append(pruneReport.VolumesDeleted, vol.Name)
                log.Printf("成功删除卷 %s", vol.Name)
                // 由于 Docker API 不提供卷大小信息，这里使用一个估计值
                pruneReport.SpaceReclaimed += 1024 * 1024 // 假设每个卷至少 1MB
            } else {
                log.Printf("删除卷 %s 失败: %v", vol.Name, removeErr)
            }
        }
    }

    // 获取清理后的卷列表
    afterVolumes, err := cli.VolumeList(context.Background(), volume.ListOptions{})
    if err != nil {
        log.Printf("获取清理后卷列表失败: %v", err)
    } else {
        log.Printf("清理后卷列表: %v", getVolumeNames(afterVolumes.Volumes))
    }

    if len(pruneReport.VolumesDeleted) == 0 {
        log.Printf("没有可清除的无用卷")
        c.JSON(http.StatusOK, gin.H{
            "message": "没有可清除的无用存储卷",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": fmt.Sprintf("已清除 %d 个存储卷，释放空间 %d bytes", 
            len(pruneReport.VolumesDeleted), 
            pruneReport.SpaceReclaimed),
        "deletedVolumes": pruneReport.VolumesDeleted,
        "spaceReclaimed": pruneReport.SpaceReclaimed,
    })
}

// 辅助函数：获取卷名列表
func getVolumeNames(volumes []*volume.Volume) []string {
    names := make([]string, len(volumes))
    for i, v := range volumes {
        names[i] = v.Name
    }
    return names
}

// 定义一个自定义的卷信息结构体，包含容器使用信息
type VolumeInfo struct {
	*volume.Volume
	InUse      bool                    `json:"InUse"`
	Containers map[string]ContainerRef `json:"Containers"`
}

// 容器引用信息
type ContainerRef struct {
	Name string `json:"Name"`
}

func listVolumes(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 获取所有容器信息
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取卷列表
	volumeList, err := cli.VolumeList(context.Background(), volume.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建增强的卷信息列表
	enhancedVolumes := make([]*VolumeInfo, 0, len(volumeList.Volumes))
	
	// 更新卷的使用信息
	for _, vol := range volumeList.Volumes {
		volumeInfo := &VolumeInfo{
			Volume:     vol,
			InUse:      false,
			Containers: make(map[string]ContainerRef),
		}
		
		// 检查每个容器是否使用了该卷
		for _, container := range containers {
			for _, mount := range container.Mounts {
				if mount.Type == "volume" && mount.Name == vol.Name {
					volumeInfo.InUse = true
					volumeInfo.Containers[container.ID] = ContainerRef{
						Name: container.Names[0],
					}
				}
			}
		}
		
		enhancedVolumes = append(enhancedVolumes, volumeInfo)
	}

	// 返回自定义结构的响应
	c.JSON(http.StatusOK, gin.H{
		"Volumes": enhancedVolumes,
		"Warnings": volumeList.Warnings,
	})
}

func createVolume(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
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

	vol, err := cli.VolumeCreate(context.Background(), volume.CreateOptions{
		Name: req.Name,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, vol)
}

func removeVolume(c *gin.Context) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	name := c.Param("name")
	if err := cli.VolumeRemove(context.Background(), name, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "数据卷已删除"})
}