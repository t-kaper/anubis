package models

import "time"

// MdAuthEmail представляет учётку авторизации по email.
// Мапится на таблицу `auth_email`.
type MdAuthEmail struct {
	// ID — первичный ключ записи.
	ID int64 `db:"id" json:"id"`

	// UserID — ссылка на пользователя.
	UserID int64 `db:"user_id" json:"user_id"`

	// Email — email в нижнем регистре, уникален в системе.
	Email string `db:"email" json:"email"`

	// PasswordHash — хэш пароля, связанного с этим email.
	PasswordHash string `db:"password_hash" json:"-"`

	// Verified — подтверждён ли email (true, если юзер прошёл верификацию).
	Verified bool `db:"verified" json:"verified"`

	// ForgotCode — код для восстановления пароля (если был запрошен).
	ForgotCode *string `db:"forgot_code" json:"-"`

	// ForgotExpiresAt — до какого времени действителен код восстановления.
	ForgotExpiresAt *time.Time `db:"forgot_expires_at" json:"-"`

	// CreatedAt — дата создания записи.
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// MdAuthPhone представляет учётку авторизации по телефону.
// Мапится на таблицу `auth_phone`.
type MdAuthPhone struct {
	// ID — первичный ключ записи.
	ID int64 `db:"id" json:"id"`

	// UserID — ссылка на пользователя.
	UserID int64 `db:"user_id" json:"user_id"`

	// Phone — номер телефона (обычно в E.164, но ты хранишь как int64).
	Phone int64 `db:"phone" json:"phone"`

	// CountryCode — код страны (например, 7, 1, 380 и т.п.).
	CountryCode int32 `db:"country_code" json:"country_code"`

	// PasswordHash — хэш пароля, если ты поддерживаешь логин по телефону+пароль.
	// Если авторизация только по коду (OTP) — можно держать nil.
	PasswordHash *string `db:"password_hash" json:"-"`

	// Verified — подтверждён ли телефон.
	Verified bool `db:"verified" json:"verified"`

	// CreatedAt — дата создания записи.
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// MdAuthVirtual представляет виртуальную учётку (без email/phone).
// Мапится на таблицу `auth_virtual`.
type MdAuthVirtual struct {
	// ID — первичный ключ записи.
	ID int64 `db:"id" json:"id"`

	// UserID — ссылка на пользователя.
	UserID int64 `db:"user_id" json:"user_id"`

	// Login — уникальный логин виртуального пользователя.
	Login string `db:"login" json:"login"`

	// PasswordHash — хэш пароля.
	PasswordHash string `db:"password_hash" json:"-"`

	// Verification — флаг верификации, по идее всегда true,
	// но можно использовать, если захочешь вводить проверку.
	Verification bool `db:"verification" json:"verification"`

	// CreatedAt — дата создания записи.
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// MdOAuthAccount представляет связь пользователя с OAuth-провайдером.
// Мапится на таблицу `oauth_accounts`.
type MdOAuthAccount struct {
	// ID — первичный ключ записи.
	ID int64 `db:"id" json:"id"`

	// UserID — ссылка на пользователя (owners).
	UserID int64 `db:"user_id" json:"user_id"`

	// Provider — название провайдера ("google", "yandex", "github" и т.п.).
	Provider string `db:"provider" json:"provider"`

	// ProviderUserID — стабильный ID пользователя в системе провайдера (sub/uid).
	ProviderUserID string `db:"provider_user_id" json:"provider_user_id"`

	// Name — имя пользователя по данным провайдера (может меняться).
	Name *string `db:"name" json:"name,omitempty"`

	// Email — email по данным провайдера (может меняться).
	Email *string `db:"email" json:"email,omitempty"`

	// UpdatedAt — когда мы последний раз обновляли профильные данные.
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// CreatedAt — когда привязка была создана.
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
