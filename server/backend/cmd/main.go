package main

import (
	"dockerpanel/server/backend/handlers"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	_ = os.MkdirAll("data/uploads", 0755)

	db, err := gorm.Open(sqlite.Open("data/templates.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	db.AutoMigrate(&handlers.Template{})

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))

	r.Static("/uploads", "./data/uploads")

	api := r.Group("/api")
	{
		api.GET("/templates", handlers.ListTemplates(db))
		api.GET("/templates/:id", handlers.GetTemplate(db))
		api.POST("/templates", handlers.CreateTemplate(db))
		api.PUT("/templates/:id", handlers.UpdateTemplate(db))
		api.DELETE("/templates/:id", handlers.DeleteTemplate(db))
		api.POST("/upload", handlers.UploadFile)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}
	r.Run(":" + port)
}
