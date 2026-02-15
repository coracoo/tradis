package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const KVAdminAllowlist = "admin_allowlist"
const KVMCPAllowlist = "mcp_allowlist"
const KVMCPToken = "mcp_token"

func GetAllowlistHandler(db *gorm.DB, key string, store *IPAllowlist, envFallback string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := store.Raw()
		source := "memory"
		if strings.TrimSpace(raw) == "" && strings.TrimSpace(envFallback) != "" {
			raw = strings.TrimSpace(envFallback)
			source = "env"
		}

		c.JSON(http.StatusOK, gin.H{
			"key":    key,
			"raw":    raw,
			"source": source,
		})
	}
}

func UpdateAllowlistHandler(db *gorm.DB, key string, store *IPAllowlist) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Raw string `json:"raw"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "无效的请求数据", err)
			return
		}
		raw := strings.TrimSpace(req.Raw)
		notes := store.Set(raw)
		if err := SetKV(db, key, raw); err != nil {
			respondError(c, http.StatusInternalServerError, "保存失败", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"key":   key,
			"raw":   store.Raw(),
			"notes": notes,
		})
	}
}

func GetMCPTokenHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, _, err := GetKV(db, KVMCPToken)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "读取失败", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"token": strings.TrimSpace(v),
		})
	}
}

func UpdateMCPTokenHandler(db *gorm.DB, onUpdate func(string)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Token string `json:"token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "无效的请求数据", err)
			return
		}
		token := strings.TrimSpace(req.Token)
		if len(token) > 256 {
			respondError(c, http.StatusBadRequest, "token 过长", fmt.Errorf("len=%d", len(token)))
			return
		}
		if err := SetKV(db, KVMCPToken, token); err != nil {
			respondError(c, http.StatusInternalServerError, "保存失败", err)
			return
		}
		if onUpdate != nil {
			onUpdate(token)
		}
		c.JSON(http.StatusOK, gin.H{
			"token": token,
		})
	}
}
