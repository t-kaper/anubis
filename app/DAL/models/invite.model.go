package models

import "time"

// MdWorkspaceInvite — одноразовый инвайт в проект.
// Мапится на таблицу `workspace_invites`.
type MdWorkspaceInvite struct {
	ID             int64      `db:"id" json:"id"`
	WorkspaceID    int64      `db:"workspace_id" json:"workspace_id"`
	CreatedBy      int64      `db:"created_by" json:"created_by"`
	TokenHash      string     `db:"token_hash" json:"-"`
	Role           string     `db:"role" json:"role"`
	AuthMethod     *string    `db:"auth_method" json:"auth_method,omitempty"`
	OAuth2Provider *string    `db:"oauth2_provider" json:"oauth2_provider,omitempty"`
	ExpiresAt      time.Time  `db:"expires_at" json:"expires_at"`
	UsedAt         *time.Time `db:"used_at" json:"used_at,omitempty"`
	UsedBy         *int64     `db:"used_by" json:"used_by,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}
