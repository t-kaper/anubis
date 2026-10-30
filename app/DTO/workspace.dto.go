package DTO

// AddUserToWorkspaceValid — добавить пользователя в проект.
// Важно: операция разрешена только при наличии валидного refresh_token.
type AddUserToWorkspaceValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`  // чей refresh — того и считаем инициатором
	WorkspaceID  string `json:"workspace_id"   binding:"required"` // в какой проект добавляем
	TargetUserID string `json:"target_user_id" binding:"required"` // какого пользователя добавляем (id в БД)
	// Role — роль участника в проекте. Может быть базовой ("user") или с вариантом ("manager:all").
	// Если не передали — бекенд выберет дефолтную ("user" -> "guest").
	Role string `json:"role"`
}

// UpdateUserToWorkspaceValid — изменить роль пользователя в проекте.
// Важно: operation owner/admin; owner workspace role нельзя менять этим endpoint'ом (только отдельным transfer-ownership, если понадобится).
type UpdateUserToWorkspaceValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id" binding:"required"`
	TargetUserID string `json:"target_user_id" binding:"required"`
	Role         string `json:"role" binding:"required"`
}

// WorkspaceUserListValid — получить список всех пользователей проекта (только owner/admin).
type WorkspaceUserListValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id" binding:"required"`
	DeviceID     string `json:"device_id" binding:"required,min=12,max=64"`
}

type WorkspaceUserListItem struct {
	UserID    int64    `json:"user_id"`
	Nickname  string   `json:"nickname"`
	IsVirtual bool     `json:"is_virtual"`
	Role      string   `json:"role"`
	AuthTypes []string `json:"auth_types"`
}

type WorkspaceUserListResponse struct {
	Users []WorkspaceUserListItem `json:"users"`
}

// WorkspaceUserListPublicValid — безопасный список участников проекта для всех участников (например, для чата).
type WorkspaceUserListPublicValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id" binding:"required"`
	DeviceID     string `json:"device_id" binding:"required,min=12,max=64"`
}

type WorkspaceUserPublicItem struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
}

type WorkspaceUserListPublicResponse struct {
	Users []WorkspaceUserPublicItem `json:"users"`
}

// RolesValid — получить справочник ролей домена (только owner/admin в проекте).
type RolesValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id" binding:"required"`
	DeviceID     string `json:"device_id" binding:"required,min=12,max=64"`
}

type RoleVariantDTO struct {
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
}

type RoleDTO struct {
	Name        string                    `json:"name"`
	Code        string                    `json:"code,omitempty"`
	Description string                    `json:"description,omitempty"`
	Variants    map[string]RoleVariantDTO `json:"variants,omitempty"`
}

type RolesResponse struct {
	Roles []RoleDTO `json:"roles"`
}

// WorkspaceRemoveUserValid — удалить пользователя из проекта.
// Авторизация идёт через refresh_token + device_id.
type WorkspaceRemoveUserValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id"   binding:"required"`
	TargetUserID string `json:"target_user_id" binding:"required"`             // кого кикаем
	DeviceID     string `json:"device_id"    binding:"required,min=12,max=64"` // девайс того, кто делает запрос
}

// WorkspaceCreateVirtualUserValid — создать виртуального пользователя и добавить в проект.
type WorkspaceCreateVirtualUserValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id"    binding:"required"`
	DeviceID     string `json:"device_id"     binding:"required,min=12,max=64"`

	Login    string `json:"login"    binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	// Роль в проекте для виртуала. Если не указать — можно по умолчанию "user"/"member".
	Role string `json:"role" binding:"required"`
}

type WorkspaceCreateVirtualUserResponse struct {
	UserID int64  `json:"user_id"`
	Login  string `json:"login"`
}

// 🔹 Изменение виртуального пользователя
type WorkspaceUpdateVirtualUserValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id"    binding:"required"`
	DeviceID     string `json:"device_id"     binding:"required,min=12,max=64"`

	// какого юзера меняем
	UserID int64 `json:"user_id" binding:"required"`

	// новые данные (можно менять только одно из полей)
	NewLogin    *string `json:"new_login,omitempty" binding:"omitempty,safe_text,min=3,max=64"`
	NewPassword *string `json:"new_password,omitempty" binding:"omitempty,min=6,max=128"`
}

type WorkspaceUpdateVirtualUserResponse struct {
	UserID int64  `json:"user_id"`
	Login  string `json:"login"`
}

// 🔹 Удаление виртуального пользователя
type WorkspaceDeleteVirtualUserValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id"    binding:"required"`
	DeviceID     string `json:"device_id"     binding:"required,min=12,max=64"`

	UserID int64 `json:"user_id" binding:"required"`
}

type WorkspaceDeleteVirtualUserResponse struct {
	Deleted bool `json:"deleted"`
}

// Создание проекта (по refresh-токену)
type CreateWorkspaceValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id"    binding:"required,min=12,max=64"`
	Name         string `json:"name"         binding:"required,safe_text,max=128"`
}

// Ответ про проект (можно переиспользовать)
type WorkspaceInfo struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	MaxMembers int    `json:"max_members"`
}

// Удаление проекта (по refresh-токену)
type DeleteWorkspaceValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id"    binding:"required,min=12,max=64"`
	WorkspaceID  string `json:"workspace_id"   binding:"required"`
}

// WorkspaceInviteCreateValid — создать инвайт-ссылку в проект (owner/admin).
// auth_method:
//   - "any" или пусто → любой из разрешённых методов домена
//   - "oauth2" / "telegram" → ограничить конкретным методом
//
// oauth2_provider имеет смысл только при auth_method="oauth2".
type WorkspaceInviteCreateValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id"    binding:"required,min=12,max=64"`
	WorkspaceID  string `json:"workspace_id"   binding:"required"`

	Role string `json:"role,omitempty" binding:"omitempty"`

	AuthMethod     *string `json:"auth_method,omitempty" binding:"omitempty"`
	OAuth2Provider *string `json:"oauth2_provider,omitempty" binding:"omitempty"`
}

type WorkspaceInviteCreateResponse struct {
	InviteToken    string  `json:"invite_token"`
	ExpiresAtUnix  int64   `json:"expires_at_unix"`
	Role           string  `json:"role"`
	AuthMethod     *string `json:"auth_method,omitempty"`
	OAuth2Provider *string `json:"oauth2_provider,omitempty"`
}
