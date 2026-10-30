package usecase

import (
	"anubis/app/DAL/models"
	"anubis/app/DAL/repo"
	"anubis/app/DTO"
	"anubis/app/core"
	"anubis/app/core/middlewares"
	"anubis/app/core/schemes"
	"anubis/internal/utils"
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// ServiceWorkspace — бизнес-логика по проектам (добавление участников и т.п.).
type ServiceWorkspace struct {
	userRepo      repo.UserRepository
	workspaceRepo repo.WorkspaceRepository
	authRepo      repo.AuthRepository
	config        core.ServiceConfig
}

// Политика ошибок (для логов):
// - Ожидаемые/клиентские ошибки: schemes.New / schemes.Wrap (origin есть, stack нет).
// - Внутренние ошибки: schemes.WrapInternal (origin + stack; stack логируется только для 5xx).

// NewServiceWorkspace — отдельный конструктор сервиса для работы с проектами.
func NewServiceWorkspace(
	userRepo repo.UserRepository,
	workspaceRepo repo.WorkspaceRepository,
	authRepo repo.AuthRepository,
	config core.ServiceConfig,
) *ServiceWorkspace {
	return &ServiceWorkspace{
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
		authRepo:      authRepo,
		config:        config,
	}
}

// ListWorkspaceUsers — список пользователей проекта (только owner/admin).
// Возвращает участников проекта + их роль + флаг virtual + типы авторизации.
func (s *ServiceWorkspace) ListWorkspaceUsers(
	ctx *gin.Context,
	body *DTO.WorkspaceUserListValid,
) (DTO.WorkspaceUserListResponse, error) {
	var answer DTO.WorkspaceUserListResponse

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	if _, ok := s.config.ListServices[domainKey]; !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// auth по refresh + device
	session, err := s.authByRefresh(ctx, body.RefreshToken, body.DeviceID)
	if err != nil {
		return answer, err
	}
	reqCtx := ctx.Request.Context()

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + role инициатора (1 запрос вместо 2).
	workspaceDomain, initRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, session.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != session.Domain {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "user is not a member of this workspace")
	}
	initBaseRole, _ := utils.SplitRole(initRole)
	if initBaseRole != "owner" && initBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions")
	}

	// список участников
	users, err := s.workspaceRepo.GetWorkspaceUsers(reqCtx, workspaceID)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace users", err)
	}

	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.UserID)
	}

	authTypes, err := s.authRepo.GetAuthTypesByUserIDs(reqCtx, ids)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load auth types", err)
	}

	answer.Users = make([]DTO.WorkspaceUserListItem, 0, len(users))
	for _, u := range users {
		answer.Users = append(answer.Users, DTO.WorkspaceUserListItem{
			UserID:    u.UserID,
			Nickname:  u.Nickname,
			IsVirtual: u.IsVirtual,
			Role:      u.Role,
			AuthTypes: authTypes[u.UserID],
		})
	}

	return answer, nil
}

// ListWorkspaceUsersPublic — безопасный список участников проекта для всех участников (например, для чата).
// Возвращает минимум данных: user_id, nickname, role.
func (s *ServiceWorkspace) ListWorkspaceUsersPublic(
	ctx *gin.Context,
	body *DTO.WorkspaceUserListPublicValid,
) (DTO.WorkspaceUserListPublicResponse, error) {
	var answer DTO.WorkspaceUserListPublicResponse

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}
	if _, ok := s.config.ListServices[domainKey]; !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	session, err := s.authByRefresh(ctx, body.RefreshToken, body.DeviceID)
	if err != nil {
		return answer, err
	}
	reqCtx := ctx.Request.Context()

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + membership текущего пользователя.
	workspaceDomain, _, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, session.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != session.Domain {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "user is not a member of this workspace")
	}

	users, err := s.workspaceRepo.GetWorkspaceUsers(reqCtx, workspaceID)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace users", err)
	}

	answer.Users = make([]DTO.WorkspaceUserPublicItem, 0, len(users))
	for _, u := range users {
		answer.Users = append(answer.Users, DTO.WorkspaceUserPublicItem{
			UserID:   u.UserID,
			Nickname: u.Nickname,
			Role:     u.Role,
		})
	}

	return answer, nil
}

