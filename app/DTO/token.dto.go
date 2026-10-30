package DTO

type ExchangeTicketValid struct {
	Ticket   string `json:"ticket"  binding:"required"`
	DeviceID string `json:"device_id" binding:"required,min=12,max=64"` // то же device_id, что и раньше
}

type AnswerExchangeTicketToken struct {
	RefreshToken   string            `json:"refresh_token"`
	Listworkspaces map[string]string `json:"list_workspaces"`
}

type AnswerToken struct {
	RefreshToken   string            `json:"refresh_token"`
	AccessToken    string            `json:"access_token"`
	Listworkspaces map[string]string `json:"list_workspaces"`
}

type RefreshTokenDomainValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	WorkspaceID  string `json:"workspace_id"  binding:"required"`
	DeviceID     string `json:"device_id"   binding:"required,min=12,max=64"`
}

type LogoutDeviceValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id"    binding:"required,min=12,max=64"`
}

type LogoutAllValid struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// VirtualLoginValid — вход виртуального пользователя по логину/паролю.
// Возвращает login-ticket, который затем обменивается на refresh_token через /auth/exchange_ticket.
type VirtualLoginValid struct {
	DeviceID string `json:"device_id" binding:"required,min=12,max=64"`
	Login    string `json:"login" binding:"required,safe_text,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

// MeResponse — информация о текущей сессии/пользователе по access-токену.
type MeResponse struct {
	UserID      int64    `json:"user_id"`
	WorkspaceID int64    `json:"workspace_id"`
	Role        string   `json:"role"`
	Audience    []string `json:"audience,omitempty"`
	ExpiresAt   int64    `json:"expires_at,omitempty"`
}

type IntrospectValid struct {
	// Token — access token
	Token string `json:"token" binding:"required"`
}

type IntrospectResponse struct {
	Active      bool     `json:"active"`
	UserID      int64    `json:"user_id,omitempty"`
	WorkspaceID int64    `json:"workspace_id,omitempty"`
	Role        string   `json:"role,omitempty"`
	Audience    []string `json:"audience,omitempty"`
	ExpiresAt   int64    `json:"expires_at,omitempty"`
}

// RevokeValid — отзыв refresh token (конкретной сессии).
// Если передать DeviceID — дополнительно проверим биндинг сессии к устройству.
type RevokeValid struct {
	RefreshToken string  `json:"refresh_token" binding:"required"`
	DeviceID     *string `json:"device_id,omitempty" binding:"omitempty,min=12,max=64"`
}
