package middlewares

import (
	"anubis/app/core"
	"anubis/app/core/schemes"
	"anubis/internal/utils"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

const (
	UserIDKey = "user-id"
)

func JwtAuthMiddleware(config core.ServiceConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// IMPORTANT (compatibility):
		// - Prefer Authorization: Bearer <token> (standard).
		// - Keep backward compatibility with cookie-based token ("access_token" / legacy "Authorization").
		authToken := extractTokenFromRequest(c, "access_token")

		if authToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, schemes.HTTPError{
				Code: 99,
				Err:  "Unable to find token",
			})
			return
		}

		// Validate + parse token in one pass (signature + exp + algorithm checks).
		var authClaims schemes.JwtCustomClaims
		if err := utils.ExtractToken(authToken, config.JWT.AccessTokenSecret, &authClaims, "", make([]string, 0)); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, schemes.HTTPError{
				Code: 97,
				Err:  "BAD token",
			})
			return
		}

		c.Set(UserIDKey, authClaims.RegisteredClaims.Subject)
		c.Request.Header.Set(UserIDKey, authClaims.RegisteredClaims.Subject)
		c.Next()
		return
	}
}

func extractTokenFromRequest(c *gin.Context, preferredCookieName string) string {
	// 1) Standard: Authorization: Bearer <token>
	h := strings.TrimSpace(c.GetHeader("Authorization"))
	if h != "" {
		if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			return strings.TrimSpace(h[7:])
		}
		// Backward compat: allow raw token in Authorization header
		return h
	}

	// 2) Cookie: preferred cookie name first
	if preferredCookieName != "" {
		if v, err := c.Cookie(preferredCookieName); err == nil {
			return strings.TrimSpace(v)
		}
	}

	return ""
}
