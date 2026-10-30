package repo

import (
	"anubis/app/DAL/models"
	"context"
	"time"
)

// UserRepository описывает контракты для работы с пользователями и их сессиями.
// Здесь нет ничего про Mongo/pgx — только бизнес-операции.
type UserRepository interface {
	// Уже есть:
	CreateUser(ctx context.Context, nickname string) (*models.MdUser, error)
	GetUserByID(ctx context.Context, id int64) (*models.MdUser, error)
	CreateUserSession(ctx context.Context, s *models.MdUserSession) error
	DeactivateSessions(ctx context.Context, userID int64) error

	// НОВОЕ:

	// Активные сессии юзера в рамках домена.
	GetActiveSessionsByUserAndDomain(
		ctx context.Context,
		userID int64,
		domain string,
	) ([]models.MdUserSession, error)

	// Обновить hash токена / expires у конкретной сессии.
	UpdateSessionToken(
		ctx context.Context,
		sessionID int64,
		hashToken string,
		expiresAt time.Time,
	) error

	// Деактивировать конкретные сессии по ID.
	DeactivateSessionsByIDs(
		ctx context.Context,
		ids []int64,
	) error

	DeactivateSessionsByDevice(
		ctx context.Context,
		userID int64,
		domain string,
		deviceID string,
	) error

	GetSessionByHash(
		ctx context.Context,
		hash string,
	) (*models.MdUserSession, error)

	DeactivateSessionsByUserAndDevice(
		ctx context.Context,
		userID int64,
		deviceID string,
	) error

	CreateUserVirtual(
		ctx context.Context,
		nickname string,
	) (*models.MdUser, error)

	// удалить пользователя
	DeleteUser(ctx context.Context, userID int64) error

	// обновить nickname / is_virtual при необходимости
	UpdateUser(
		ctx context.Context,
		user *models.MdUser,
	) error
}
