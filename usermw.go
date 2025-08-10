package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const cookieName = "sq_uid"

// EnsureUser resolves the current user using, in order:
// 1) "X-Public-Id" header (frontend fallback for cross-site scenarios),
// 2) Cookie "sq_uid",
// 3) Creates a new user if neither is provided.
//
// secureCookies should be true in production behind HTTPS (Cloud Run, etc.).
// When secureCookies is true we set SameSite=None to allow cross-site cookies
// (e.g., GH Pages -> Cloud Run).
func EnsureUser(db *gorm.DB, secureCookies bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Choose SameSite mode
		sameSite := http.SameSiteLaxMode
		if secureCookies {
			// Cross-site cookie requires None + Secure
			sameSite = http.SameSiteNoneMode
		}

		// 1) Try identity via header first (works even if browser blocks 3rd-party cookies)
		if hdr := c.GetHeader("X-Public-Id"); hdr != "" {
			var u User
			if err := db.First(&u, "public_id = ?", hdr).Error; err != nil {
				// Not found -> create user with provided public id
				u = User{PublicID: hdr}
				if err := db.Create(&u).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "user create failed"})
					c.Abort()
					return
				}
			}

			// Set context for downstream handlers
			c.Set("userPublicID", u.PublicID)
			c.Set("userDBID", u.ID)

			// Optionally also set cookie for browsers that allow it
			http.SetCookie(c.Writer, &http.Cookie{
				Name:     cookieName,
				Value:    u.PublicID,
				Path:     "/",
				MaxAge:   365 * 24 * 3600, // 1 year
				HttpOnly: true,
				Secure:   secureCookies,
				SameSite: sameSite,
			})

			c.Next()
			return
		}

		// 2) Try cookie
		if pubID, err := c.Cookie(cookieName); err == nil && pubID != "" {
			var u User
			if err := db.First(&u, "public_id = ?", pubID).Error; err != nil {
				// Cookie present but user missing -> recreate
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
			return
		}

		// 3) Create new anonymous user + set cookie
		pubID := uuid.New().String()
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
			MaxAge:   365 * 24 * 3600, // 1 year
			HttpOnly: true,
			Secure:   secureCookies,
			SameSite: sameSite,
		})

		c.Set("userPublicID", pubID)
		c.Set("userDBID", u.ID)
		c.Next()
	}
}
