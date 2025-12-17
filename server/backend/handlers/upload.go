package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "文件上传失败"})
		return
	}

	_ = os.MkdirAll("data/uploads", 0755)

	project := strings.TrimSpace(c.PostForm("project"))
	ftype := strings.TrimSpace(c.PostForm("type"))
	indexStr := strings.TrimSpace(c.PostForm("index"))

	ext := strings.ToLower(filepath.Ext(file.Filename))
	base := strings.TrimSuffix(file.Filename, ext)

	var targetName string
	if project != "" && ftype != "" {
		switch ftype {
		case "icon":
			targetName = fmt.Sprintf("%s_icon%s", project, ext)
		case "screenshot":
			num := 1
			if indexStr != "" {
				fmt.Sscanf(indexStr, "%d", &num)
			}
			targetName = fmt.Sprintf("%s_Screenshot_%d%s", project, num, ext)
		default:
			targetName = base + ext
		}
	} else {
		targetName = base + ext
	}

	// 防止名称冲突，存在则覆盖
	savePath := filepath.Join("data", "uploads", targetName)
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(500, gin.H{"error": "文件保存失败"})
		return
	}

	c.JSON(200, gin.H{
		"url":  "/uploads/" + targetName,
		"name": targetName,
	})
}
