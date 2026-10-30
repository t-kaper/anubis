package models

import "time"

// MdUser представляет пользователя в системе.
// Эта модель мапится на таблицу `users` в Postgres.
type MdUser struct {
	// ID — первичный ключ пользователя (BIGSERIAL).
	ID int64 `db:"id" json:"id"`

	// Nickname — отображаемое имя пользователя.
	Nickname string `db:"nickname" json:"nickname"`

	// IsVirtual — флаг виртуального пользователя.
	// true → юзер создан как "виртуальный" (логин/пароль без email/phone).
	IsVirtual bool `db:"is_virtual" json:"is_virtual"`

	// CreatedAt — дата/время создания пользователя.
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// MdUsersSession описывает одну сессию пользователя.
// Мапится на таблицу `user_sessions`.
type MdUserSession struct {
	// ID — первичный ключ сессии.
	ID int64 `db:"id" json:"id"`

	// UserID — ссылка на пользователя (users.id).
	UserID int64 `db:"user_id" json:"user_id"`

	// Domain — домен/сервис, для которого выдана сессия (например, "chanki").
	Domain string `db:"domain" json:"domain"`

	// DeviceID — идентификатор устройства (генерируется на клиенте).
	DeviceID string `db:"device_id" json:"device_id"`

	// HashToken — хэш refresh-токена/сессионного токена.
	// В JSON наружу обычно это не отдаем.
	HashToken string `db:"hash_token" json:"-"`

	// DeviceType — тип/идентификатор клиента (web/ios/android/cli/...).
	DeviceType string `db:"device_type" json:"device_type"`

	// CreatedAt — когда сессия была создана.
	CreatedAt time.Time `db:"created_at" json:"created_at"`

	// ExpiresAt — когда сессия истекает.
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`

	// IP — IP-адрес, с которого создавалась сессия.
	IP string `db:"ip" json:"ip"`

	// IsActive — активна ли сессия сейчас.
	// Бизнес-правило "максимум 3 активных" проверяется в коде,
	// но это поле помогает быстро искать актуальные записи.
	IsActive bool `db:"is_active" json:"is_active"`
}
