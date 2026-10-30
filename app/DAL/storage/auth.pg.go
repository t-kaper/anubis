package storage

import (
	"anubis/app/DAL/models"
	"anubis/app/DAL/repo"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgAuthRepository struct {
	DB *pgxpool.Pool
}

func NewRepositoryPgAuth(db *pgxpool.Pool) repo.AuthRepository {
	return &PgAuthRepository{DB: db}
}

//
// ---------------- EMAIL AUTH ----------------
//

// GetByEmail — ищем email в auth_email.
func (r *PgAuthRepository) GetByEmail(ctx context.Context, email string) (*models.MdAuthEmail, error) {

	query := `
		SELECT id, user_id, email, password_hash, verified, forgot_code, forgot_expires_at
		FROM auth_email
		WHERE email = $1;
	`

	out := &models.MdAuthEmail{}
	err := r.DB.QueryRow(ctx, query, email).Scan(
		&out.ID,
		&out.UserID,
		&out.Email,
		&out.PasswordHash,
		&out.Verified,
		&out.ForgotCode,
		&out.ForgotExpiresAt,
	)

	return out, err
}

// CreateEmailAuth — создаём запись auth_email.
func (r *PgAuthRepository) CreateEmailAuth(
	ctx context.Context,
	a *models.MdAuthEmail,
) error {

	query := `
		INSERT INTO auth_email (user_id, email, password_hash, verified, forgot_code, forgot_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6);
	`

	_, err := r.DB.Exec(ctx, query,
		a.UserID,
		a.Email,
		a.PasswordHash,
		a.Verified,
		a.ForgotCode,
		a.ForgotExpiresAt,
	)
	return err
}

//
// ---------------- PHONE AUTH ----------------
//

// GetByPhone — ищем телефон.
func (r *PgAuthRepository) GetByPhone(ctx context.Context, phone int64) (*models.MdAuthPhone, error) {

	query := `
		SELECT id, user_id, phone, country_code, password_hash, verified
		FROM auth_phone
		WHERE phone = $1;
	`

	out := &models.MdAuthPhone{}

	err := r.DB.QueryRow(ctx, query, phone).Scan(
		&out.ID,
		&out.UserID,
		&out.Phone,
		&out.CountryCode,
		&out.PasswordHash,
		&out.Verified,
	)

	return out, err
}

//
// ---------------- VIRTUAL LOGIN ----------------
//

// GetVirtual — ищем виртуальный логин.
func (r *PgAuthRepository) GetVirtual(ctx context.Context, login string) (*models.MdAuthVirtual, error) {
	query := `
		SELECT id, user_id, login, password_hash, verification
		FROM auth_virtual
		WHERE login = $1;
	`

	out := &models.MdAuthVirtual{}
	err := r.DB.QueryRow(ctx, query, login).Scan(
		&out.ID,
		&out.UserID,
		&out.Login,
		&out.PasswordHash,
		&out.Verification,
	)

	return out, err
}

//
// ---------------- OAUTH AUTH ----------------
//

// GetOAuth — ищем связку provider + external_id.
func (r *PgAuthRepository) GetOAuth(
	ctx context.Context,
	provider string,
	externalID string,
) (*models.MdOAuthAccount, error) {

	query := `
		SELECT id, user_id, provider, provider_user_id, created_at
		FROM oauth_accounts
		WHERE provider = $1 AND provider_user_id = $2;
	`

	// Не выделяем память сразу, создаем структуру только для сканирования
	var out models.MdOAuthAccount

	err := r.DB.QueryRow(ctx, query, provider, externalID).Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.ProviderUserID,
		&out.CreatedAt,
	)

	if err != nil {
		// Если ничего не найдено или ошибка БД — возвращаем nil и ошибку
		return nil, err
	}

	// Если все ок — возвращаем указатель
	return &out, nil
}

