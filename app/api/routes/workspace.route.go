package routes

import (
	"anubis/app/DAL/storage"
	"anubis/app/api/controllers"
	"anubis/app/api/usecase"
	"anubis/app/core"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouteWorkspace(
	pgPool *pgxpool.Pool,
	app *gin.RouterGroup,
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

	handler := controllers.NewControllerWorkspace(serviceWorkspace)

	route := app.Group("")
	route.POST("/create", handler.HandlerPOSTCreateWorkspace)
	route.POST("/delete", handler.HandlerPOSTDeleteWorkspace)

	// Добавление пользователя в проект (через refresh_token)
	route.POST("/add_member", handler.HandlerPOSTAddUserToWorkspace)
	route.POST("/update_member", handler.HandlerPOSTUpdateUserToWorkspace)
	route.POST("/remove_member", handler.HandlerPOSTWorkspaceRemoveUser)
	route.POST("/invite/create", handler.HandlerPOSTWorkspaceInviteCreate)
	route.POST("/user/list", handler.HandlerPOSTWorkspaceUserList)
	route.POST("/user/list_public", handler.HandlerPOSTWorkspaceUserListPublic)
	route.POST("/virtual_user/create", handler.HandlerPOSTWorkspaceCreateVirtualUser)
	route.POST("/virtual_user/update", handler.HandlerPOSTWorkspaceUpdateVirtualUser)
	route.POST("/virtual_user/delete", handler.HandlerPOSTWorkspaceDeleteVirtualUser)
}
