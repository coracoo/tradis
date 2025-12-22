package handlers

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Variable struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Category    string `json:"category"`
	ServiceName string `json:"serviceName"`
	ParamType   string `json:"paramType"` // port, path, env, hardware, other
}

type Variables []Variable

// Value 实现 driver.Valuer 接口
func (v Variables) Value() (driver.Value, error) {
	if len(v) == 0 {
		return "[]", nil
	}
	return json.Marshal(v)
}

// Scan 实现 sql.Scanner 接口
func (v *Variables) Scan(value interface{}) error {
	if value == nil {
		*v = make(Variables, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		// 尝试处理字符串类型
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to unmarshal JSONB value")
		}
		bytes = []byte(str)
	}

	return json.Unmarshal(bytes, v)
}

type StringArray []string

// Value 实现 driver.Valuer 接口
func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner 接口
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = make(StringArray, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to unmarshal JSONB value")
		}
		bytes = []byte(str)
	}

	return json.Unmarshal(bytes, s)
}

type Template struct {
	ID          uint              `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DeletedAt   gorm.DeletedAt    `gorm:"index" json:"deleted_at"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Website     string            `json:"website"`
	Logo        string            `json:"logo"`
	Tutorial    string            `json:"tutorial"`
	Dotenv      string            `json:"dotenv" gorm:"type:text"`
	DotenvJSON  map[string]string `json:"dotenv_json,omitempty" gorm:"-"`
	Compose     string            `json:"compose"`
	Screenshots StringArray       `json:"screenshots" gorm:"type:text"`
	Schema      Variables         `json:"schema" gorm:"type:text"`
	Enabled     bool              `json:"enabled" gorm:"default:true"`
}

func parseDotenvToMap(content string) map[string]string {
	out := make(map[string]string)
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}

		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		out[key] = val
	}
	return out
}

func ListTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var templates []Template
		if err := db.Find(&templates).Error; err != nil {
			c.JSON(500, gin.H{"error": "获取模板列表失败"})
			return
		}
		for i := range templates {
			templates[i].DotenvJSON = parseDotenvToMap(templates[i].Dotenv)
		}
		c.JSON(200, templates)
	}
}

func GetTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var template Template
		idOrName := c.Param("id")

		var result *gorm.DB
		// 简单的数字检查 (或者使用 strconv.Atoi)
		if isNumeric(idOrName) {
			result = db.First(&template, idOrName)
		} else {
			result = db.Where("name = ?", idOrName).First(&template)
		}

		if result.Error != nil {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}
		template.DotenvJSON = parseDotenvToMap(template.Dotenv)
		c.JSON(200, template)
	}
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func CreateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var template Template
		if err := c.ShouldBindJSON(&template); err != nil {
			c.JSON(400, gin.H{"error": "无效的请求数据"})
			return
		}

		if err := db.Create(&template).Error; err != nil {
			c.JSON(500, gin.H{"error": "创建模板失败"})
			return
		}
		c.JSON(201, template)
	}
}

func UpdateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var existingTemplate Template
		if err := db.First(&existingTemplate, c.Param("id")).Error; err != nil {
			c.JSON(404, gin.H{"error": "模板不存在"})
			return
		}

		var input Template
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(400, gin.H{"error": "无效的请求数据"})
			return
		}

		// 更新字段
		existingTemplate.Name = input.Name
		existingTemplate.Category = input.Category
		existingTemplate.Description = input.Description
		existingTemplate.Version = input.Version
		existingTemplate.Website = input.Website
		existingTemplate.Logo = input.Logo
		existingTemplate.Tutorial = input.Tutorial
		existingTemplate.Dotenv = input.Dotenv
		existingTemplate.Compose = input.Compose
		existingTemplate.Screenshots = input.Screenshots
		existingTemplate.Schema = input.Schema
		existingTemplate.Enabled = input.Enabled

		if err := db.Save(&existingTemplate).Error; err != nil {
			c.JSON(500, gin.H{"error": "更新模板失败"})
			return
		}
		existingTemplate.DotenvJSON = parseDotenvToMap(existingTemplate.Dotenv)
		c.JSON(200, existingTemplate)
	}
}

func DeleteTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.Delete(&Template{}, c.Param("id")).Error; err != nil {
			c.JSON(500, gin.H{"error": "删除模板失败"})
			return
		}
		c.JSON(200, gin.H{"message": "删除成功"})
	}
}
