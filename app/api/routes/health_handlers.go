package routes

import (
	"anubis/app/core/schemes"
	"github.com/gin-gonic/gin"
	"net/http"
)

// NOTE: The blank references below are only to help swag (swagger generator) resolve types
// used in annotations without impacting runtime behavior.
var (
	_ = schemes.HTTPError{}
)

// PingHandler godoc
// @Summary Ping
// @Description Простой health-check эндпоинт.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /ping [get]
func PingHandler(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"version": version,
		})
	}
}

// PingAuthHandler godoc
// @Summary Ping (JWT защищённый)
// @Description Health-check эндпоинт, требующий валидный access token.
// @Tags Health
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} schemes.HTTPError
// @Router /ping_auth [get]
func PingAuthHandler(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"version": version,
		})
	}
}
