package interfaces

import (
	"anubis/app/DTO"
	"anubis/app/core/schemes"

	"github.com/gin-gonic/gin"
)

type WorkspaceUseCase interface {

	// Добавить пользователя в проект через refresh_token
	AddUserToWorkspace(ctx *gin.Context, body *DTO.AddUserToWorkspaceValid) (schemes.EmptyResponses, error)
	RemoveUserFromWorkspace(ctx *gin.Context, body *DTO.WorkspaceRemoveUserValid) (schemes.EmptyResponses, error)
	UpdateUserToWorkspace(ctx *gin.Context, body *DTO.UpdateUserToWorkspaceValid) (schemes.EmptyResponses, error)
	CreateVirtualUserInWorkspace(ctx *gin.Context, body *DTO.WorkspaceCreateVirtualUserValid) (DTO.WorkspaceCreateVirtualUserResponse, error)
	UpdateVirtualUserInWorkspace(ctx *gin.Context, body *DTO.WorkspaceUpdateVirtualUserValid) (DTO.WorkspaceUpdateVirtualUserResponse, error)
	DeleteVirtualUserInWorkspace(ctx *gin.Context, body *DTO.WorkspaceDeleteVirtualUserValid) (DTO.WorkspaceDeleteVirtualUserResponse, error)
	ListWorkspaceUsers(ctx *gin.Context, body *DTO.WorkspaceUserListValid) (DTO.WorkspaceUserListResponse, error)
	ListWorkspaceUsersPublic(ctx *gin.Context, body *DTO.WorkspaceUserListPublicValid) (DTO.WorkspaceUserListPublicResponse, error)
	CreateWorkspaceInvite(ctx *gin.Context, body *DTO.WorkspaceInviteCreateValid) (DTO.WorkspaceInviteCreateResponse, error)
	// RemoveUserFromWorkspace, и т.д.

	CreateWorkspace(ctx *gin.Context, input *DTO.CreateWorkspaceValid) (DTO.WorkspaceInfo, error)
	DeleteWorkspace(ctx *gin.Context, input *DTO.DeleteWorkspaceValid) error
}