// GetOAuthByEmail — ищем oauth аккаунт по email (если сохранён).
func (r *PgAuthRepository) GetOAuthByEmail(
	ctx context.Context,
	email string,
) (*models.MdOAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, created_at
		FROM oauth_accounts
		WHERE email = $1
		ORDER BY updated_at DESC
		LIMIT 1;
	`

	var out models.MdOAuthAccount
	err := r.DB.QueryRow(ctx, query, email).Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.ProviderUserID,
		&out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PgAuthRepository) CreateOAuth(
	ctx context.Context,
	acc *models.MdOAuthAccount,
) error {

	query := `
		INSERT INTO oauth_accounts (user_id, provider, provider_user_id, name, email, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW());
	`

	_, err := r.DB.Exec(ctx, query,
		acc.UserID,
		acc.Provider,
		acc.ProviderUserID,
		acc.Name,
		acc.Email,
	)
	return err
}

func (r *PgAuthRepository) UpsertOAuthProfile(
	ctx context.Context,
	profile *models.MdOAuthAccount,
) error {
	fmt.Printf("[DEBUG] OAuth profile: %+v\n", profile)
	query := `
		INSERT INTO oauth_accounts (user_id, provider, provider_user_id, name, email, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_user_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			updated_at = EXCLUDED.updated_at;
	`

	_, err := r.DB.Exec(ctx,
		query,
		profile.UserID,         // $1 → user_id
		profile.Provider,       // $2
		profile.ProviderUserID, // $3
		profile.Name,           // $4
		profile.Email,          // $5
		time.Now(),             // $6 → updated_at
	)

	return err
}

func (r *PgAuthRepository) CreateVirtualAuth(
	ctx context.Context,
	a *models.MdAuthVirtual,
) error {

	query := `
		INSERT INTO auth_virtual (user_id, login, password_hash, verification, created_at)
		VALUES ($1, $2, $3, TRUE, NOW());
	`

	_, err := r.DB.Exec(ctx, query,
		a.UserID,
		a.Login,
		a.PasswordHash,
	)
	return err
}

// GetVirtualByUserID — ищем виртуального пользователя по user_id.
func (r *PgAuthRepository) GetVirtualByUserID(
	ctx context.Context,
	userID int64,
) (*models.MdAuthVirtual, error) {

	query := `
		SELECT id, user_id, login, password_hash, verification, created_at
		FROM auth_virtual
		WHERE user_id = $1;
	`

	out := &models.MdAuthVirtual{}
	err := r.DB.QueryRow(ctx, query, userID).Scan(
		&out.ID,
		&out.UserID,
		&out.Login,
		&out.PasswordHash,
		&out.Verification,
		&out.CreatedAt,
	)

	return out, err
}

// UpdateVirtualAuth — обновляем логин/пароль виртуального пользователя.
func (r *PgAuthRepository) UpdateVirtualAuth(
	ctx context.Context,
	a *models.MdAuthVirtual,
) error {

	query := `
		UPDATE auth_virtual
		SET login = $1,
		    password_hash = $2
		WHERE user_id = $3;
	`

	_, err := r.DB.Exec(ctx,
		query,
		a.Login,
		a.PasswordHash,
		a.UserID,
	)

	return err
}

// DeleteVirtualByUserID — удаляем auth_virtual по user_id.
func (r *PgAuthRepository) DeleteVirtualByUserID(
	ctx context.Context,
	userID int64,
) error {
	query := `
		DELETE FROM auth_virtual
		WHERE user_id = $1;
	`
	_, err := r.DB.Exec(ctx, query, userID)
	return err
}

func (r *PgAuthRepository) GetAuthTypesByUserIDs(
	ctx context.Context,
	userIDs []int64,
) (map[int64][]string, error) {
	out := make(map[int64][]string)
	if len(userIDs) == 0 {
		return out, nil
	}

	const q = `
		SELECT user_id, auth_type
		FROM (
			SELECT user_id, 'email'::text AS auth_type FROM auth_email WHERE user_id = ANY($1)
			UNION ALL
			SELECT user_id, 'phone'::text AS auth_type FROM auth_phone WHERE user_id = ANY($1)
			UNION ALL
			SELECT user_id, 'virtual'::text AS auth_type FROM auth_virtual WHERE user_id = ANY($1)
			UNION ALL
			SELECT user_id, ('oauth:' || provider)::text AS auth_type FROM oauth_accounts WHERE user_id = ANY($1)
		) t;
	`

	rows, err := r.DB.Query(ctx, q, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var uid int64
		var at string
		if err := rows.Scan(&uid, &at); err != nil {
			return nil, err
		}
		at = strings.TrimSpace(at)
		if at == "" {
			continue
		}
		out[uid] = append(out[uid], at)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	for uid, list := range out {
		sort.Strings(list)
		dedup := list[:0]
		var prev string
		for _, v := range list {
			if v == prev {
				continue
			}
			dedup = append(dedup, v)
			prev = v
		}
		out[uid] = dedup
	}

	return out, nil
}