// GetRoles — справочник ролей домена (только owner/admin в проекте).
func (s *ServiceWorkspace) GetRoles(
	ctx *gin.Context,
	body *DTO.RolesValid,
) (DTO.RolesResponse, error) {
	var answer DTO.RolesResponse

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	session, err := s.authByRefresh(ctx, body.RefreshToken, body.DeviceID)
	if err != nil {
		return answer, err
	}
	reqCtx := ctx.Request.Context()

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + role инициатора.
	workspaceDomain, initRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, session.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != session.Domain {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "user is not a member of this workspace")
	}
	initBaseRole, _ := utils.SplitRole(initRole)
	if initBaseRole != "owner" && initBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions")
	}

	// конвертируем роли домена в DTO
	keys := make([]string, 0, len(svcCfg.Roles))
	for k := range svcCfg.Roles {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	answer.Roles = make([]DTO.RoleDTO, 0, len(keys))
	for _, name := range keys {
		rc := svcCfg.Roles[name]
		roleDTO := DTO.RoleDTO{
			Name:        name,
			Code:        rc.Code,
			Description: rc.Description,
		}
		if len(rc.Variants) > 0 {
			roleDTO.Variants = make(map[string]DTO.RoleVariantDTO, len(rc.Variants))
			for vName, vCfg := range rc.Variants {
				roleDTO.Variants[vName] = DTO.RoleVariantDTO{
					Code:        vCfg.Code,
					Description: vCfg.Description,
				}
			}
		}
		answer.Roles = append(answer.Roles, roleDTO)
	}

	return answer, nil
}

// CreateWorkspaceInvite — создаёт одноразовый инвайт в проект (owner/admin).
// TTL по умолчанию: 7 дней (типичный стандарт).
func (s *ServiceWorkspace) CreateWorkspaceInvite(
	ctx *gin.Context,
	body *DTO.WorkspaceInviteCreateValid,
) (DTO.WorkspaceInviteCreateResponse, error) {
	var answer DTO.WorkspaceInviteCreateResponse

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// auth по refresh + device
	session, err := s.authByRefresh(ctx, body.RefreshToken, body.DeviceID)
	if err != nil {
		return answer, err
	}
	reqCtx := ctx.Request.Context()

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// проверяем, что инициатор — owner/admin в проекте
	workspaceDomain, initRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, session.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != session.Domain {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "initiator is not a member of workspace")
	}
	initBaseRole, _ := utils.SplitRole(initRole)
	if initBaseRole != "owner" && initBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions")
	}

	// роль по умолчанию
	role := strings.TrimSpace(body.Role)
	if role == "" {
		if _, ok := svcCfg.Roles["user"]; ok {
			role = "user"
		} else if _, ok := svcCfg.Roles["guest"]; ok {
			role = "guest"
		} else {
			return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalRoleConfigMissing, "default role is not configured (expected roles.user or roles.guest)")
		}
	}
	if err := utils.ValidateRoleAgainstConfig(svcCfg.Roles, role); err != nil {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid role", err)
	}
	baseRole, _ := utils.SplitRole(role)
	if baseRole == "owner" {
		return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrInviteRoleNotAllowed, "cannot invite with owner role")
	}

	// auth_method: nil/"any" -> NULL, либо конкретный (oauth2/telegram/...)
	var authMethodPtr *string
	if body.AuthMethod != nil {
		m := strings.TrimSpace(*body.AuthMethod)
		if m != "" && m != "any" {
			// проверяем что метод разрешён в этом домене (config_server.yaml list_services.<domain>.auth)
			allowed := false
			for _, a := range svcCfg.Auth {
				if string(a) == m {
					allowed = true
					break
				}
			}
			if !allowed {
				return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrInviteAuthMethodNotAllowed, "auth_method is not allowed for this domain")
			}
			authMethodPtr = &m
		}
	}

	// oauth2_provider: только если auth_method == "oauth2"
	var oauth2ProviderPtr *string
	if body.OAuth2Provider != nil {
		p := strings.TrimSpace(*body.OAuth2Provider)
		if p != "" {
			if authMethodPtr == nil || *authMethodPtr != "oauth2" {
				return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrInviteOAuthProviderRequiresOAuth2, "oauth2_provider requires auth_method=oauth2")
			}
			// provider должен быть в provider_oauth и не быть "telegram"
			if _, ok := svcCfg.Provider[p]; !ok || p == "telegram" {
				return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrInviteOAuthProviderUnsupported, "unsupported oauth2_provider for this domain")
			}
			oauth2ProviderPtr = &p
		}
	}

	// создаём invite token
	inviteToken, err := utils.GenerateInviteToken()
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to generate invite token", err)
	}
	tokenHash := utils.HashToken(inviteToken)

	// TTL по стандарту: 7 дней
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	inv := &models.MdWorkspaceInvite{
		WorkspaceID:    workspaceID,
		CreatedBy:      session.UserID,
		TokenHash:      tokenHash,
		Role:           role,
		AuthMethod:     authMethodPtr,
		OAuth2Provider: oauth2ProviderPtr,
		ExpiresAt:      expiresAt,
	}

	if _, err := s.workspaceRepo.CreateWorkspaceInvite(reqCtx, inv); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to create invite", err)
	}

	answer.InviteToken = inviteToken
	answer.ExpiresAtUnix = expiresAt.Unix()
	answer.Role = role
	answer.AuthMethod = authMethodPtr
	answer.OAuth2Provider = oauth2ProviderPtr

	return answer, nil
}

