package api

import (
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func init() {
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("default_secret_key_change_me")
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func RegisterAuthRoutes(r *gin.Engine) {
	r.POST("/api/auth/login", login)

	authGroup := r.Group("/api/auth")
	authGroup.Use(AuthMiddleware())
	{
		authGroup.POST("/change-password", changePassword)
		authGroup.GET("/me", getCurrentUser)
	}
}

func login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	db := database.GetDB()
	var storedPassword string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&storedPassword)

	if err == sql.ErrNoRows {
		respondError(c, http.StatusUnauthorized, "Invalid username or password", nil)
		return
	} else if err != nil {
		respondError(c, http.StatusInternalServerError, "Database error", err)
		return
	}

	// Debug: 观测密码存储形态（前缀与长度）
	func() {
		if storedPassword == "" {
			log.Printf("[AUTH] user=%s no password stored", req.Username)
		} else {
			prefix := storedPassword
			if len(prefix) > 7 {
				prefix = prefix[:7]
			}
			log.Printf("[AUTH] user=%s stored pw len=%d prefix=%s", req.Username, len(storedPassword), prefix)
		}
	}()

	// 使用 bcrypt 校验密码；若不是哈希（兼容旧数据），进行明文比较并升级为哈希
	if strings.HasPrefix(storedPassword, "$2a$") || strings.HasPrefix(storedPassword, "$2b$") || strings.HasPrefix(storedPassword, "$2y$") {
		if bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(req.Password)) != nil {
			respondError(c, http.StatusUnauthorized, "Invalid username or password", nil)
			return
		}
	} else {
		// 旧明文密码
		if storedPassword != req.Password {
			respondError(c, http.StatusUnauthorized, "Invalid username or password", nil)
			return
		}
		// 升级为哈希
		newHash, herr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if herr == nil {
			_, _ = db.Exec("UPDATE users SET password = ? WHERE username = ?", string(newHash), req.Username)
		}
	}

	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": req.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Could not generate token", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func changePassword(c *gin.Context) {
	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	username := c.GetString("username")
	db := database.GetDB()

	var currentPassword string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&currentPassword)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Database error", err)
		return
	}

	// 验证旧密码
	if bcrypt.CompareHashAndPassword([]byte(currentPassword), []byte(req.OldPassword)) != nil {
		respondError(c, http.StatusBadRequest, "Old password incorrect", nil)
		return
	}

	// 哈希新密码并更新
	newHash, herr := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if herr != nil {
		respondError(c, http.StatusInternalServerError, "Failed to hash new password", herr)
		return
	}
	_, err = db.Exec("UPDATE users SET password = ? WHERE username = ?", string(newHash), username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to update password", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func getCurrentUser(c *gin.Context) {
	username := c.GetString("username")
	c.JSON(http.StatusOK, gin.H{"username": username})
}

// AuthMiddleware validates JWT token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		// 尝试从 Query 参数获取 Token，以支持 SSE 等无法设置 Header 的场景
		if tokenString == "" {
			tokenString = c.Query("token")
		}
		if tokenString == "" {
			if cookieToken, err := c.Cookie("token"); err == nil {
				tokenString = cookieToken
			}
		}

		if tokenString == "" {
			respondError(c, http.StatusUnauthorized, "Authorization header required", nil)
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			respondError(c, http.StatusUnauthorized, "Invalid token", nil)
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			respondError(c, http.StatusUnauthorized, "Invalid token claims", nil)
			c.Abort()
			return
		}

		username, ok := claims["username"].(string)
		if !ok {
			respondError(c, http.StatusUnauthorized, "Invalid token payload", nil)
			c.Abort()
			return
		}

		c.Set("username", username)
		c.Next()
	}
}
