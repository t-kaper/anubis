package storage

import (
	"anubis/app/DAL/models"
	"anubis/app/DAL/repo"
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgUserRepository — реализация интерфейса UserRepository.
type PgUserRepository struct {
	DB *pgxpool.Pool
}

// NewRepositoryPgUser — конструктор.
func NewRepositoryPgUser(db *pgxpool.Pool) repo.UserRepository {
	return &PgUserRepository{DB: db}
}

// CreateUser — создаём нового пользователя.
func (r *PgUserRepository) CreateUser(ctx context.Context, nickname string) (*models.MdUser, error) {
	query := `
		INSERT INTO users (nickname)
		VALUES ($1)
		RETURNING id, nickname, created_at;
	`

	user := &models.MdUser{}

	err := r.DB.QueryRow(ctx, query, nickname).Scan(
		&user.ID,
		&user.Nickname,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID — получаем пользователя по ID.
func (r *PgUserRepository) GetUserByID(ctx context.Context, id int64) (*models.MdUser, error) {
	query := `
		SELECT id, nickname, is_virtual, created_at
		FROM users
		WHERE id = $1;
	`

	user := &models.MdUser{}

	err := r.DB.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Nickname,
		&user.IsVirtual,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// CreateUserSession — создаём новую сессию (access/refresh хранится хэш).
func (r *PgUserRepository) CreateUserSession(
	ctx context.Context,
	session *models.MdUserSession,
) error {
	query := `
		INSERT INTO user_sessions
			(user_id, domain, device_id, hash_token, device_type, created_at, expires_at, ip, is_active)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, TRUE);
	`

	_, err := r.DB.Exec(ctx, query,
		session.UserID,
		session.Domain,
		session.DeviceID,
		session.HashToken,
		session.DeviceType,
		time.Now(),
		session.ExpiresAt,
		session.IP,
	)

	return err
}

// DeactivateSessions — делает все активные сессии неактивными.
func (r *PgUserRepository) DeactivateSessions(ctx context.Context, userID int64) error {
	query := `
		UPDATE user_sessions
		SET is_active = FALSE
		WHERE user_id = $1;
	`
	_, err := r.DB.Exec(ctx, query, userID)
	return err
}

func (r *PgUserRepository) UpdateSessionToken(
	ctx context.Context,
	sessionID int64,
	hashToken string,
	expiresAt time.Time,
) error {

	query := `
		UPDATE user_sessions
		SET
			hash_token = $1,
			expires_at = $2
		WHERE id = $3;
	`

	_, err := r.DB.Exec(ctx, query, hashToken, expiresAt, sessionID)
	return err
}

func (r *PgUserRepository) GetActiveSessionsByUserAndDomain(
	ctx context.Context,
	userID int64,
	domain string,
) ([]models.MdUserSession, error) {

	query := `
		SELECT
			id,
			user_id,
			domain,
			device_id,
			hash_token,
			device_type,
			created_at,
			expires_at,
			ip,
			is_active
		FROM user_sessions
		WHERE user_id = $1
		  AND domain  = $2
		  AND is_active = TRUE
		  AND expires_at > NOW()
		ORDER BY created_at ASC;
	`

	rows, err := r.DB.Query(ctx, query, userID, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.MdUserSession

	for rows.Next() {
		var s models.MdUserSession
		if err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.Domain,
			&s.DeviceID,
			&s.HashToken,
			&s.DeviceType,
			&s.CreatedAt,
			&s.ExpiresAt,
			&s.IP,
			&s.IsActive,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return sessions, nil
}

func (r *PgUserRepository) DeactivateSessionsByIDs(
	ctx context.Context,
	ids []int64,
) error {

	if len(ids) == 0 {
		return nil
	}

	// Используем ANY($1), pgx нормально кодирует []int64 в массив.
	query := `
		UPDATE user_sessions
		SET is_active = FALSE
		WHERE id = ANY($1);
	`

	_, err := r.DB.Exec(ctx, query, ids)
	return err
}

// GetSessionByHash — находим сессию по хэшу refresh-токена.
// Используется для opaque refresh: в самом токене нет session_id,
// мы ищем только по hash_token.
func (r *PgUserRepository) GetSessionByHash(
	ctx context.Context,
	hash string,
) (*models.MdUserSession, error) {

	query := `
		SELECT
			id,
			user_id,
			domain,
			device_id,
			hash_token,
			device_type,
			created_at,
			expires_at,
			ip,
			is_active
		FROM user_sessions
		WHERE hash_token = $1
		LIMIT 1;
	`

	out := &models.MdUserSession{}

	err := r.DB.QueryRow(ctx, query, hash).Scan(
		&out.ID,
		&out.UserID,
		&out.Domain,
		&out.DeviceID,
		&out.HashToken,
		&out.DeviceType,
		&out.CreatedAt,
		&out.ExpiresAt,
		&out.IP,
		&out.IsActive,
	)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// DeactivateSessionsByDevice — деактивируем ВСЕ сессии
// конкретного девайса пользователя в рамках домена.
// Используется как жёсткая реакция на reuse / подмену device_id.
func (r *PgUserRepository) DeactivateSessionsByDevice(
	ctx context.Context,
	userID int64,
	domain string,
	deviceID string,
) error {

	query := `
		UPDATE user_sessions
		SET is_active = FALSE
		WHERE user_id = $1
		  AND domain = $2
		  AND device_id = $3
		  AND is_active = TRUE;
	`

	_, err := r.DB.Exec(ctx, query, userID, domain, deviceID)
	return err
}

func (r *PgUserRepository) DeactivateSessionsByUserAndDevice(
	ctx context.Context,
	userID int64,
	deviceID string,
) error {

	query := `
		UPDATE user_sessions
		SET is_active = FALSE
		WHERE user_id = $1
		  AND device_id = $2
		  AND is_active = TRUE;
	`

	_, err := r.DB.Exec(ctx, query, userID, deviceID)
	return err
}

func (r *PgUserRepository) CreateUserVirtual(
	ctx context.Context,
	nickname string,
) (*models.MdUser, error) {

	query := `
		INSERT INTO users (nickname, is_virtual)
		VALUES ($1, TRUE)
		RETURNING id, nickname, is_virtual, created_at;
	`

	user := &models.MdUser{}

	err := r.DB.QueryRow(ctx, query, nickname).Scan(
		&user.ID,
		&user.Nickname,
		&user.IsVirtual,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// UpdateUser — обновляем nickname (и при желании is_virtual).
func (r *PgUserRepository) UpdateUser(
	ctx context.Context,
	user *models.MdUser,
) error {

	query := `
		UPDATE users
		SET nickname = $1,
		    is_virtual = $2
		WHERE id = $3;
	`

	_, err := r.DB.Exec(ctx,
		query,
		user.Nickname,
		user.IsVirtual,
		user.ID,
	)

	return err
}

// DeleteUser — удаляем пользователя.
// Важно: должны быть ON DELETE CASCADE в workspace_members, auth_*, user_sessions,
// либо удалять вручную в транзакции.
func (r *PgUserRepository) DeleteUser(ctx context.Context, userID int64) error {
	const q = `
		DELETE FROM users
		WHERE id = $1;
	`
	_, err := r.DB.Exec(ctx, q, userID)
	return err
}
