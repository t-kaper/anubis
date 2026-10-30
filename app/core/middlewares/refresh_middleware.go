package middlewares

import (
	"anubis/app/core"
	"anubis/app/core/schemes"
	"anubis/internal/utils"
	"github.com/gin-gonic/gin"
	"net/http"
)

const (
	SessionIDKey = "session-id"
	HashToken    = "hash-token"
)

func RefreshAuthMiddleware(config core.ServiceConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// IMPORTANT (compatibility):
		// - Prefer Authorization: Bearer <token> (standard).
		// - Keep backward compatibility with cookie-based token ("refresh_token" / legacy "Authorization").
		refreshToken := extractTokenFromRequest(c, "refresh_token")

		if refreshToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, schemes.HTTPError{
				Code: 99,
				Err:  "Unable to find token",
			})
			return
		}

		// Validate + parse token in one pass (signature + exp + algorithm checks).
		var token schemes.JwtCustomRefreshClaims
		if err := utils.ExtractToken(refreshToken, config.JWT.RefreshTokenSecret, &token, "", make([]string, 0)); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, schemes.HTTPError{
				Code: 97,
				Err:  "BAD token",
			})
			return
		}

		c.Set(HashToken, utils.HashToken(refreshToken))
		c.Set(SessionIDKey, token.ID)
		c.Next()
		return
	}
}
