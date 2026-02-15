package api

import (
	"dockerpanel/backend/pkg/database"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterMCPRoutes(r *gin.RouterGroup) {
	group := r.Group("/mcp")
	{
		group.GET("/icons/clay", listClayIcons)
		group.POST("/navigation/:id/icon", setNavigationIcon)
	}
}

type clayIconItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func listClayIcons(c *gin.Context) {
	dir := ""
	for _, cand := range []string{
		filepath.Join(".", "dist", "icons", "clay"),
		filepath.Join(".", "icons", "clay"),
		filepath.Join("..", "frontend", "public", "icons", "clay"),
	} {
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			dir = cand
			break
		}
	}
	if dir == "" {
		c.JSON(http.StatusOK, gin.H{"items": []clayIconItem{}})
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to read icons directory", err)
		return
	}

	items := make([]clayIconItem, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".ico", ".avif", ".bmp", ".tif", ".tiff":
		default:
			continue
		}
		items = append(items, clayIconItem{Name: name, Value: "/icons/clay/" + name})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type setNavigationIconRequest struct {
	Icon string `json:"icon" binding:"required"`
}

func setNavigationIcon(c *gin.Context) {
	idRaw := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idRaw)
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "Invalid id", err)
		return
	}

	var req setNavigationIconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	icon := strings.TrimSpace(req.Icon)
	if !isAllowedNavigationIconValue(icon) {
		respondError(c, http.StatusBadRequest, "Icon value is not allowed", nil)
		return
	}

	db := database.GetDB()
	result, err := db.Exec("UPDATE navigation_items SET icon = ?, ai_generated = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", icon, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to update icon", err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondError(c, http.StatusNotFound, "Navigation item not found", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func isAllowedNavigationIconValue(icon string) bool {
	v := strings.TrimSpace(icon)
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "mdi-") {
		return true
	}
	if strings.HasPrefix(v, "/icons/clay/") {
		return true
	}
	if strings.HasPrefix(v, "/data/pic/") || strings.HasPrefix(v, "/uploads/icons/") {
		return true
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return true
	}
	return false
}
