package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const cookieName = "sq_uid"

// EnsureUser tworzy/odczytuje anonimowego usera przypisanego do cookie.
// secureCookies ustaw na true w produkcji (HTTPS), w dev może być false.
func EnsureUser(db *gorm.DB, secureCookies bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pubID, err := c.Cookie(cookieName)
		if err != nil || pubID == "" {
			// brak cookie → utwórz usera i ustaw cookie
			pubID = uuid.New().String()
			u := User{PublicID: pubID}
			if err := db.Create(&u).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user create failed"})
				c.Abort()
				return
			}

			http.SetCookie(c.Writer, &http.Cookie{
				Name:     cookieName,
				Value:    pubID,
				Path:     "/",
				MaxAge:   365 * 24 * 3600, // 1 rok
				HttpOnly: true,
				Secure:   secureCookies,
				SameSite: http.SameSiteLaxMode,
			})

			c.Set("userPublicID", pubID)
			c.Set("userDBID", u.ID)
			c.Next()
			return
		}

		// cookie jest → upewnij się, że user istnieje
		var u User
		if err := db.First(&u, "public_id = ?", pubID).Error; err != nil {
			u = User{PublicID: pubID}
			if err := db.Create(&u).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user recreate failed"})
				c.Abort()
				return
			}
		}

		c.Set("userPublicID", pubID)
		c.Set("userDBID", u.ID)
		c.Next()
	}
}
