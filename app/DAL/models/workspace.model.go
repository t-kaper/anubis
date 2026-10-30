package models

import (
	"fmt"
	"time"
)

// MdWorkspace представляет проект/группу, в которой объединяются пользователи
// для работы в рамках одного сервиса/домена.
// Мапится на таблицу `workspaces`.
type MdWorkspace struct {
	// ID — первичный ключ проекта.
	ID int64 `db:"id" json:"id"`

	// Name — имя проекта (отображаемое).
	Name string `db:"name" json:"name"`

	// Domain — домен/сервис, к которому относится проект (chanki/yocha/...).
	Domain string `db:"domain" json:"domain"`

	// CreatedAt — когда проект был создан.
	CreatedAt time.Time `db:"created_at" json:"created_at"`

	// MaxMembers — лимит участников проекта (по умолчанию 10).
	MaxMembers int `db:"max_members" json:"max_members"`
}

// MdWorkspaceMember описывает участие пользователя в проекте.
// Мапится на таблицу `workspace_members`.
type MdWorkspaceMember struct {
	// WorkspaceID — ссылка на проект (workspaces.id).
	WorkspaceID int64 `db:"workspace_id" json:"workspace_id"`

	// UserID — ссылка на пользователя (users.id).
	UserID int64 `db:"user_id" json:"user_id"`

	// Role — роль пользователя в проекте.
	// Допустимые значения: "admin", "editor", "viewer", "owner", "guest".
	Role string `db:"role" json:"role"`

	// JoinedAt — когда пользователь вступил в проект.
	JoinedAt time.Time `db:"joined_at" json:"joined_at"`
}

// MdWorkspaceUserInfo — расширенная информация об участнике проекта,
// удобная для админских списков (user list) без дополнительных запросов к users.
type MdWorkspaceUserInfo struct {
	UserID    int64     `json:"user_id"`
	Nickname  string    `json:"nickname"`
	IsVirtual bool      `json:"is_virtual"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

// GetUserRole — helper-функция, чтобы найти роль юзера в проекте
// по списку участников.
func (p *MdWorkspace) GetUserRole(userID int64, members []MdWorkspaceMember) (string, error) {
	for _, m := range members {
		if m.UserID == userID {
			return m.Role, nil
		}
	}
	return "", fmt.Errorf("user with ID %d not found in workspace %d", userID, p.ID)
}