// AddUserToWorkspace — добавить пользователя в проект.
// Доступ разрешён только при наличии валидного refresh_token.
// Логика:
// 1) Вытаскиваем domain из middleware.
// 2) Валидируем refresh (подпись, exp, iss, aud).
// 3) Ищем сессию по hash(refresh), убеждаемся, что она активна и не истекла.
// 4) Берём userID инициатора из сессии (session.UserID).
// 5) Проверяем, что проект существует и принадлежит текущему домену.
// 6) Проверяем, что инициатор имеет роль owner/admin в проекте.
// 7) Проверяем, что TargetUser существует.
// 8) Проверяем, что TargetUser ещё не состоит в проекте.
// 9) Добавляем TargetUser с заданной или дефолтной ролью.
func (s *ServiceWorkspace) AddUserToWorkspace(
	ctx *gin.Context,
	body *DTO.AddUserToWorkspaceValid,
) (schemes.EmptyResponses, error) {

	var answer schemes.EmptyResponses

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	// IMPORTANT: роли теперь расширяемые (manager:all), поэтому валидируем их по конфигу домена.
	// Достаём конфиг домена один раз и используем дальше.
	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// -------- 1) Валидируем refresh-токен --------

	var refreshClaims schemes.JwtCustomRefreshClaims
	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret, // ВАЖНО: именно RefreshTokenSecret
		&refreshClaims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	// -------- 2) Ищем сессию по hash(refresh) --------

	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}

		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// domain сессии должен совпадать с текущим доменом
	if session.Domain != bdName {
		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for workspace operation")
	}

	// Проверяем активность и срок жизни сессии.
	if !session.IsActive || session.ExpiresAt.Before(now) {
		return answer, schemes.Unauthorized(schemes.ErrAuthSessionExpiredOrInactive, "refresh session expired or inactive")
	}

	initiatorUserID := session.UserID

	// -------- 3) Валидация workspace_id и загрузка проекта --------

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		// 400: ошибка валидации входного workspace_id (клиент прислал не число/<=0).
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + роль инициатора (1 запрос вместо 2).
	workspaceDomain, initRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, initiatorUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != bdName {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "initiator is not a member of workspace")
	}

	// IMPORTANT: роль может быть с вариантом, например "admin:limited" или "manager:all".
	// Для проверок прав на уровне проекта сравниваем базовую роль.
	initBaseRole, _ := utils.SplitRole(initRole)
	if initBaseRole != "owner" && initBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions to add member")
	}

	// -------- 5) Валидация TargetUserID --------

	targetUserID, err := strconv.ParseInt(body.TargetUserID, 10, 64)
	if err != nil || targetUserID <= 0 {
		// 400: ошибка валидации target_user_id (клиент прислал не число/<=0).
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidTargetUserID, "invalid target_user_id", err)
	}

	// Можно доп. проверить, что такой юзер вообще существует:
	if _, err := s.userRepo.GetUserByID(reqCtx, targetUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceTargetUserNotFound, "target user not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to get target user", err)
	}

	// -------- 6) Проверяем, не состоит ли уже targetUser в проекте --------

	_, err = s.workspaceRepo.GetUserRole(reqCtx, targetUserID, workspaceID)
	if err == nil {
		// роль нашлась → уже участник
		return answer, schemes.Conflict(schemes.ErrWorkspaceUserAlreadyMember, "target user is already a member of workspace")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// любая другая ошибка
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to check target membership", err)
	}

	// -------- 7) Определяем роль для нового участника --------

	role := body.Role
	if role == "" {
		// Выбираем безопасный дефолт только из разрешённых ролей домена.
		// ВАЖНО: роли валидируются по конфигу, потому что в БД нет CHECK constraint.
		if _, ok := svcCfg.Roles["user"]; ok {
			role = "user"
		} else if _, ok := svcCfg.Roles["guest"]; ok {
			role = "guest"
		} else {
			return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalRoleConfigMissing, "default role is not configured (expected roles.user or roles.guest)")
		}
	}

	// IMPORTANT: since DB no longer enforces a fixed role list, validate role against config.
	if err := utils.ValidateRoleAgainstConfig(svcCfg.Roles, role); err != nil {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidRole, "invalid role", err)
	}

	// IMPORTANT (business rule): only owner can assign base-role "owner".
	newBaseRole, _ := utils.SplitRole(role)
	if newBaseRole == "owner" && initBaseRole != "owner" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceOwnerOnlyAction, "only owner can assign owner role")
	}

	// -------- 8) Добавляем участника --------

	if err := s.workspaceRepo.TryAddMemberWithLimit(reqCtx, workspaceID, targetUserID, role); err != nil {
		if errors.Is(err, repo.ErrWorkspaceMemberLimitReached) {
			return answer, schemes.Conflict(schemes.ErrWorkspaceMemberLimitReached, "workspace member limit reached")
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to add member to workspace", err)
	}

	return answer, nil
}

