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
    LanUrl            string `json:"lanUrl"`
    WanUrl            string `json:"wanUrl"`
    AppStoreServerUrl string `json:"appStoreServerUrl"`
	AllocPortStart    int    `json:"allocPortStart"`
	AllocPortEnd      int    `json:"allocPortEnd"`
}

func updateGlobalSettings(c *gin.Context) {
	log.Println("Received updateGlobalSettings request")
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("Bound request: %+v", req)

    err := settings.UpdateSettings(settings.Settings{
        LanUrl:            req.LanUrl,
        WanUrl:            req.WanUrl,
        AppStoreServerUrl: req.AppStoreServerUrl,
		AllocPortStart:    req.AllocPortStart,
		AllocPortEnd:      req.AllocPortEnd,
    })
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings: " + err.Error()})
		return
	}

	// 触发容器自动发现以更新导航项的 URL
	go system.ProcessContainerDiscovery()

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}
