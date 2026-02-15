package api

import (
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/system"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"math/big"
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
		respondError(c, http.StatusInternalServerError, "Failed to get settings", err)
		return
	}
	c.JSON(http.StatusOK, s)
}

type UpdateSettingsRequest struct {
	LanUrl                      string   `json:"lanUrl"`
	WanUrl                      string   `json:"wanUrl"`
	AppStoreServerUrl           string   `json:"appStoreServerUrl"`
	AllocPortStart              int      `json:"allocPortStart"`
	AllocPortEnd                int      `json:"allocPortEnd"`
	AllowAutoAllocPort          bool     `json:"allowAutoAllocPort"`
	ImageUpdateIntervalMinutes  int      `json:"imageUpdateIntervalMinutes"`
	AiEnabled                   *bool    `json:"aiEnabled"`
	AiBaseUrl                   *string  `json:"aiBaseUrl"`
	AiApiKey                    *string  `json:"aiApiKey"`
	AiModel                     *string  `json:"aiModel"`
	AiTemperature               *float64 `json:"aiTemperature"`
	AiPrompt                    *string  `json:"aiPrompt"`
	VolumeBackupEnabled         *bool    `json:"volumeBackupEnabled"`
	VolumeBackupImage           *string  `json:"volumeBackupImage"`
	VolumeBackupEnv             *string  `json:"volumeBackupEnv"`
	VolumeBackupVolumes         []string `json:"volumeBackupVolumes"`
	VolumeBackupArchiveDir      *string  `json:"volumeBackupArchiveDir"`
	VolumeBackupMountDockerSock *bool    `json:"volumeBackupMountDockerSock"`
}

func updateGlobalSettings(c *gin.Context) {
	if settings.IsDebugEnabled() {
		log.Println("Received updateGlobalSettings request")
	}
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	current, err := settings.GetSettings()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to get settings", err)
		return
	}
	if settings.IsDebugEnabled() {
		log.Printf(
			"Bound request: allocPortStart=%d allocPortEnd=%d allowAutoAllocPort=%t imageUpdateIntervalMinutes=%d appStoreServerUrl=%s aiEnabled=%v aiBaseUrl=%v aiModel=%v aiTemperature=%v",
			req.AllocPortStart,
			req.AllocPortEnd,
			req.AllowAutoAllocPort,
			req.ImageUpdateIntervalMinutes,
			settings.RedactAppStoreURL(req.AppStoreServerUrl),
			req.AiEnabled,
			req.AiBaseUrl,
			req.AiModel,
			req.AiTemperature,
		)
	}

	merged := current
	merged.LanUrl = req.LanUrl
	merged.WanUrl = req.WanUrl
	merged.AppStoreServerUrl = req.AppStoreServerUrl
	merged.AllocPortStart = req.AllocPortStart
	merged.AllocPortEnd = req.AllocPortEnd
	merged.AllowAutoAllocPort = req.AllowAutoAllocPort
	merged.ImageUpdateIntervalMinutes = req.ImageUpdateIntervalMinutes
	if req.AiEnabled != nil {
		merged.AiEnabled = *req.AiEnabled
	}
	if req.AiBaseUrl != nil {
		merged.AiBaseUrl = strings.TrimSpace(*req.AiBaseUrl)
	}
	if req.AiModel != nil {
		merged.AiModel = strings.TrimSpace(*req.AiModel)
	}
	if req.AiTemperature != nil {
		merged.AiTemperature = *req.AiTemperature
	}
	if req.AiPrompt != nil {
		merged.AiPrompt = *req.AiPrompt
	}
	if req.VolumeBackupEnabled != nil {
		merged.VolumeBackupEnabled = *req.VolumeBackupEnabled
	}
	if req.VolumeBackupImage != nil {
		merged.VolumeBackupImage = strings.TrimSpace(*req.VolumeBackupImage)
	}
	if req.VolumeBackupEnv != nil {
		merged.VolumeBackupEnv = *req.VolumeBackupEnv
	}
	if req.VolumeBackupVolumes != nil {
		merged.VolumeBackupVolumes = req.VolumeBackupVolumes
	}
	if req.VolumeBackupArchiveDir != nil {
		merged.VolumeBackupArchiveDir = strings.TrimSpace(*req.VolumeBackupArchiveDir)
	}
	if req.VolumeBackupMountDockerSock != nil {
		merged.VolumeBackupMountDockerSock = *req.VolumeBackupMountDockerSock
	}

	err = settings.UpdateSettings(merged)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to update settings", err)
		return
	}

	if req.AiApiKey != nil {
		if err := settings.SetValue("ai_api_key", strings.TrimSpace(*req.AiApiKey)); err != nil {
			respondError(c, http.StatusInternalServerError, "Failed to update AI api key", err)
			return
		}
	}

	// 触发容器自动发现以更新导航项的 URL
	go system.ProcessContainerDiscovery()
	go system.EnsureVolumeBackupContainer(merged)

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

func getKVSetting(c *gin.Context) {
	key := c.Param("key")
	val, err := settings.GetValue(key)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to get value", err)
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
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}
	if err := settings.SetValue(key, req.Value); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to set value", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

const (
	kvClientVersionKey    = "client_version"
	kvAppStoreVersionKey  = "appstore_server_version"
	kvHasNewVersionKey    = "appstore_has_new_version"
	kvVersionCheckedAtKey = "appstore_version_checked_at"
)

func InitClientVersionFromEnv() {
	v := strings.TrimSpace(os.Getenv("CLIENT_VERSION"))
	if v == "" {
		v = strings.TrimSpace(os.Getenv("DOCKPIER_CLIENT_VERSION"))
	}
	if v == "" {
		return
	}
	_ = settings.SetValue(kvClientVersionKey, v)
}

func StartVersionMonitor() {
	go func() {
		runOnce := func() {
			s, err := settings.GetSettings()
			if err != nil {
				return
			}
			base := strings.TrimRight(strings.TrimSpace(s.AppStoreServerUrl), "/")
			if base == "" {
				return
			}

			client := &http.Client{Timeout: 15 * time.Second}
			req, err := http.NewRequest(http.MethodGet, base+"/api/version", nil)
			if err != nil {
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return
			}
			var payload struct {
				ServerVersion string `json:"server_version"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return
			}
			serverVersion := strings.TrimSpace(payload.ServerVersion)
			if serverVersion == "" {
				return
			}

			_ = settings.SetValue(kvAppStoreVersionKey, serverVersion)
			_ = settings.SetValue(kvVersionCheckedAtKey, time.Now().Format(time.RFC3339))

			localVersion, _ := settings.GetValue(kvClientVersionKey)
			hasNew := compareVersionDigits(serverVersion, localVersion) > 0
			if hasNew {
				_ = settings.SetValue(kvHasNewVersionKey, "true")
			} else {
				_ = settings.SetValue(kvHasNewVersionKey, "false")
			}
		}

		runOnce()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			runOnce()
		}
	}()
}

func compareVersionDigits(a, b string) int {
	aa := extractDigitsBigInt(a)
	bb := extractDigitsBigInt(b)
	return aa.Cmp(bb)
}

var digitsRe = regexp.MustCompile(`\d+`)

func extractDigitsBigInt(raw string) *big.Int {
	parts := digitsRe.FindAllString(strings.TrimSpace(raw), -1)
	digits := strings.Join(parts, "")
	if digits == "" {
		return big.NewInt(0)
	}
	n := new(big.Int)
	if _, ok := n.SetString(digits, 10); !ok {
		return big.NewInt(0)
	}
	return n
}
