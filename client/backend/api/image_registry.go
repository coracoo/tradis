package api

import (
	"dockerpanel/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func RegisterImageRegistryRoutes(r *gin.RouterGroup) {
	group := r.Group("/image-registry") // 修改路由路径
	{
		group.GET("", getImageRegistries)
		group.POST("", updateImageRegistries)
	}
}

// 获取所有镜像注册表配置
func getImageRegistries(c *gin.Context) {
	registries, err := database.GetAllRegistries()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取镜像注册表配置失败", err)
		return
	}
	c.JSON(http.StatusOK, registries)
}

// 更新镜像注册表配置
func updateImageRegistries(c *gin.Context) {
	var registries map[string]database.Registry
	if err := c.ShouldBindJSON(&registries); err != nil {
		respondError(c, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 打印接收到的注册表配置
	log.Printf("接收到 %d 个注册表配置", len(registries))
	for key, registry := range registries {
		log.Printf("接收到注册表配置: key=%s, name=%s, url=%s", key, registry.Name, registry.URL)
	}

	// 清除现有配置
	if err := database.ClearRegistries(); err != nil {
		respondError(c, http.StatusInternalServerError, "清除镜像注册表配置失败", err)
		return
	}

	// 保存新配置
	for key, registry := range registries {
		// 确保 URL 不为空
		if registry.URL == "" {
			registry.URL = key // 如果 URL 为空，使用键作为 URL
			log.Printf("注册表 URL 为空，使用键作为 URL: %s", key)
		}

		registry.IsDefault = (key == "docker.io")
		log.Printf("保存注册表: key=%s, name=%s, url=%s, isDefault=%v",
			key, registry.Name, registry.URL, registry.IsDefault)

		if err := database.SaveRegistry(&registry); err != nil {
			log.Printf("保存注册表失败: %v", err)
			respondError(c, http.StatusInternalServerError, "保存镜像注册表配置失败", err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "镜像注册表配置已更新"})
}