// UpdateUserToWorkspace — изменить роль участника в проекте.
// Правила:
// - owner/admin могут менять роли участникам;
// - НЕЛЬЗЯ менять роль текущего owner этим endpoint'ом (чтобы не осталось проекта без owner);
// - НЕЛЬЗЯ назначать owner этим endpoint'ом (перенос владения лучше сделать отдельным endpoint'ом).
func (s *ServiceWorkspace) UpdateUserToWorkspace(
	ctx *gin.Context,
	body *DTO.UpdateUserToWorkspaceValid,
) (schemes.EmptyResponses, error) {
	var answer schemes.EmptyResponses

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// 1) Валидируем refresh
	var refreshClaims schemes.JwtCustomRefreshClaims
	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret,
		&refreshClaims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	// 2) Находим сессию
	refreshHash := utils.HashToken(body.RefreshToken)
	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	if session.Domain != bdName {
		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for workspace operation")
	}
	if !session.IsActive || session.ExpiresAt.Before(now) {
		return answer, schemes.Unauthorized(schemes.ErrAuthSessionExpiredOrInactive, "refresh session expired or inactive")
	}

	initiatorUserID := session.UserID

	// 3) workspace_id + проект
	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}
	// Комбайн: workspace.domain + роль инициатора.
	workspaceDomain, initRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, initiatorUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != bdName {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "initiator is not a member of workspace")
	}
	initBaseRole, _ := utils.SplitRole(initRole)
	if initBaseRole != "owner" && initBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions to update member role")
	}

	// 5) target user
	targetUserID, err := strconv.ParseInt(body.TargetUserID, 10, 64)
	if err != nil || targetUserID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidTargetUserID, "invalid target_user_id", err)
	}

	// target должен быть участником
	targetRole, err := s.workspaceRepo.GetUserRole(reqCtx, targetUserID, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceMemberNotFound, "target user is not a member of workspace", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to get target user role in workspace", err)
	}
	targetBaseRole, _ := utils.SplitRole(targetRole)
	if targetBaseRole == "owner" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceOwnerRoleImmutable, "cannot change owner role via update_member")
	}

	// 6) Validate new role and apply business constraints
	newRole := body.Role
	if err := utils.ValidateRoleAgainstConfig(svcCfg.Roles, newRole); err != nil {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidRole, "invalid role", err)
	}
	newBaseRole, _ := utils.SplitRole(newRole)
	if newBaseRole == "owner" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceOwnerRoleImmutable, "cannot assign owner role via update_member")
	}

	// 7) Update role
	if err := s.workspaceRepo.UpdateMemberRole(reqCtx, workspaceID, targetUserID, newRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceMemberNotFound, "target user is not a member of workspace", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to update member role", err)
	}

	return answer, nil
}

func (s *ServiceWorkspace) RemoveUserFromWorkspace(
	ctx *gin.Context,
	body *DTO.WorkspaceRemoveUserValid,
) (schemes.EmptyResponses, error) {

	var answer schemes.EmptyResponses

	// ---------- 0) Достаём domain + конфиг ----------

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	_ = svcCfg // пока не нужен, но может пригодиться позже
	reqCtx := ctx.Request.Context()
	now := time.Now()

	// ---------- 1) Валидация refresh-токена (ровно как в RefreshTokenDomainFlow) ----------

	var refreshClaims schemes.JwtCustomRefreshClaims

	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret,
		&refreshClaims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}

		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// Проверяем домен сессии.
	if session.Domain != bdName {
		_ = s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID})
		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for refresh token")
	}

	// Проверяем device_id (биндинг к устройству).
	if session.DeviceID == "" || session.DeviceID != body.DeviceID {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for refresh token")
	}

	// Проверяем активность/срок жизни сессии.
	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthRefreshExpiredOrReused, "refresh token expired or reused")
	}

	currentUserID := session.UserID // тот, кто вызывает удаление

	// ---------- 2) Парсим workspace_id и target_user_id ----------

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	targetUserID, err := strconv.ParseInt(body.TargetUserID, 10, 64)
	if err != nil || targetUserID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidTargetUserID, "invalid target_user_id", err)
	}

	// ---------- 3) Проверяем роль текущего юзера в проекте (кто вызывает) ----------

	workspaceDomain, currentRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, currentUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != bdName {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "current user has no access to this workspace")
	}

	// Разрешаем удалять только owner/admin (подгони под свои роли, если они кодированы).
	currentBaseRole, _ := utils.SplitRole(currentRole)
	if currentBaseRole != "owner" && currentBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions to remove users from workspace")
	}

	// ---------- 4) Проверяем роль целевого юзера (кого кикаем) ----------

	targetRole, err := s.workspaceRepo.GetUserRole(reqCtx, targetUserID, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Юзер вообще не состоит в проекте — считаем, что и так ок (idempotent).
			return answer, nil
		}

		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to get target user role in workspace", err)
	}

	// НЕЛЬЗЯ удалять owner'а.
	targetBaseRole, _ := utils.SplitRole(targetRole)
	if targetBaseRole == "owner" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceOwnerOnlyAction, "cannot remove owner from workspace")
	}

	// (опционально) можно запретить кикать самого себя, если не хочешь self-leave логику
	// if targetUserID == currentUserID {

	// }

	// ---------- 5) Удаляем участника ----------

	if err := s.workspaceRepo.RemoveMember(reqCtx, workspaceID, targetUserID); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to remove user from workspace", err)
	}

	return answer, nil
}

