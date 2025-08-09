package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MeResponse struct {
	PublicID    string  `json:"publicId"`
	DisplayName *string `json:"displayName,omitempty"`
	Email       *string `json:"email,omitempty"` // na przyszłość
}

type MeUpdateReq struct {
	DisplayName *string `json:"displayName"` // opcjonalne
	// Email *string `json:"email"` // sugeruję dodać dopiero z weryfikacją
}

type RestoreReq struct {
	PublicID string `json:"publicId"`
}

// GET /api/v1/me
func GetMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pubIDAny, _ := c.Get("userPublicID")
		pubID, _ := pubIDAny.(string)
		if pubID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		var u User
		if err := db.First(&u, "public_id = ?", pubID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, MeResponse{
			PublicID:    u.PublicID,
			DisplayName: u.DisplayName,
			Email:       u.Email,
		})
	}
}

// PUT /api/v1/me
func UpdateMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pubIDAny, _ := c.Get("userPublicID")
		pubID, _ := pubIDAny.(string)
		if pubID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		var u User
		if err := db.First(&u, "public_id = ?", pubID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		var req MeUpdateReq
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
			return
		}

		// Walidacja displayName (jeśli podano)
		if req.DisplayName != nil {
			name := strings.TrimSpace(*req.DisplayName)
			if len(name) < 2 || len(name) > 40 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "displayName must be 2..40 chars"})
				return
			}
			u.DisplayName = &name
		}

		// (Email polecam dodać dopiero z flow weryfikacji/magic linkiem)

		if err := db.Save(&u).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		c.JSON(http.StatusOK, MeResponse{
			PublicID:    u.PublicID,
			DisplayName: u.DisplayName,
			Email:       u.Email,
		})
	}
}

func ExportKey(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pubIDAny, _ := c.Get("userPublicID")
		pubID, _ := pubIDAny.(string)
		if pubID == "" {
			c.JSON(401, gin.H{"error": "no user"})
			return
		}
		c.JSON(200, gin.H{"publicId": pubID})
	}
}

func RestoreAccount(db *gorm.DB, secureCookies bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RestoreReq
		if err := c.BindJSON(&req); err != nil || strings.TrimSpace(req.PublicID) == "" {
			c.JSON(400, gin.H{"error": "publicId required"})
			return
		}
		var u User
		if err := db.First(&u, "public_id = ?", req.PublicID).Error; err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     cookieName,
			Value:    u.PublicID,
			Path:     "/",
			MaxAge:   365 * 24 * 3600,
			HttpOnly: true,
			Secure:   secureCookies, // true w prod/HTTPS
			SameSite: http.SameSiteLaxMode,
		})
		c.JSON(200, gin.H{"status": "restored"})
	}
}