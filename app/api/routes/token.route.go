package routes

import (
	"anubis/app/DAL/storage"
	"anubis/app/api/controllers"
	"anubis/app/api/usecase"
	"anubis/app/core"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouteOAuth2Auth регистрирует роуты для OAuth2.0 аутентификации
func NewRouteToken(
	pgPool *pgxpool.Pool,
	app *gin.RouterGroup,
	config core.ServiceConfig,
) {
	userRepo := storage.NewRepositoryPgUser(pgPool)
	workspaceRepo := storage.NewRepositoryPgWorkspace(pgPool, &config)
	authRepo := storage.NewRepositoryPgAuth(pgPool)

	ServiceToken := usecase.NewServiceToken(
		userRepo,
		workspaceRepo,
		authRepo,
		config,
	)

	handler := controllers.NewControllerToken(ServiceToken)

	route := app.Group("")
	route.POST("/virtual/login", handler.HandlerPOSTVirtualLogin)
	route.GET("/me", handler.HandlerGETMe)
	route.POST("/introspect", handler.HandlerPOSTIntrospect)
	route.POST("/exchange_ticket", handler.HandlerPOSTValidTicketUserFlow)
	route.POST("/refresh_token", handler.HandlerPOSTRefreshTokenDomain)
	route.POST("/revoke", handler.HandlerPOSTRevoke)
	// logout девайса (во всех доменах)
	route.POST("/logout", handler.HandlerPOSTLogoutDeviceAllDomains)

	// полный logout (все девайсы + все домены)
	route.POST("/all_logout", handler.HandlerPOSTAllLogout)
}