func (s *ServiceWorkspace) CreateVirtualUserInWorkspace(
	ctx *gin.Context,
	body *DTO.WorkspaceCreateVirtualUserValid,
) (DTO.WorkspaceCreateVirtualUserResponse, error) {

	var answer DTO.WorkspaceCreateVirtualUserResponse

	// ---------- 0) Достаём domain + конфиг ----------

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// 🔹 Если SingleMember включён — запретить создание доп. пользователей.
	if svcCfg.SingleMember {
		return answer, schemes.Conflict(schemes.ErrworkspacesingleMemberOnly, "this service/workspace is single-member, cannot create additional users")
	}

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// ---------- 1) Валидация refresh-токена (копипаста из RefreshTokenDomainFlow) ----------

	var refreshClaims schemes.JwtCustomRefreshClaims

	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret,
		&refreshClaims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}

		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// Проверяем домен сессии.
	if session.Domain != bdName {
		_ = s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID})
		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for refresh token")
	}

	// Проверяем device_id (биндинг к устройству).
	if session.DeviceID == "" || session.DeviceID != body.DeviceID {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for refresh token")
	}

	// Проверяем активность/срок жизни сессии.
	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthRefreshExpiredOrReused, "refresh token expired or reused")
	}

	currentUserID := session.UserID // кто создаёт виртуального юзера

	// ---------- 2) Парсим workspace_id ----------

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + роль текущего пользователя (1 запрос вместо 2).
	workspaceDomain, currentRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, currentUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != bdName {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "current user has no access to this workspace")
	}

	// Разрешаем создавать виртуальных только owner/admin (подгони под свою систему ролей).
	currentBaseRole, _ := utils.SplitRole(currentRole)
	if currentBaseRole != "owner" && currentBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions to create virtual users in workspace")
	}

	// ---------- 4) SingleMember-проверка на уровне проекта (опционально) ----------

	// Если хочешь именно "один участник в проекте" — можно ещё проверить количество мемберов:
	// membersCount, err := s.workspaceRepo.CountMembers(reqCtx, workspaceID)
	// if err != nil {
	// 	return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to count workspace members", err)
	// }
	// if membersCount >= 1 && svcCfg.SingleMember {

	// }

	// ---------- 5) Проверяем уникальность логина (опционально) ----------

	// Если есть уникальный индекс, то можно просто ловить ошибку из БД, но
	// мягко можем проверить заранее.
	if _, err := s.authRepo.GetVirtual(reqCtx, body.Login); err == nil {
		return answer, schemes.Conflict(schemes.ErrWorkspaceVirtualLoginAlreadyExists, "virtual login already exists")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to check virtual login", err)
	}

	// ---------- 6) Создаём пользователя (MdUser с IsVirtual = true) ----------

	nickname := body.Login                                      // можно сгенерить по-другому
	user, err := s.userRepo.CreateUserVirtual(reqCtx, nickname) // 🔹 новый метод
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to create virtual user", err)
	}

	// ---------- 7) Хэш пароля и создание auth_virtual ----------

	passwordHash, err := utils.GeneratePasswordHash(body.Password) // используй свой helper
	if err != nil {
		_ = s.userRepo.DeleteUser(reqCtx, user.ID) // best-effort cleanup
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to hash password", err)
	}

	virtualAuth := &models.MdAuthVirtual{
		UserID:       user.ID,
		Login:        body.Login,
		PasswordHash: passwordHash,
		Verification: true,
		CreatedAt:    time.Now(),
	}

	if err := s.authRepo.CreateVirtualAuth(reqCtx, virtualAuth); err != nil {
		_ = s.userRepo.DeleteUser(reqCtx, user.ID) // best-effort cleanup
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to create virtual auth", err)
	}

	// ---------- 8) Добавляем виртуального пользователя в проект ----------

	role := body.Role
	if role == "" {
		role = "user" // дефолтная роль, если не передали
	}

	// IMPORTANT: validate role (including variants like manager:all) against domain config.
	if err := utils.ValidateRoleAgainstConfig(svcCfg.Roles, role); err != nil {
		_ = s.deleteVirtualUserFull(reqCtx, user.ID) // best-effort cleanup
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidRole, "invalid role", err)
	}

	if err := s.workspaceRepo.TryAddMemberWithLimit(reqCtx, workspaceID, user.ID, role); err != nil {
		_ = s.deleteVirtualUserFull(reqCtx, user.ID) // best-effort cleanup
		if errors.Is(err, repo.ErrWorkspaceMemberLimitReached) {
			return answer, schemes.Conflict(schemes.ErrWorkspaceMemberLimitReached, "workspace member limit reached")
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to add virtual user to workspace", err)
	}

	// ---------- 9) Формируем ответ ----------

	answer.UserID = user.ID
	answer.Login = body.Login

	return answer, nil
}

