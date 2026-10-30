package routes

import (
	"anubis/app/DAL/storage"
	"anubis/app/api/controllers"
	"anubis/app/api/usecase"
	"anubis/app/core"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouteRoles регистрирует роут /roles (owner/admin only) на корневом роутере.
func NewRouteRoles(
	pgPool *pgxpool.Pool,
	app *gin.Engine,
	config core.ServiceConfig,
) {
	userRepo := storage.NewRepositoryPgUser(pgPool)
	workspaceRepo := storage.NewRepositoryPgWorkspace(pgPool, &config)
	authRepo := storage.NewRepositoryPgAuth(pgPool)

	serviceWorkspace := usecase.NewServiceWorkspace(
		userRepo,
		workspaceRepo,
		authRepo,
		config,
	)

	handler := controllers.NewControllerRoles(serviceWorkspace)
	app.POST("/roles", handler.HandlerPOSTRoles)
}
