package storage

import (
	"anubis/app/DAL/models"
	"anubis/app/DAL/repo"
	"anubis/app/core"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgWorkspaceRepository struct {
	DB     *pgxpool.Pool
	Config *core.ServiceConfig
}

func NewRepositoryPgWorkspace(db *pgxpool.Pool, cfg *core.ServiceConfig) repo.WorkspaceRepository {
	return &PgWorkspaceRepository{DB: db, Config: cfg}
}

// GetworkspacesByUserInDomain — получаем все проекты пользователя в рамках конкретного домена.
func (r *PgWorkspaceRepository) GetworkspacesByUserInDomain(
	ctx context.Context,
	userID int64,
	domain string,
) ([]models.MdWorkspace, error) {

	query := `
		SELECT p.id, p.name, p.domain
		FROM workspaces p
		JOIN workspace_members m ON m.workspace_id = p.id
		WHERE m.user_id = $1 AND p.domain = $2;
	`

	rows, err := r.DB.Query(ctx, query, userID, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.MdWorkspace

	for rows.Next() {
		var p models.MdWorkspace
		if err := rows.Scan(&p.ID, &p.Name, &p.Domain); err != nil {
			return nil, err
		}
		list = append(list, p)
	}

	return list, nil
}

func (r *PgWorkspaceRepository) CreateWorkspace(
	ctx context.Context,
	workspace *models.MdWorkspace,
	ownerID int64,
) (*models.MdWorkspace, error) {

	// 1. Создаём проект
	queryWorkspace := `
		INSERT INTO workspaces (name, domain, max_members)
		VALUES ($1, $2, $3)
		RETURNING id, name, domain, created_at, max_members;
	`

	maxMembers := workspace.MaxMembers
	if maxMembers <= 0 {
		// Safety-net: если вызывающий не проставил лимит, используем дефолт БД (10).
		// (При этом в миграции всё равно стоит DEFAULT 10, но мы держим модель консистентной.)
		maxMembers = 10
	}

	created := &models.MdWorkspace{}
	err := r.DB.QueryRow(ctx, queryWorkspace,
		workspace.Name,
		workspace.Domain,
		maxMembers,
	).Scan(
		&created.ID,
		&created.Name,
		&created.Domain,
		&created.CreatedAt,
		&created.MaxMembers,
	)
	if err != nil {
		return nil, err
	}

	// 2. Добавляем владельца (owner) в workspace_members
	queryMember := `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner');
	`

	_, err = r.DB.Exec(ctx, queryMember, created.ID, ownerID)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *PgWorkspaceRepository) TryAddMemberWithLimit(
	ctx context.Context,
	workspaceID int64,
	userID int64,
	role string,
) error {
	// Делаем всё атомарно, чтобы параллельные добавления не пробивали лимит.
	// Лочим строку проекта (FOR UPDATE), считаем участников, сравниваем с max_members,
	// и только потом делаем INSERT.
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Один запрос вместо 3х: max_members + count members + exists membership.
	// Лок на workspaces гарантирует, что другие потоки (которые тоже лочат workspaces)
	// не смогут параллельно пройти проверку лимита и вставку.
	var (
		maxMembers int64
		cnt        int64
		exists     bool
	)
	if err := tx.QueryRow(ctx, `
		SELECT
			p.max_members,
			(SELECT COUNT(*) FROM workspace_members pm WHERE pm.workspace_id = p.id) AS cnt,
			EXISTS(SELECT 1 FROM workspace_members pm2 WHERE pm2.workspace_id = p.id AND pm2.user_id = $2) AS exists
		FROM workspaces p
		WHERE p.id = $1
		FOR UPDATE
	`, workspaceID, userID).Scan(&maxMembers, &cnt, &exists); err != nil {
		return err
	}

	if exists {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return nil
	}

	if maxMembers > 0 && cnt >= maxMembers {
		return repo.ErrWorkspaceMemberLimitReached
	}

	ct, err := tx.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
	`, workspaceID, userID, role)
	if err != nil {
		return err
	}
	// Если кто-то “в обход” успел добавить — считаем это идемпотентным успехом.
	if ct.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return nil
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// GetUserRole — возвращаем роль пользователя в проекте.
func (r *PgWorkspaceRepository) GetUserRole(
	ctx context.Context,
	userID int64,
	workspaceID int64,
) (string, error) {
	query := `
		SELECT role
		FROM workspace_members
		WHERE user_id = $1 AND workspace_id = $2;
	`

	var role string
	err := r.DB.QueryRow(ctx, query, userID, workspaceID).Scan(&role)
	return role, err
}

func (r *PgWorkspaceRepository) GetWorkspaceDomainAndUserRole(
	ctx context.Context,
	workspaceID int64,
	userID int64,
) (domain string, role string, isMember bool, err error) {
	// LEFT JOIN lets us distinguish:
	// - no rows => workspace not found
	// - role NULL => workspace exists but user is not a member
	const q = `
		SELECT p.domain, m.role
		FROM workspaces p
		LEFT JOIN workspace_members m
			ON m.workspace_id = p.id AND m.user_id = $2
		WHERE p.id = $1;
	`

	var rRole *string
	if err := r.DB.QueryRow(ctx, q, workspaceID, userID).Scan(&domain, &rRole); err != nil {
		return "", "", false, err
	}
	if rRole == nil {
		return domain, "", false, nil
	}
	return domain, *rRole, true, nil
}

func (r *PgWorkspaceRepository) GetWorkspaceByID(
	ctx context.Context,
	workspaceID int64,
) (*models.MdWorkspace, error) {

	query := `
		SELECT id, name, domain, created_at, max_members
		FROM workspaces
		WHERE id = $1;
	`

	var p models.MdWorkspace
	err := r.DB.QueryRow(ctx, query, workspaceID).Scan(
		&p.ID,
		&p.Name,
		&p.Domain,
		&p.CreatedAt,
		&p.MaxMembers,
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PgWorkspaceRepository) RemoveMember(
	ctx context.Context,
	workspaceID int64,
	userID int64,
) error {
	const q = `
		DELETE FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2;
	`

	_, err := r.DB.Exec(ctx, q, workspaceID, userID)
	return err
}

func (r *PgWorkspaceRepository) UpdateMemberRole(
	ctx context.Context,
	workspaceID int64,
	userID int64,
	role string,
) error {
	const q = `
		UPDATE workspace_members
		SET role = $3
		WHERE workspace_id = $1 AND user_id = $2;
	`

	ct, err := r.DB.Exec(ctx, q, workspaceID, userID, role)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		// mimic pgx.ErrNoRows behavior for callers that expect membership existence checks
		return pgx.ErrNoRows
	}
	return nil
}

func (r *PgWorkspaceRepository) CountMembers(
	ctx context.Context,
	workspaceID int64,
) (int64, error) {

	const q = `
		SELECT COUNT(*)
		FROM workspace_members
		WHERE workspace_id = $1;
	`

	var cnt int64
	err := r.DB.QueryRow(ctx, q, workspaceID).Scan(&cnt)
	return cnt, err
}

func (r *PgWorkspaceRepository) CreateWorkspaceInvite(
	ctx context.Context,
	invite *models.MdWorkspaceInvite,
) (*models.MdWorkspaceInvite, error) {
	const q = `
		INSERT INTO workspace_invites
			(workspace_id, created_by, token_hash, role, auth_method, oauth2_provider, expires_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, workspace_id, created_by, token_hash, role, auth_method, oauth2_provider, expires_at, used_at, used_by, created_at;
	`

	out := &models.MdWorkspaceInvite{}
	err := r.DB.QueryRow(
		ctx,
		q,
		invite.WorkspaceID,
		invite.CreatedBy,
		invite.TokenHash,
		invite.Role,
		invite.AuthMethod,
		invite.OAuth2Provider,
		invite.ExpiresAt,
	).Scan(
		&out.ID,
		&out.WorkspaceID,
		&out.CreatedBy,
		&out.TokenHash,
		&out.Role,
		&out.AuthMethod,
		&out.OAuth2Provider,
		&out.ExpiresAt,
		&out.UsedAt,
		&out.UsedBy,
		&out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PgWorkspaceRepository) GetWorkspaceInviteByHash(
	ctx context.Context,
	tokenHash string,
) (*models.MdWorkspaceInvite, error) {
	const q = `
		SELECT id, workspace_id, created_by, token_hash, role, auth_method, oauth2_provider,
		       expires_at, used_at, used_by, created_at
		FROM workspace_invites
		WHERE token_hash = $1;
	`
	out := &models.MdWorkspaceInvite{}
	if err := r.DB.QueryRow(ctx, q, tokenHash).Scan(
		&out.ID,
		&out.WorkspaceID,
		&out.CreatedBy,
		&out.TokenHash,
		&out.Role,
		&out.AuthMethod,
		&out.OAuth2Provider,
		&out.ExpiresAt,
		&out.UsedAt,
		&out.UsedBy,
		&out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PgWorkspaceRepository) AcceptWorkspaceInvite(
	ctx context.Context,
	tokenHash string,
	userID int64,
) (workspaceID int64, role string, err error) {
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1) Лочим инвайт
	var (
		invID          int64
		invWorkspaceID int64
		invRole        string
		expiresAt      time.Time
		usedAt         *time.Time
	)

	if err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, role, expires_at, used_at
		FROM workspace_invites
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(&invID, &invWorkspaceID, &invRole, &expiresAt, &usedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", repo.ErrInviteNotFound
		}
		return 0, "", err
	}

	now := time.Now()
	if usedAt != nil {
		return 0, "", repo.ErrInviteUsed
	}
	if !expiresAt.After(now) {
		return 0, "", repo.ErrInviteExpired
	}

	// 2) Лочим проект и проверяем лимит (одним запросом).
	var (
		maxMembers int64
		cnt        int64
		exists     bool
	)
	if err := tx.QueryRow(ctx, `
		SELECT
			p.max_members,
			(SELECT COUNT(*) FROM workspace_members pm WHERE pm.workspace_id = p.id) AS cnt,
			EXISTS(SELECT 1 FROM workspace_members pm2 WHERE pm2.workspace_id = p.id AND pm2.user_id = $2) AS exists
		FROM workspaces p
		WHERE p.id = $1
		FOR UPDATE
	`, invWorkspaceID, userID).Scan(&maxMembers, &cnt, &exists); err != nil {
		return 0, "", err
	}

	if exists {
		return 0, "", repo.ErrInviteAlreadyMember
	}

	if maxMembers > 0 && cnt >= maxMembers {
		return 0, "", repo.ErrWorkspaceMemberLimitReached
	}

	// 3) Добавляем участника
	ct, err := tx.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
	`, invWorkspaceID, userID, invRole)
	if err != nil {
		return 0, "", err
	}
	if ct.RowsAffected() == 0 {
		// Уже участник (например, добавили вручную) — инвайт не тратим.
		return 0, "", repo.ErrInviteAlreadyMember
	}

	// 4) Помечаем инвайт использованным (одноразовый)
	if _, err := tx.Exec(ctx, `
		UPDATE workspace_invites
		SET used_at = NOW(), used_by = $2
		WHERE id = $1
	`, invID, userID); err != nil {
		return 0, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, "", err
	}
	return invWorkspaceID, invRole, nil
}

func (r *PgWorkspaceRepository) GetWorkspaceMembers(
	ctx context.Context,
	workspaceID int64,
) ([]models.MdWorkspaceMember, error) {

	const q = `
		SELECT workspace_id, user_id, role, joined_at
		FROM workspace_members
		WHERE workspace_id = $1;
	`

	rows, err := r.DB.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.MdWorkspaceMember
	for rows.Next() {
		var m models.MdWorkspaceMember
		if err := rows.Scan(
			&m.WorkspaceID,
			&m.UserID,
			&m.Role,
			&m.JoinedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

func (r *PgWorkspaceRepository) GetWorkspaceUsers(
	ctx context.Context,
	workspaceID int64,
) ([]models.MdWorkspaceUserInfo, error) {
	const q = `
		SELECT m.user_id, u.nickname, u.is_virtual, m.role, m.joined_at
		FROM workspace_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.workspace_id = $1
		ORDER BY m.joined_at ASC;
	`

	rows, err := r.DB.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.MdWorkspaceUserInfo
	for rows.Next() {
		var row models.MdWorkspaceUserInfo
		if err := rows.Scan(&row.UserID, &row.Nickname, &row.IsVirtual, &row.Role, &row.JoinedAt); err != nil {
			return nil, err
		}
		res = append(res, row)
	}
	return res, rows.Err()
}

func (r *PgWorkspaceRepository) DeleteWorkspace(
	ctx context.Context,
	workspaceID int64,
) error {
	const q = `DELETE FROM workspaces WHERE id = $1;`
	_, err := r.DB.Exec(ctx, q, workspaceID)
	return err
}
