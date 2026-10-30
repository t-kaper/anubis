package controllers

import (
	"anubis/app/DTO"
	"anubis/app/api/interfaces"
	"anubis/app/core/common"
	"anubis/app/core/schemes"

	"github.com/gin-gonic/gin"
)

// NOTE: Пустые ссылки ниже предназначены только для помощи swag (генератора swagger)
// используется в аннотациях без влияния на поведение во время выполнения.
var (
	_ = DTO.WorkspaceUserListValid{}
	_ = DTO.WorkspaceUserListResponse{}
	_ = DTO.WorkspaceUserListPublicValid{}
	_ = DTO.WorkspaceUserListPublicResponse{}
	_ = DTO.WorkspaceInviteCreateValid{}
	_ = DTO.WorkspaceInviteCreateResponse{}
)

type ControllerWorkspace struct {
	workspaceUC interfaces.WorkspaceUseCase
}

func NewControllerWorkspace(workspaceUC interfaces.WorkspaceUseCase) *ControllerWorkspace {
	return &ControllerWorkspace{workspaceUC: workspaceUC}
}

// HandlerPOSTAddUserToWorkspace adds an existing user to a workspace.
// @Summary Добавить участника в проект
// @Description Добавляет target_user_id в workspace_id с указанной ролью (роль может быть с вариантом, например manager:all).
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.AddUserToWorkspaceValid true "refresh_token + workspace_id + target_user_id + role"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/add_member [post]
func (c *ControllerWorkspace) HandlerPOSTAddUserToWorkspace(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.AddUserToWorkspace)
}

// HandlerPOSTWorkspaceRemoveUser removes a user from a workspace.
// @Summary Удалить участника из проекта
// @Description Удаляет target_user_id из workspace_id. Доступно только owner/admin (по базовой роли).
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceRemoveUserValid true "refresh_token + workspace_id + target_user_id + device_id"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/remove_member [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceRemoveUser(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.RemoveUserFromWorkspace)
}

// HandlerPOSTUpdateUserToWorkspace updates role of an existing workspace member.
// @Summary Обновить роль участника в проекте
// @Description Меняет роль target_user_id в проекте. Доступно owner/admin (по базовой роли). Назначать/менять owner этим эндпоинтом нельзя.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.UpdateUserToWorkspaceValid true "refresh_token + workspace_id + target_user_id + role"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/update_member [post]
func (c *ControllerWorkspace) HandlerPOSTUpdateUserToWorkspace(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.UpdateUserToWorkspace)
}

// HandlerPOSTWorkspaceUserList returns workspace users with auth types (admin/owner only).
// @Summary Список пользователей проекта
// @Description Возвращает всех участников проекта с их ролью, флагом is_virtual и типами авторизации (email/phone/virtual/oauth:<provider>). Только owner/admin.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceUserListValid true "refresh_token + workspace_id + device_id"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceUserListResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/user/list [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceUserList(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.ListWorkspaceUsers)
}

// HandlerPOSTWorkspaceUserListPublic returns safe workspace users list for all workspace members (e.g. for chat).
// @Summary Список пользователей проекта (для участников)
// @Description Возвращает участников проекта (user_id, nickname, role). Доступно любому участнику проекта.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceUserListPublicValid true "refresh_token + workspace_id + device_id"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceUserListPublicResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/user/list_public [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceUserListPublic(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.ListWorkspaceUsersPublic)
}

// HandlerPOSTWorkspaceInviteCreate creates one-time invite token for workspace.
// @Summary Создать инвайт в проект
// @Description Создаёт одноразовый invite_token (с TTL). Можно ограничить типом авторизации (auth_method) и для oauth2 — провайдером (oauth2_provider). Только owner/admin.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceInviteCreateValid true "refresh_token + device_id + workspace_id + role + auth_method + oauth2_provider"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceInviteCreateResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/invite/create [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceInviteCreate(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.CreateWorkspaceInvite)
}

// HandlerPOSTWorkspaceCreateVirtualUser creates a virtual user and adds them to workspace.
// @Summary Создать виртуального пользователя в проекте
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceCreateVirtualUserValid true "refresh_token + workspace_id + device_id + login + password + role"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceCreateVirtualUserResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/virtual_user/create [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceCreateVirtualUser(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.CreateVirtualUserInWorkspace)
}

// HandlerPOSTWorkspaceUpdateVirtualUser updates virtual user's login/password.
// @Summary Обновить виртуального пользователя
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceUpdateVirtualUserValid true "refresh_token + workspace_id + device_id + user_id + new_login/new_password"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceUpdateVirtualUserResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/virtual_user/update [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceUpdateVirtualUser(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.UpdateVirtualUserInWorkspace)
}

// HandlerPOSTWorkspaceDeleteVirtualUser removes virtual user from workspace and deletes account.
// @Summary Удалить виртуального пользователя
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.WorkspaceDeleteVirtualUserValid true "refresh_token + workspace_id + device_id + user_id"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceDeleteVirtualUserResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/virtual_user/delete [post]
func (c *ControllerWorkspace) HandlerPOSTWorkspaceDeleteVirtualUser(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.DeleteVirtualUserInWorkspace)
}

// HandlerPOSTCreateWorkspace creates a new workspace in current domain.
// @Summary Создать проект
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.CreateWorkspaceValid true "refresh_token + device_id + name"
// @Success 200 {object} schemes.Responses{data=DTO.WorkspaceInfo}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/create [post]
func (c *ControllerWorkspace) HandlerPOSTCreateWorkspace(ctx *gin.Context) {
	common.JsonHandler(ctx, c.workspaceUC.CreateWorkspace)
}

// HandlerPOSTDeleteWorkspace deletes a workspace (only owner base-role).
// @Summary Удалить проект
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.DeleteWorkspaceValid true "refresh_token + device_id + workspace_id"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /workspace/delete [post]
func (c *ControllerWorkspace) HandlerPOSTDeleteWorkspace(ctx *gin.Context) {
	common.JsonHandler(ctx, func(ctx *gin.Context, body *DTO.DeleteWorkspaceValid) (schemes.EmptyResponses, error) {
		err := c.workspaceUC.DeleteWorkspace(ctx, body)
		return schemes.EmptyResponses{}, err
	})
}
