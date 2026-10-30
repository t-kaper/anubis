package repo

import (
	"anubis/app/DAL/models"
	"context"
)

// AuthRepository описывает все операции по разным типам авторизации:
// email, phone, виртуальная, OAuth.
type AuthRepository interface {
	// ---------------- EMAIL AUTH ----------------

	// GetByEmail ищет auth-запись по email.
	// Возвращает MdAuthEmail или ошибку (pgx.ErrNoRows, если не нашли).
	GetByEmail(ctx context.Context, email string) (*models.MdAuthEmail, error)

	// CreateEmailAuth создаёт запись в auth_email для пользователя.
	// Обычно вызывается при регистрации/подтверждении email.
	CreateEmailAuth(ctx context.Context, a *models.MdAuthEmail) error

	// ---------------- PHONE AUTH ----------------

	// GetByPhone ищет auth-запись по номеру телефона.
	GetByPhone(ctx context.Context, phone int64) (*models.MdAuthPhone, error)

	// TODO при необходимости:
	// CreatePhoneAuth(ctx context.Context, a *models.MdAuthPhone) error

	// ---------------- VIRTUAL LOGIN ----------------

	GetVirtual(ctx context.Context, login string) (*models.MdAuthVirtual, error)

	// 🔹 НОВОЕ
	CreateVirtualAuth(ctx context.Context, a *models.MdAuthVirtual) error

	// найти виртуал по user_id
	GetVirtualByUserID(ctx context.Context, userID int64) (*models.MdAuthVirtual, error)

	// обновить логин/пароль виртуального пользователя
	UpdateVirtualAuth(ctx context.Context, a *models.MdAuthVirtual) error

	// удалить виртуальный логин по user_id
	DeleteVirtualByUserID(ctx context.Context, userID int64) error

	// ---------------- OAUTH AUTH ----------------

	// GetOAuth ищет OAuth-аккаунт по provider + provider_user_id.
	// Используется как основной способ связать callback от провайдера с нашим пользователем.
	GetOAuth(ctx context.Context, provider string, providerUserID string) (*models.MdOAuthAccount, error)

	// GetOAuthByEmail ищет OAuth-аккаунт по email (если провайдер отдал email и мы его сохранили).
	// Используется для "склейки" аккаунтов между провайдерами по одному и тому же email.
	GetOAuthByEmail(ctx context.Context, email string) (*models.MdOAuthAccount, error)

	// CreateOAuth создаёт новую связку user <-> OAuth-провайдер.
	CreateOAuth(ctx context.Context, acc *models.MdOAuthAccount) error

	// UpsertOAuthProfile обновляет профильные данные провайдера (name/email/updated_at).
	// В текущей реализации ты используешь MdOAuthAccount как источник данных.
	// Внутри: INSERT ... ON CONFLICT (provider, provider_user_id / external_id) DO UPDATE ...
	UpsertOAuthProfile(ctx context.Context, profile *models.MdOAuthAccount) error

	// GetAuthTypesByUserIDs возвращает список способов авторизации пользователя:
	// - "email", "phone", "virtual", "oauth:<provider>"
	GetAuthTypesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]string, error)
}
