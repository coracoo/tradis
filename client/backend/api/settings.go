package api

import (
	"encoding/json"
	"dockerpanel/backend/pkg/settings"
	"dockerpanel/backend/pkg/system"
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
	AllowAutoAllocPort         bool   `json:"allowAutoAllocPort"`
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
			"Bound request: allocPortStart=%d allocPortEnd=%d allowAutoAllocPort=%t imageUpdateIntervalMinutes=%d appStoreServerUrl=%s",
			req.AllocPortStart,
			req.AllocPortEnd,
			req.AllowAutoAllocPort,
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
		AllowAutoAllocPort:         req.AllowAutoAllocPort,
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

const (
	kvClientVersionKey     = "client_version"
	kvAppStoreVersionKey   = "appstore_server_version"
	kvHasNewVersionKey     = "appstore_has_new_version"
	kvVersionCheckedAtKey  = "appstore_version_checked_at"
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
