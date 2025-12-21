package api

import (
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/system"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterSettingsRoutes(r *gin.RouterGroup) {
	group := r.Group("/settings")
	{
		group.GET("/global", getGlobalSettings)
		group.POST("/global", updateGlobalSettings)
		group.GET("/kv/:key", getKVSetting)
		group.POST("/kv/:key", setKVSetting)
	}
}

func getGlobalSettings(c *gin.Context) {
	s, err := settings.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get settings"})
		return
	}
	c.JSON(http.StatusOK, s)
}

type UpdateSettingsRequest struct {
	LanUrl                     string `json:"lanUrl"`
	WanUrl                     string `json:"wanUrl"`
	AppStoreServerUrl          string `json:"appStoreServerUrl"`
	AllocPortStart             int    `json:"allocPortStart"`
	AllocPortEnd               int    `json:"allocPortEnd"`
	ImageUpdateIntervalMinutes int    `json:"imageUpdateIntervalMinutes"`
}

func updateGlobalSettings(c *gin.Context) {
	if settings.IsDebugEnabled() {
		log.Println("Received updateGlobalSettings request")
	}
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if settings.IsDebugEnabled() {
		log.Printf(
			"Bound request: allocPortStart=%d allocPortEnd=%d imageUpdateIntervalMinutes=%d appStoreServerUrl=%s",
			req.AllocPortStart,
			req.AllocPortEnd,
			req.ImageUpdateIntervalMinutes,
			settings.RedactAppStoreURL(req.AppStoreServerUrl),
		)
	}

	err := settings.UpdateSettings(settings.Settings{
		LanUrl:                     req.LanUrl,
		WanUrl:                     req.WanUrl,
		AppStoreServerUrl:          req.AppStoreServerUrl,
		AllocPortStart:             req.AllocPortStart,
		AllocPortEnd:               req.AllocPortEnd,
		ImageUpdateIntervalMinutes: req.ImageUpdateIntervalMinutes,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings: " + err.Error()})
		return
	}

	// 触发容器自动发现以更新导航项的 URL
	go system.ProcessContainerDiscovery()

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

func getKVSetting(c *gin.Context) {
	key := c.Param("key")
	val, err := settings.GetValue(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get value"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "value": val})
}

type kvRequest struct {
	Value string `json:"value"`
}

func setKVSetting(c *gin.Context) {
	key := c.Param("key")
	var req kvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := settings.SetValue(key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set value"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