func (s *ServiceWorkspace) UpdateVirtualUserInWorkspace(
	ctx *gin.Context,
	body *DTO.WorkspaceUpdateVirtualUserValid,
) (DTO.WorkspaceUpdateVirtualUserResponse, error) {

	var answer DTO.WorkspaceUpdateVirtualUserResponse

	// ---------- 0) domain + config ----------

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	_, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// SingleMember не запрещает изменять существующего виртуала,
	// только создание. Тут проверки не делаем.

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// ---------- 1) Валидация refresh-токена (ровно как раньше) ----------

	var refreshClaims schemes.JwtCustomRefreshClaims

	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret,
		&refreshClaims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}

		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	if session.Domain != bdName {
		_ = s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID})
		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for refresh token")
	}

	if session.DeviceID == "" || session.DeviceID != body.DeviceID {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)
		return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for refresh token")
	}

	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)
		return answer, schemes.Unauthorized(schemes.ErrAuthRefreshExpiredOrReused, "refresh token expired or reused")
	}

	currentUserID := session.UserID

	// ---------- 2) workspace_id + роль текущего пользователя ----------

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + роль текущего пользователя.
	workspaceDomain, currentRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, currentUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != bdName {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "current user has no access to this workspace")
	}

	currentBaseRole, _ := utils.SplitRole(currentRole)
	if currentBaseRole != "owner" && currentBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions to update virtual users")
	}

	// ---------- 3) Проверяем, что target user — участник проекта и виртуальный ----------

	targetUser, err := s.userRepo.GetUserByID(reqCtx, body.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceTargetUserNotFound, "target user not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load target user", err)
	}

	if !targetUser.IsVirtual {
		return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrWorkspaceTargetUserNotVirtual, "target user is not virtual")
	}

	// Убедимся, что этот пользователь состоит в проекте
	if _, err := s.workspaceRepo.GetUserRole(reqCtx, targetUser.ID, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceMemberNotFound, "target user is not a member of this workspace", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to get target user role", err)
	}

	// ---------- 4) Загружаем auth_virtual и обновляем ----------

	virtualAuth, err := s.authRepo.GetVirtualByUserID(reqCtx, targetUser.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, 103, "virtual auth not found for target user", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load virtual auth", err)
	}

	// Меняем логин (если указан)
	if body.NewLogin != nil && *body.NewLogin != "" {
		// если логин не меняется — ничего не делаем
		if *body.NewLogin == virtualAuth.Login {
			// noop
		} else {
			// опционально: проверка уникальности логина
			if found, err := s.authRepo.GetVirtual(reqCtx, *body.NewLogin); err == nil {
				// если нашли тот же самый аккаунт — это ок (например, повторный вызов)
				if found.UserID != targetUser.ID {
					return answer, schemes.Conflict(schemes.ErrWorkspaceVirtualLoginAlreadyExists, "virtual login already exists")
				}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to check virtual login", err)
			}

			virtualAuth.Login = *body.NewLogin
			targetUser.Nickname = *body.NewLogin
		}
	}

	// Меняем пароль (если указан)
	if body.NewPassword != nil && *body.NewPassword != "" {
		passwordHash, err := utils.GeneratePasswordHash(*body.NewPassword)
		if err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to hash new password", err)
		}
		virtualAuth.PasswordHash = passwordHash
	}

	if err := s.authRepo.UpdateVirtualAuth(reqCtx, virtualAuth); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to update virtual auth", err)
	}

	// Обновляем nickname в users, если нужно
	if err := s.userRepo.UpdateUser(reqCtx, targetUser); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to update user", err)
	}

	answer.UserID = targetUser.ID
	answer.Login = virtualAuth.Login

	return answer, nil
}

