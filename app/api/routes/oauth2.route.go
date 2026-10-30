package routes

import (
	// TODO: Добавьте необходимые импорты, когда появятся соответствующие реализации
	"anubis/app/DAL/storage"
	"anubis/app/api/controllers"
	"anubis/app/api/usecase"
	"anubis/app/core"
	"anubis/app/core/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouteOAuth2Auth регистрирует роуты для OAuth2.0 аутентификации
func NewRouteOAuth2AuthPg(
	pgPool *pgxpool.Pool,
	app *gin.RouterGroup,
	config core.ServiceConfig,
) {
	userRepo := storage.NewRepositoryPgUser(pgPool)
	workspaceRepo := storage.NewRepositoryPgWorkspace(pgPool, &config)
	authRepo := storage.NewRepositoryPgAuth(pgPool)

	serviceAuth := usecase.NewServiceOAuth2Auth(
		userRepo,
		workspaceRepo,
		authRepo,
		config,
	)

	handler := controllers.NewControllerOAuth2Auth(serviceAuth)

	route := app.Group("/oauth2")
	route.Use(middlewares.CheckAuthTypeMiddleware("oauth2"))

	route.GET("/login/:provider", handler.HandlerGETOAuth2Login)
	route.GET("/callback/:provider", handler.HandlerGETOAuth2Callback)
}
