package api

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type NavigationItem struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"` // Deprecated: Use LanUrl or WanUrl
	LanUrl      string    `json:"lan_url"`
	WanUrl      string    `json:"wan_url"`
	Icon        string    `json:"icon"`
	Category    string    `json:"category"`
	IsAuto      bool      `json:"is_auto"`
	IsDeleted   bool      `json:"is_deleted"`
	ContainerID string    `json:"container_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateNavigationRequest struct {
	Title    string `json:"title" binding:"required"`
	URL      string `json:"url"` // Optional now
	LanUrl   string `json:"lan_url"`
	WanUrl   string `json:"wan_url"`
	Icon     string `json:"icon"`
	Category string `json:"category"`
}

type UpdateNavigationRequest struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	LanUrl   string `json:"lan_url"`
	WanUrl   string `json:"wan_url"`
	Icon     string `json:"icon"`
	Category string `json:"category"`
}

func RegisterNavigationRoutes(r *gin.RouterGroup) {
	nav := r.Group("/navigation")
	{
		nav.GET("", listNavigationItems)
		nav.POST("", createNavigationItem)
		nav.PUT("/:id", updateNavigationItem)
		nav.DELETE("/:id", deleteNavigationItem)
		nav.POST("/:id/restore", restoreNavigationItem)
	}
}

// @Summary 获取导航项列表
// @Description 获取所有导航项，支持按分类分组
// @Tags navigation
// @Accept json
// @Produce json
// @Success 200 {array} NavigationItem
func listNavigationItems(c *gin.Context) {
	db := database.GetDB()

	includeDeleted := c.Query("include_deleted") == "true"
	query := "SELECT id, title, url, lan_url, wan_url, icon, category, is_auto, is_deleted, container_id, created_at, updated_at FROM navigation_items"
	if !includeDeleted {
		query += " WHERE is_deleted = 0"
	}
	query += " ORDER BY category, title"

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var items []NavigationItem
	for rows.Next() {
		var item NavigationItem
		var isAuto int
		var isDeleted int
		// 处理可能为 NULL 的字段
		var url, lanUrl, wanUrl sql.NullString

		if err := rows.Scan(&item.ID, &item.Title, &url, &lanUrl, &wanUrl, &item.Icon, &item.Category, &isAuto, &isDeleted, &item.ContainerID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		item.URL = url.String
		item.LanUrl = lanUrl.String
		item.WanUrl = wanUrl.String
		item.IsAuto = isAuto == 1
		item.IsDeleted = isDeleted == 1
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

// @Summary 创建导航项
// @Description 创建一个新的手动导航项
// @Tags navigation
// @Accept json
// @Produce json
// @Success 201 {object} NavigationItem
func createNavigationItem(c *gin.Context) {
	var req CreateNavigationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()
	result, err := db.Exec(
		"INSERT INTO navigation_items (title, url, lan_url, wan_url, icon, category, is_auto) VALUES (?, ?, ?, ?, ?, ?, 0)",
		req.Title, req.URL, req.LanUrl, req.WanUrl, req.Icon, req.Category,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()

	// Fetch the created item
	var item NavigationItem
	var isAuto int
	var isDeleted int
	var url, lanUrl, wanUrl sql.NullString
	err = db.QueryRow("SELECT id, title, url, lan_url, wan_url, icon, category, is_auto, is_deleted, container_id, created_at, updated_at FROM navigation_items WHERE id = ?", id).
		Scan(&item.ID, &item.Title, &url, &lanUrl, &wanUrl, &item.Icon, &item.Category, &isAuto, &isDeleted, &item.ContainerID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	item.URL = url.String
	item.LanUrl = lanUrl.String
	item.WanUrl = wanUrl.String
	item.IsAuto = isAuto == 1
	item.IsDeleted = isDeleted == 1

	c.JSON(http.StatusCreated, item)
}

// @Summary 更新导航项
// @Description 更新现有的导航项
// @Tags navigation
// @Accept json
// @Produce json
// @Success 200 {object} NavigationItem
func updateNavigationItem(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNavigationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()
	// 注意：这里我们更新所有字段，如果前端传空字符串，也会被更新进去。
	// 根据需求，用户可能想清空某个 URL，所以这是合理的。
	_, err := db.Exec(
		"UPDATE navigation_items SET title = ?, url = ?, lan_url = ?, wan_url = ?, icon = ?, category = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.Title, req.URL, req.LanUrl, req.WanUrl, req.Icon, req.Category, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch the updated item
	var item NavigationItem
	var isAuto int
	var isDeleted int
	var url, lanUrl, wanUrl sql.NullString
	err = db.QueryRow("SELECT id, title, url, lan_url, wan_url, icon, category, is_auto, is_deleted, container_id, created_at, updated_at FROM navigation_items WHERE id = ?", id).
		Scan(&item.ID, &item.Title, &url, &lanUrl, &wanUrl, &item.Icon, &item.Category, &isAuto, &isDeleted, &item.ContainerID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	item.URL = url.String
	item.LanUrl = lanUrl.String
	item.WanUrl = wanUrl.String
	item.IsAuto = isAuto == 1
	item.IsDeleted = isDeleted == 1

	c.JSON(http.StatusOK, item)
}

// @Summary 删除导航项
// @Description 删除指定的导航项 (软删除)
// @Tags navigation
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
func deleteNavigationItem(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	result, err := db.Exec("UPDATE navigation_items SET is_deleted = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "导航项不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "导航项已移至回收站"})
}

// @Summary 恢复导航项
// @Description 恢复已删除的导航项
// @Tags navigation
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
func restoreNavigationItem(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	result, err := db.Exec("UPDATE navigation_items SET is_deleted = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "导航项不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "导航项已恢复"})
}