func (s *ServiceWorkspace) DeleteVirtualUserInWorkspace(
	ctx *gin.Context,
	body *DTO.WorkspaceDeleteVirtualUserValid,
) (DTO.WorkspaceDeleteVirtualUserResponse, error) {

	var answer DTO.WorkspaceDeleteVirtualUserResponse

	// ---------- 0) domain + config ----------

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	_, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// ---------- 1) Валидация refresh-токена (как всегда) ----------

	var refreshClaims schemes.JwtCustomRefreshClaims

	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret,
		&refreshClaims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	if session.Domain != bdName {
		_ = s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID})
		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for refresh token")
	}

	if session.DeviceID == "" || session.DeviceID != body.DeviceID {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)
		return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for refresh token")
	}

	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)
		return answer, schemes.Unauthorized(schemes.ErrAuthRefreshExpiredOrReused, "refresh token expired or reused")
	}

	currentUserID := session.UserID

	// ---------- 2) workspace_id и права текущего пользователя ----------

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Комбайн: workspace.domain + роль текущего пользователя.
	workspaceDomain, currentRole, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, currentUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != bdName {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "current user has no access to this workspace")
	}

	currentBaseRole, _ := utils.SplitRole(currentRole)
	if currentBaseRole != "owner" && currentBaseRole != "admin" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "insufficient permissions to delete virtual users")
	}

	// ---------- 3) Загружаем target user, проверяем виртуальность и членство ----------

	targetUser, err := s.userRepo.GetUserByID(reqCtx, body.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceTargetUserNotFound, "target user not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load target user", err)
	}

	if !targetUser.IsVirtual {
		return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrWorkspaceTargetUserNotVirtual, "target user is not virtual")
	}

	// Не даём удалить владельца проекта (owner), даже если он виртуальный.
	targetRole, err := s.workspaceRepo.GetUserRole(reqCtx, targetUser.ID, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceMemberNotFound, "target user is not a member of this workspace", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to get target user role", err)
	}
	targetBaseRole, _ := utils.SplitRole(targetRole)
	if targetBaseRole == "owner" {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceCannotDeleteOwner, "cannot delete workspace owner")
	}

	// ---------- 4) Удаляем из workspace_members ----------

	if err := s.workspaceRepo.RemoveMember(reqCtx, workspaceID, targetUser.ID); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to remove user from workspace", err)
	}

	// Если виртуальный юзер может быть в других проектах — можно тут
	// проверить CountworkspacesByUser и удалять только если count == 0.
	// Предположим, что виртуал используется только в одном проекте → удаляем полностью.

	// ---------- 5) Чистим auth_virtual, сессии и самого юзера ----------

	if err := s.authRepo.DeleteVirtualByUserID(reqCtx, targetUser.ID); err != nil {
		// не фатально, но лучше вернуть ошибку
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to delete virtual auth", err)
	}

	// Деактивируем все его сессии (на всякий случай, если он уже логинился).
	_ = s.userRepo.DeactivateSessions(reqCtx, targetUser.ID)

	if err := s.userRepo.DeleteUser(reqCtx, targetUser.ID); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to delete user", err)
	}

	answer.Deleted = true
	return answer, nil
}

// helper auth по refresh_token + device_id, возвращает userID и session
func (s *ServiceWorkspace) authByRefresh(
	ctx *gin.Context,
	refreshToken string,
	deviceID string,
) (*models.MdUserSession, error) {

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return nil, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	reqCtx := ctx.Request.Context()
	now := time.Now()

	var claims schemes.JwtCustomRefreshClaims
	if err := utils.ExtractToken(
		refreshToken,
		s.config.JWT.RefreshTokenSecret,
		&claims,
		s.config.App.URL,
		[]string{s.config.App.URL},
	); err != nil {
		return nil, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	hash := utils.HashToken(refreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}
		return nil, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to find session by refresh", err)
	}

	if session.Domain != bdName {
		_ = s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID})
		return nil, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch")
	}

	if session.DeviceID == "" || session.DeviceID != deviceID {
		_ = s.userRepo.DeactivateSessionsByDevice(reqCtx, session.UserID, session.Domain, session.DeviceID)
		return nil, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for refresh token")
	}
	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessionsByDevice(reqCtx, session.UserID, session.Domain, session.DeviceID)
		return nil, schemes.Unauthorized(schemes.ErrAuthRefreshExpiredOrReused, "refresh token expired or reused")
	}

	return session, nil
}

// helper: удалить виртуального пользователя (сессии + auth_virtual + user)
func (s *ServiceWorkspace) deleteVirtualUserFull(
	ctx context.Context,
	userID int64,
) error {
	// выключаем все сессии (во всех доменах)
	_ = s.userRepo.DeactivateSessions(ctx, userID)

	// удаляем virtual auth
	if err := s.authRepo.DeleteVirtualByUserID(ctx, userID); err != nil {
		return err
	}

	// удаляем самого юзера
	if err := s.userRepo.DeleteUser(ctx, userID); err != nil {
		return err
	}

	return nil
}

