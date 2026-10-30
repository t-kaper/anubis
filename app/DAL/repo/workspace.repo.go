package repo

import (
	"anubis/app/DAL/models"
	"context"
	"errors"
)

var (
	// ErrWorkspaceMemberLimitReached — лимит участников проекта исчерпан.
	ErrWorkspaceMemberLimitReached = errors.New("workspace member limit reached")

	// ErrInviteNotFound — инвайт не найден.
	ErrInviteNotFound = errors.New("invite not found")
	// ErrInviteExpired — инвайт протух.
	ErrInviteExpired = errors.New("invite expired")
	// ErrInviteUsed — инвайт уже использован.
	ErrInviteUsed = errors.New("invite already used")
	// ErrInviteAlreadyMember — пользователь уже состоит в проекте.
	ErrInviteAlreadyMember = errors.New("user is already a workspace member")
)

// WorkspaceRepository описывает операции с проектами (workspaces) и участниками (workspace_members).
type WorkspaceRepository interface {
	// GetworkspacesByUser возвращает все проекты, в которых участвует пользователь.
	// Используется, чтобы собрать list_workspaces после авторизации.
	GetworkspacesByUserInDomain(ctx context.Context, userID int64, domain string) ([]models.MdWorkspace, error)

	CreateWorkspace(
		ctx context.Context,
		workspace *models.MdWorkspace,
		ownerID int64,
	) (*models.MdWorkspace, error)
	// GetUserRole возвращает роль пользователя в конкретном проекте.
	// Например: "owner", "admin", "editor", "viewer", "guest".
	GetUserRole(ctx context.Context, userID int64, workspaceID int64) (string, error)

	// GetWorkspaceDomainAndUserRole — комбайн-запрос:
	// - возвращает domain проекта (если проект существует)
	// - возвращает роль пользователя в проекте (если он участник)
	// - позволяет избежать двух запросов (GetWorkspaceByID + GetUserRole)
	GetWorkspaceDomainAndUserRole(ctx context.Context, workspaceID int64, userID int64) (domain string, role string, isMember bool, err error)

	// Получить проект по ID (чтобы проверить domain, принадлежность и т.п.).
	GetWorkspaceByID(ctx context.Context, workspaceID int64) (*models.MdWorkspace, error)

	// Все участники проекта
	GetWorkspaceMembers(ctx context.Context, workspaceID int64) ([]models.MdWorkspaceMember, error)

	// GetWorkspaceUsers — список участников проекта с данными пользователя (nickname/is_virtual).
	GetWorkspaceUsers(ctx context.Context, workspaceID int64) ([]models.MdWorkspaceUserInfo, error)

	// Удаление проекта (workspace_members должны быть ON DELETE CASCADE или удаляться отдельно)
	DeleteWorkspace(ctx context.Context, workspaceID int64) error

	// RemoveMember — убрать пользователя из проекта.
	RemoveMember(ctx context.Context, workspaceID, userID int64) error

	// UpdateMemberRole — изменить роль участника в проекте.
	UpdateMemberRole(ctx context.Context, workspaceID, userID int64, role string) error

	// TryAddMemberWithLimit — атомарно пытается добавить участника в проект,
	// проверяя лимит max_members. Если лимит исчерпан — ErrWorkspaceMemberLimitReached.
	// Если участник уже есть — вернёт nil (идемпотентно).
	TryAddMemberWithLimit(ctx context.Context, workspaceID, userID int64, role string) error

	// Посчитать количество участников проекта (для SingleMember можно использовать).
	CountMembers(ctx context.Context, workspaceID int64) (int64, error)

	// -------- Invites --------

	CreateWorkspaceInvite(ctx context.Context, invite *models.MdWorkspaceInvite) (*models.MdWorkspaceInvite, error)
	GetWorkspaceInviteByHash(ctx context.Context, tokenHash string) (*models.MdWorkspaceInvite, error)

	// AcceptWorkspaceInvite — атомарно принимает инвайт:
	//  - проверяет used_at/expires_at,
	//  - проверяет лимит участников,
	//  - добавляет user в workspace_members,
	//  - помечает инвайт использованным.
	//
	// Возвращает workspace_id и роль из инвайта.
	AcceptWorkspaceInvite(ctx context.Context, tokenHash string, userID int64) (workspaceID int64, role string, err error)
}
