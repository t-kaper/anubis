package routes

import (
	"anubis/app/DAL/storage"
	"anubis/app/api/controllers"
	"anubis/app/api/usecase"
	"anubis/app/core"
	"anubis/app/core/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouteTelegramAuth(
	pgPool *pgxpool.Pool,
	app *gin.RouterGroup,
	config core.ServiceConfig,
) {
	userRepo := storage.NewRepositoryPgUser(pgPool)
	workspaceRepo := storage.NewRepositoryPgWorkspace(pgPool, &config)
	authRepo := storage.NewRepositoryPgAuth(pgPool)

	// usecase будем писать позже, сейчас просто показываю, куда он встанет:
	serviceTelegram := usecase.NewServiceTelegramAuth(
		userRepo,
		workspaceRepo,
		authRepo,
		config,
	)

	handler := controllers.NewControllerTelegramAuth(serviceTelegram)

	route := app.Group("/telegram")
	route.Use(middlewares.CheckAuthTypeMiddleware("oauth2")) // или отдельный тип, если захочешь

	route.POST("/login_widget", handler.HandlerPOSTTelegramLoginWidget)
}