func (s *ServiceWorkspace) CreateWorkspace(
	ctx *gin.Context,
	body *DTO.CreateWorkspaceValid,
) (DTO.WorkspaceInfo, error) {

	var answer DTO.WorkspaceInfo

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// если включён SingleWorkspaceOnly — проекты не создаём вообще
	if svcCfg.SingleWorkspace {
		return answer, schemes.Forbidden(schemes.ErrWorkspaceCreationDisabledSingleWorkspace, "workspace creation disabled for this service (single workspace only)")
	}

	// auth по refresh + device
	session, err := s.authByRefresh(ctx, body.RefreshToken, body.DeviceID)
	if err != nil {
		return answer, err
	}
	reqCtx := ctx.Request.Context()
	userID := session.UserID

	// смотрим сколько проектов уже есть у пользователя
	_, err = s.workspaceRepo.GetworkspacesByUserInDomain(reqCtx, userID, session.Domain)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load user workspaces", err)
	}

	// (если захочешь лимит на количество проектов — можно добавить тут)

	// Лимит участников для нового проекта берём из конфига домена и записываем в БД.
	// Если в конфиге не задано — оставляем дефолт 10.
	maxMembers := 10
	if svcCfg.DefaultMaxMembers != nil && *svcCfg.DefaultMaxMembers > 0 {
		maxMembers = *svcCfg.DefaultMaxMembers
	}

	newProj := &models.MdWorkspace{
		Name:       body.Name,
		Domain:     session.Domain,
		MaxMembers: maxMembers,
	}

	created, err := s.workspaceRepo.CreateWorkspace(reqCtx, newProj, userID)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "couldn't create workspace", err)
	}

	answer.ID = created.ID
	answer.Name = created.Name
	answer.MaxMembers = created.MaxMembers

	return answer, nil
}

func (s *ServiceWorkspace) DeleteWorkspace(
	ctx *gin.Context,
	body *DTO.DeleteWorkspaceValid,
) error {

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainKey]
	if !ok {
		return schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// SingleWorkspaceOnly → запрет на удаление проектов
	if svcCfg.SingleWorkspace {
		return schemes.Forbidden(schemes.ErrWorkspaceDeletionDisabledSingleWorkspace, "workspace deletion disabled for this service (single workspace only)")
	}

	// auth
	session, err := s.authByRefresh(ctx, body.RefreshToken, body.DeviceID)
	if err != nil {
		return err
	}
	reqCtx := ctx.Request.Context()
	userID := session.UserID

	// парсим workspace_id
	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		return schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// проверяем роль в проекте
	workspaceDomain, role, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemes.WrapKind(schemes.KindNotFound, schemes.ErrWorkspaceNotFound, "workspace not found", err)
		}
		return schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace/role", err)
	}
	if workspaceDomain != session.Domain {
		return schemes.Forbidden(schemes.ErrWorkspaceDomainMismatch, "workspace domain mismatch")
	}
	if !isMember {
		return schemes.Forbidden(schemes.ErrWorkspaceNotMember, "user is not a member of this workspace")
	}

	// только owner может удалить (если хочешь добавить admin — просто расширь проверку)
	baseRole, _ := utils.SplitRole(role)
	if baseRole != "owner" {
		return schemes.Forbidden(schemes.ErrWorkspaceOnlyOwnerCanDelete, "only owner can delete workspace")
	}

	// проверяем, сколько проектов у юзера
	userworkspaces, err := s.workspaceRepo.GetworkspacesByUserInDomain(reqCtx, userID, session.Domain)
	if err != nil {
		return schemes.WrapInternal(schemes.ErrInternalDB, "failed to load user workspaces", err)
	}

	if len(userworkspaces) <= 1 {
		// нельзя удалить последний проект
		return schemes.Conflict(schemes.ErrWorkspaceCannotDeleteLastWorkspace, "cannot delete the last workspace")
	}

	// загружаем участников проекта
	members, err := s.workspaceRepo.GetWorkspaceUsers(reqCtx, workspaceID)
	if err != nil {
		return schemes.WrapInternal(schemes.ErrInternalDB, "failed to load workspace members", err)
	}

	// для каждого участника проверяем, виртуал ли он, и если да — удаляем
	for _, m := range members {
		if m.IsVirtual {
			if err := s.deleteVirtualUserFull(reqCtx, m.UserID); err != nil {
				return schemes.WrapInternal(schemes.ErrInternalDB, "failed to delete virtual user", err)
			}
		}
	}

	// наконец, удаляем сам проект
	if err := s.workspaceRepo.DeleteWorkspace(reqCtx, workspaceID); err != nil {
		return schemes.WrapInternal(schemes.ErrInternalDB, "failed to delete workspace", err)
	}

	return nil
}
