package usecase

import (
	"anubis/app/DAL/models"
	"anubis/app/DAL/repo"
	"anubis/app/DTO"
	"anubis/app/api/helpers"
	"anubis/app/core"
	"anubis/app/core/common"
	"anubis/app/core/middlewares"
	"anubis/app/core/schemes"
	"anubis/internal/utils"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

// ServiceToken отвечает за бизнес-логику Token refresh и logaut:
type ServiceToken struct {
	// Репозиторий пользователей (users).
	userRepo repo.UserRepository

	// Репозиторий проектов (workspaces, workspace_members).
	workspaceRepo repo.WorkspaceRepository

	// Репозиторий авторизаций (auth/email/phone/oauth/...).
	authRepo repo.AuthRepository

	// Глобальный конфиг сервиса (list_services, jwt, storage и т.д.).
	config core.ServiceConfig
}

// Политика ошибок (для логов):
// - Ожидаемые/клиентские ошибки: schemes.New / schemes.Wrap (origin есть, stack нет).
// - Внутренние ошибки: schemes.WrapInternal (origin + stack; stack логируется только для 5xx).

// NewServiceToken — конструктор сервиса OAuth2.
// СЮДА мы передаём интерфейсы репозиториев, а не конкретные Mongo/Pg реализации.
// Это позволит позже легко заменить Mongo-репы на Pg-репы, не меняя код бизнес-логики.
func NewServiceToken(
	userRepo repo.UserRepository,
	workspaceRepo repo.WorkspaceRepository,
	authRepo repo.AuthRepository,
	config core.ServiceConfig,
) *ServiceToken {
	return &ServiceToken{
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
		authRepo:      authRepo,
		config:        config,
	}
}

// ExchangeTicketForTokens обменивает одноразовый login-ticket на пару
// (refresh token + access token), привязанную к конкретному пользователю,
// проекту и девайсу.
//
// Поток логики:
//  1. Берём domain из middleware (middlewares.Domain).
//  2. Валидируем и парсим ticket:
//     - проверяем подпись (HMAC),
//     - проверяем exp/iat,
//     - проверяем issuer/audience,
//     - сверяем, что ticket.Domain == текущему domain,
//     - сверяем, что ticket.DeviceID == body.DeviceID
//     (ЭТО ЧАСТЬ ЗАЩИТЫ ЧЕРЕЗ state).
//  3. Проверяем, что user_id из тикета действительно состоит в workspace_id,
//     и забираем его роль в этом проекте.
//  4. Загружаем все активные сессии юзера в этом домене,
//     считаем, сколько разных девайсов сейчас активно.
//     - деактивируем все старые сессии текущего девайса (device_id)
//     (ротация токена на устройстве);
//     - если девайсов > MaxDevicesUser, выкидываем самый старый девайс
//     (деактивируем все его сессии).
//     ВАЖНО: деактивацию делаем ОДНИМ UPDATE по списку IDs.
//  5. Генерируем opaque refresh-токен (без session_id внутри),
//     считаем его хэш, создаём запись user_sessions с этим hash_token.
//     Считаем время жизни сессии по RefreshTokenMinute.
//     Если запись создать не удалось — токены НЕ отдаём.
//  6. Генерируем access-токен, в котором:
//     - subject = user_id,
//     - aud = [domain],
//     - в поле D лежит строка "userID|workspaceID|role".
//  7. Возвращаем пару {refresh_token, access_token}.

func (s *ServiceToken) ExchangeTicketForTokens(ctx *gin.Context, body *DTO.ExchangeTicketValid) (DTO.AnswerExchangeTicketToken, error) {

	var answer DTO.AnswerExchangeTicketToken

	// ------------ 0) Вытаскиваем domain из middleware ------------

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

	// ------------ 1) Валидируем и парсим ticket ------------

	var claims helpers.LoginTicketClaims

	// Тут мы:
	//   - проверяем подпись по RefreshTokenSecret,
	//   - проверяем exp/iat (делает jwt сам),
	//   - проверяем issuer == s.config.App.URL,
	//   - проверяем audience содержит s.config.App.URL,
	//   - разрешаем только HS256.
	if err := utils.ExtractToken(
		body.Ticket,
		s.config.JWT.TicketTokenSecret,
		&claims,
		s.config.App.URL,           // expectedIssuer
		[]string{s.config.App.URL}, // expectedAudience
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "invalid ticket", err)
	}

	// 1.1) Домен в тикете должен совпадать с доменом из контекста.
	if claims.Domain != domainKey {
		return answer, schemes.Forbidden(schemes.ErrAuthTicketDomainMismatch, "domain mismatch in ticket")
	}

	// 1.2) DeviceID в тикете должен совпадать с тем, что прислал клиент сейчас.
	// Это как раз "биндинг" тикета к конкретному девайсу:
	// атакер может подсунуть жертве ссылку с СВОИМ state/device_id,
	// но когда жертва придёт на /exchange, она отправит СВОЙ device_id,
	// и проверка ниже завалится.
	if claims.DeviceID == "" || claims.DeviceID != body.DeviceID {
		return answer, schemes.Forbidden(schemes.ErrAuthTicketDeviceMismatch, "device_id mismatch in ticket (state protection)")
	}

	// 1.3) Парсим user_id из subject.
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 {
		// 401: тикет повреждён/подменён (не должен содержать нечисловой user_id).
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidTicket, "invalid user id in ticket", err)
	}

	// -------------------- 3) Грузим проекты пользователя --------------------

	// Если в ticket пришёл invite_token — пытаемся прикрепить пользователя к проекту
	// ДО того, как отдавать list_workspaces.
	if strings.TrimSpace(claims.InviteToken) != "" {
		inviteHash := utils.HashToken(strings.TrimSpace(claims.InviteToken))

		inv, err := s.workspaceRepo.GetWorkspaceInviteByHash(reqCtx, inviteHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return answer, schemes.NotFound(schemes.ErrInviteNotFound, "invite not found")
			}
			return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load invite", err)
		}

		now := time.Now()
		if inv.UsedAt != nil {
			return answer, schemes.Conflict(schemes.ErrInviteAlreadyUsed, "invite already used")
		}
		if !inv.ExpiresAt.After(now) {
			return answer, schemes.Gone(schemes.ErrInviteExpired, "invite expired")
		}

		// Проверяем ограничение по типу авторизации.
		if inv.AuthMethod != nil && strings.TrimSpace(*inv.AuthMethod) != "" {
			if strings.TrimSpace(claims.AuthMethod) == "" || strings.TrimSpace(*inv.AuthMethod) != strings.TrimSpace(claims.AuthMethod) {
				return answer, schemes.Forbidden(schemes.ErrInviteAuthMethodMismatch, "invite auth_method mismatch")
			}
			// oauth2_provider проверяем только для oauth2
			if *inv.AuthMethod == "oauth2" && inv.OAuth2Provider != nil && strings.TrimSpace(*inv.OAuth2Provider) != "" {
				if strings.TrimSpace(claims.AuthProvider) == "" || strings.TrimSpace(*inv.OAuth2Provider) != strings.TrimSpace(claims.AuthProvider) {
					return answer, schemes.Forbidden(schemes.ErrInviteOAuthProviderMismatch, "invite oauth2_provider mismatch")
				}
			}
		}

		// Применяем инвайт атомарно (добавление участника + пометка used).
		if _, _, err := s.workspaceRepo.AcceptWorkspaceInvite(reqCtx, inviteHash, userID); err != nil {
			switch {
			case errors.Is(err, repo.ErrInviteNotFound):
				return answer, schemes.NotFound(schemes.ErrInviteNotFound, "invite not found")
			case errors.Is(err, repo.ErrInviteExpired):
				return answer, schemes.Gone(schemes.ErrInviteExpired, "invite expired")
			case errors.Is(err, repo.ErrInviteUsed):
				return answer, schemes.Conflict(schemes.ErrInviteAlreadyUsed, "invite already used")
			case errors.Is(err, repo.ErrInviteAlreadyMember):
				return answer, schemes.Conflict(schemes.ErrWorkspaceUserAlreadyMember, "user is already a workspace member")
			case errors.Is(err, repo.ErrWorkspaceMemberLimitReached):
				return answer, schemes.Conflict(schemes.ErrWorkspaceMemberLimitReached, "workspace member limit reached")
			default:
				return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to accept invite", err)
			}
		}
	}

	workspaces, err := s.workspaceRepo.GetworkspacesByUserInDomain(reqCtx, userID, bdName)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "Couldn't load workspaces for user", err)
	}

	answer.Listworkspaces = make(map[string]string, len(workspaces))

	// Если домен допускает только один проект и у пользователя пока ни одного нет — создаём первый
	if len(workspaces) == 0 {
		// Если пользователь пришёл по invite — не создаём ему "личный проект" автоматически.
		// Это помогает избегать мусорных проектов, когда вход должен был быть только по приглашению.
		if strings.TrimSpace(claims.InviteToken) != "" {
			// Бизнес-ошибка: пользователь не может работать без доступных проектов в домене.
			return answer, schemes.Forbidden(schemes.ErrWorkspaceNotMember, "no workspaces available for user (invite not applied)")
		}
		newProj := &models.MdWorkspace{
			Name:   utils.GenerateWorkspaceName(),
			Domain: bdName,
		}

		// Лимит участников нового проекта берём из конфига домена.
		// Если в конфиге не задано — используем дефолт 10.
		maxMembers := 10
		if svcCfg.DefaultMaxMembers != nil && *svcCfg.DefaultMaxMembers > 0 {
			maxMembers = *svcCfg.DefaultMaxMembers
		}
		newProj.MaxMembers = maxMembers

		created, err := s.workspaceRepo.CreateWorkspace(reqCtx, newProj, userID)

		if err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalDB, "Couldn't create default workspace", err)
		}

		answer.Listworkspaces[strconv.FormatInt(created.ID, 10)] = created.Name
	} else {
		// Иначе просто заполняем список всех проектов
		for _, p := range workspaces {
			idStr := strconv.FormatInt(p.ID, 10)
			answer.Listworkspaces[idStr] = p.Name
		}
	}

	// ------------ 3) Политика по сессиям и девайсам (макс. N девайсов) ------------

	// Забираем все активные сессии юзера в рамках домена.
	activeSessions, err := s.userRepo.GetActiveSessionsByUserAndDomain(
		reqCtx,
		userID,
		bdName,
	)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load active sessions", err)
	}

	now := time.Now()

	// Для подсчёта девайсов и списков на деактивацию.
	deviceSet := make(map[string]time.Time) // device_id -> самый ранний created_at
	var sessionsToDeactivate []int64        // сюда соберём ВСЕ сессии, которые надо выключить одним UPDATE

	for _, sss := range activeSessions {
		// 3.1) сразу собираем все сессии текущего девайса — их мы ротируем.
		if sss.DeviceID == body.DeviceID {
			sessionsToDeactivate = append(sessionsToDeactivate, sss.ID)
		}

		// 3.2) собираем минимальный created_at по каждому device_id для поиска "самого старого девайса".
		if t, ok := deviceSet[sss.DeviceID]; !ok || sss.CreatedAt.Before(t) {
			deviceSet[sss.DeviceID] = sss.CreatedAt
		}
	}

	// Считаем, сколько уникальных девайсов сейчас активно (без учёта новой сессии).
	activeDevicesCount := len(deviceSet)

	// Мы всегда создаём новую сессию для текущего device_id,
	// поэтому считаем, сколько будет после этого.
	// Если device_id новый — девайсов станет на 1 больше.
	if _, exists := deviceSet[body.DeviceID]; !exists {
		activeDevicesCount++
	}

	// Если лимит по девайсам превышен — нужно выкинуть самый старый девайс (кроме текущего).
	if svcCfg.MaxDevicesUser > 0 && activeDevicesCount > svcCfg.MaxDevicesUser {
		var oldestDeviceID string
		var oldestTime time.Time

		for devID, created := range deviceSet {
			if devID == body.DeviceID {
				// этот девайс мы не выкидываем — именно на него сейчас делаем новую сессию
				continue
			}
			if oldestDeviceID == "" || created.Before(oldestTime) {
				oldestDeviceID = devID
				oldestTime = created
			}
		}

		// Если нашли старый девайс — помечаем все его сессии к деактивации.
		if oldestDeviceID != "" {
			for _, sss := range activeSessions {
				if sss.DeviceID == oldestDeviceID {
					sessionsToDeactivate = append(sessionsToDeactivate, sss.ID)
				}
			}
		}
	}

	// Если есть что деактивировать — делаем это одним запросом.
	if len(sessionsToDeactivate) > 0 {
		if err := s.userRepo.DeactivateSessionsByIDs(reqCtx, sessionsToDeactivate); err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to deactivate old sessions", err)
		}
	}

	// ------------ 4) Генерируем opaque refresh-токен и создаём сессию ------------

	// Важно: мы не кладём session_id внутрь refresh-токена.
	// Refresh — просто случайный JWT, по которому мы на сервере ничего
	// напрямую не узнаем. Мы считаем его хэш и ищем по хэшу.
	refreshTTL := time.Duration(s.config.JWT.RefreshTokenMinute) * time.Minute
	expiresAt := now.Add(refreshTTL)

	// Пока твой CreateRefreshToken ожидает idSession, но мы не хотим его использовать,
	// поэтому передаём пустую строку. Потом можно будет переписать сам helper.
	refreshToken, err := utils.CreateRefreshToken(
		"", // idSession сейчас не используем, opaque refresh
		s.config.JWT.RefreshTokenSecret,
		s.config.App.URL,
		[]string{s.config.App.URL},
		s.config.JWT.RefreshTokenMinute,
	)
	if err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "couldn't create refresh token", err)
	}

	hash := utils.HashToken(refreshToken)

	newSession := &models.MdUserSession{
		UserID:     userID,
		Domain:     bdName,
		DeviceID:   body.DeviceID,
		HashToken:  hash,
		DeviceType: common.GetDeviceType(ctx),
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
		IP:         common.GetClientIP(ctx),
		IsActive:   true,
	}

	// Пытаемся сохранить сессию. Если не получилось — НИЧЕГО не отдаём наружу.
	if err := s.userRepo.CreateUserSession(reqCtx, newSession); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "couldn't persist session", err)
	}

	answer.RefreshToken = refreshToken

	return answer, nil
}

// VirtualLogin — вход виртуального пользователя по логину/паролю.
// Возвращает login-ticket, который потом обменивается на refresh_token через /auth/exchange_ticket.
func (s *ServiceToken) VirtualLogin(ctx *gin.Context, body *DTO.VirtualLoginValid) (DTO.AnswerTicketToken, error) {
	var answer DTO.AnswerTicketToken

	domainKey := ctx.GetString(middlewares.Domain)
	if domainKey == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}
	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}
	if _, ok := s.config.ListServices[domainKey]; !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	reqCtx := ctx.Request.Context()

	// 1) Ищем виртуальный аккаунт по логину.
	login := strings.TrimSpace(body.Login)
	virtualAuth, err := s.authRepo.GetVirtual(reqCtx, login)
	if err != nil {
		// Не раскрываем, существует логин или нет.
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.Unauthorized(schemes.ErrAuthInvalidCredentials, "invalid credentials")
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to query virtual auth", err)
	}

	// Если виртуальный логин ещё не подтверждён — не пускаем.
	// (Сейчас при создании виртуала ставим verification=true, но оставим проверку как safety-net.)
	if !virtualAuth.Verification {
		return answer, schemes.Unauthorized(schemes.ErrAuthInvalidCredentials, "invalid credentials")
	}

	// 2) Проверяем пароль.
	if err := utils.CheckPasswordHash(virtualAuth.PasswordHash, body.Password); err != nil {
		return answer, schemes.Unauthorized(schemes.ErrAuthInvalidCredentials, "invalid credentials")
	}

	// 3) Доп. sanity: пользователь должен быть виртуальным.
	u, err := s.userRepo.GetUserByID(reqCtx, virtualAuth.UserID)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load user", err)
	}
	if !u.IsVirtual {
		return answer, schemes.Unauthorized(schemes.ErrAuthInvalidCredentials, "invalid credentials")
	}

	// Виртуальный пользователь должен принадлежать домену (virtual-only-in-one-workspace).
	if workspaces, err := s.workspaceRepo.GetworkspacesByUserInDomain(reqCtx, u.ID, bdName); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to check user workspaces", err)
	} else if len(workspaces) == 0 {
		// Не раскрываем, что логин валиден, но домен другой.
		return answer, schemes.Unauthorized(schemes.ErrAuthInvalidCredentials, "invalid credentials")
	}

	// 4) Генерируем login-ticket (как в telegram/oauth2).
	claims := helpers.LoginTicketClaims{
		DeviceID: body.DeviceID,
		Domain:   domainKey,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  []string{s.config.App.URL},
			Subject:   strconv.FormatInt(u.ID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.config.App.URL,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ticket, err := tok.SignedString([]byte(s.config.JWT.TicketTokenSecret))
	if err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "couldn't create login ticket", err)
	}

	answer.Ticket = ticket
	answer.RedirectURL = ""
	return answer, nil
}

// Me — информация о текущем пользователе по access-токену.
func (s *ServiceToken) Me(ctx *gin.Context) (DTO.MeResponse, error) {
	var answer DTO.MeResponse

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	// 1) Достаём access token: Authorization: Bearer <token> (или cookie access_token).
	authToken := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if authToken != "" && len(authToken) > 7 && strings.EqualFold(authToken[:7], "bearer ") {
		authToken = strings.TrimSpace(authToken[7:])
	}
	if authToken == "" {
		if v, err := ctx.Cookie("access_token"); err == nil {
			authToken = strings.TrimSpace(v)
		}
	}
	if authToken == "" {
		return answer, schemes.Unauthorized(schemes.ErrAuthTokenMissing, "Unable to find token")
	}

	// 2) Валидируем и парсим access token.
	var claims schemes.JwtCustomClaims
	if err := utils.ExtractToken(
		authToken,
		s.config.JWT.AccessTokenSecret,
		&claims,
		"",
		[]string{bdName},
	); err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "BAD token", err)
	}

	answer.UserID = claims.UserID
	answer.WorkspaceID = claims.WorkspaceID
	answer.Role = claims.Role
	answer.Audience = claims.Audience
	if claims.ExpiresAt != nil {
		answer.ExpiresAt = claims.ExpiresAt.Unix()
	}

	return answer, nil
}

// Introspect — проверка access-токена (валиден ли и какие claims внутри).
// IMPORTANT: возвращает 200 всегда, с Active=true/false.
func (s *ServiceToken) Introspect(ctx *gin.Context, body *DTO.IntrospectValid) (DTO.IntrospectResponse, error) {
	var answer DTO.IntrospectResponse

	bdName := ctx.GetString(middlewares.BDName)
	if bdName == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalBDNameMissing, "bd_name not found in context")
	}

	tokenStr := strings.TrimSpace(body.Token)
	if tokenStr == "" {
		answer.Active = false
		return answer, nil
	}

	var claims schemes.JwtCustomClaims
	if err := utils.ExtractToken(
		tokenStr,
		s.config.JWT.AccessTokenSecret,
		&claims,
		"",
		[]string{bdName},
	); err != nil {
		answer.Active = false
		return answer, nil
	}

	answer.Active = true
	answer.UserID = claims.UserID
	answer.WorkspaceID = claims.WorkspaceID
	answer.Role = claims.Role
	answer.Audience = claims.Audience
	if claims.ExpiresAt != nil {
		answer.ExpiresAt = claims.ExpiresAt.Unix()
	}

	return answer, nil
}

// Revoke — отзыв refresh token (конкретной сессии).
func (s *ServiceToken) Revoke(ctx *gin.Context, body *DTO.RevokeValid) (schemes.EmptyResponses, error) {
	var answer schemes.EmptyResponses

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// 1) Валидируем refresh-токен (ИМЕННО RefreshTokenSecret!)
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

	// 2) Ищем сессию по hash(refresh)
	refreshHash := utils.HashToken(body.RefreshToken)
	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// 3) Опционально проверяем device_id
	if body.DeviceID != nil {
		deviceID := strings.TrimSpace(*body.DeviceID)
		if deviceID != "" && (session.DeviceID == "" || session.DeviceID != deviceID) {
			return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for revoke")
		}
	}

	// 4) Идемпотентность: если уже протухло/неактивно — просто ок
	if !session.IsActive || session.ExpiresAt.Before(now) {
		return answer, nil
	}

	// 5) Выключаем конкретную сессию
	if err := s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID}); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to revoke session", err)
	}

	return answer, nil
}

// RefreshTokenDomainFlow — безопасный обмен refresh-токена на новую пару access/refresh
// в рамках конкретного домена (chanki/poster/...).
//
// Поток:
//
//  1. Достаём домен из middleware.
//  2. Парсим и валидируем сам refresh (подпись, exp, iss, aud).
//  3. Находим сессию по hash(refresh_token) в user_sessions.
//     - если нет записи → токен не наш, просто 401/97.
//  4. Проверяем, что:
//     - сессия ещё активна,
//     - не протухла,
//     - принадлежит этому домену,
//     - принадлежит этому device_id (body.DeviceID).
//     Если что-то не так → считаем это реюзом/аномалией, блочим ВСЕ сессии этого девайса.
//  5. Валидируем workspace_id — пользователь обязан быть участником проекта, достаём роль.
//  6. Загружаем все активные сессии юзера в домене,
//     считаем девайсы и применяем политику:
//     - все старые сессии этого же device_id деактивируем,
//     - если девайсов > MaxDevicesUser → находим самый старый девайс (≠ текущий) и
//     деактивируем все его сессии.
//  7. Создаём НОВУЮ сессию для этого девайса (rotation):
//     - инсертим user_sessions (IsActive=true, ExpiresAt = now + refreshTTL).
//     - генерим новый opaque refresh JWT (без session_id),
//     считаем hash и забиваем его в user_sessions через UpdateSessionToken.
//  8. Генерим новый access-токен с payload D = "userID|workspaceID|role".
//  9. Только если всё ОК (все апдейты/инсерты прошли) — возвращаем пару токенов.
//
// Важно:
//   - старый refresh токен после успешного вызова становится бесполезным,
//     так как его hash уже сидит в неактивной/старой сессии;
//   - повторное использование старого refresh'а → мы найдём эту сессию,
//     увидим, что IsActive = false или ExpiresAt < now, и заблочим весь девайс.
func (s *ServiceToken) RefreshTokenDomainFlow(
	ctx *gin.Context,
	body *DTO.RefreshTokenDomainValid,
) (DTO.AnswerToken, error) {
	var answer DTO.AnswerToken

	// ------------ 0) Достаём домен и конфиг ------------

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

	// ------------ 1) Валидируем и парсим refresh-токен ------------

	var refreshClaims schemes.JwtCustomRefreshClaims

	if err := utils.ExtractToken(
		body.RefreshToken,
		s.config.JWT.RefreshTokenSecret,
		&refreshClaims,
		s.config.App.URL,           // expectedIssuer
		[]string{s.config.App.URL}, // expectedAudience
	); err != nil {
		// Тут не можем сказать ничего про конкретный девайс — токен вообще не валиден.
		return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthInvalidRefreshToken, "invalid refresh token", err)
	}

	// Доп. safety: exp уже проверился в Valid(), но оставим комментарий.
	// refreshClaims.ExpiresAt != nil && refreshClaims.ExpiresAt.Time.Before(now) → уже учтено.

	// ------------ 2) Ищем сессию по hash(refresh_token) ------------

	refreshHash := utils.HashToken(body.RefreshToken)

	// Предполагаем, что у UserRepository есть метод:
	//   GetSessionByHash(ctx, hash string) (*models.MdUserSession, error)
	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Токен валидный криптографически, но мы его не знаем.
			// Это может быть и "рандом", и украденный уже-ротированный токен.
			// Пока просто отвечаем "invalid", без глобальной рубки по девайсу.
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}

		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// ------------ 3) Проверяем, что сессия живая и относится к этому домену/девайсу ------------

	// 3.1) Домен сессии должен совпадать с доменом запроса.
	if session.Domain != bdName {
		// Это уже похоже на странный кросс-доменный токен → деактивируем эту сессию.
		_ = s.userRepo.DeactivateSessionsByIDs(reqCtx, []int64{session.ID})

		return answer, schemes.Forbidden(schemes.ErrAuthSessionDomainMismatch, "session domain mismatch for refresh token")
	}

	// 3.2) DeviceID должен совпадать с тем, что прислал клиент сейчас.
	//     Это как раз часть защиты "через state".
	if session.DeviceID == "" || session.DeviceID != body.DeviceID {
		// Неправильный device_id → кто-то пытался использовать refresh с другого девайса.
		// Блокируем этот device_id (из записи сессии).
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for refresh token")
	}

	// 3.3) Проверяем активность и срок жизни сессии.
	if !session.IsActive || session.ExpiresAt.Before(now) {
		// Это классический reuse/expired-кейс:
		// - Сессия уже не должна была использоваться,
		// - но токен всё ещё кто-то прислал.
		// Реакция: блочим все сессии этого девайса.
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthRefreshExpiredOrReused, "refresh token expired or reused")
	}

	userID := session.UserID
	deviceID := session.DeviceID

	// ------------ 4) Валидируем workspace_id и роль пользователя ------------

	workspaceID, err := strconv.ParseInt(body.WorkspaceID, 10, 64)
	if err != nil || workspaceID <= 0 {
		// 400: workspace_id невалидный.
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrWorkspaceInvalidID, "invalid workspace_id", err)
	}

	// Проверяем, что пользователь действительно состоит в проекте, и получаем роль.
	workspaceDomain, role, isMember, err := s.workspaceRepo.GetWorkspaceDomainAndUserRole(reqCtx, workspaceID, userID)
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
		return answer, schemes.Forbidden(schemes.ErrWorkspaceInsufficientPermissions, "user has no access to this workspace")
	}

	// ------------ 5) Политика по девайсам (максимум N девайсов) ------------

	// Забираем все активные сессии юзера в рамках домена.
	activeSessions, err := s.userRepo.GetActiveSessionsByUserAndDomain(
		reqCtx,
		userID,
		bdName,
	)
	if err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to load active sessions", err)
	}

	// Собираем ID сессий, которые надо деактивировать:
	//  - все сессии этого же девайса (rotation),
	//  - возможно, все сессии самого старого другого девайса, если девайсов > MaxDevicesUser.
	var toDeactivate []int64

	// Сначала собираем по девайсам:
	type devInfo struct {
		Oldest time.Time
	}

	deviceMap := make(map[string]devInfo)

	for _, sss := range activeSessions {
		// 5.1) Все сессии текущего device_id будут ротироваться → деактивируем.
		if sss.DeviceID == deviceID {
			toDeactivate = append(toDeactivate, sss.ID)
			continue
		}

		// 5.2) Остальные девайсы учитываем в deviceMap.
		if info, ok := deviceMap[sss.DeviceID]; ok {
			if sss.CreatedAt.Before(info.Oldest) {
				info.Oldest = sss.CreatedAt
				deviceMap[sss.DeviceID] = info
			}
		} else {
			deviceMap[sss.DeviceID] = devInfo{Oldest: sss.CreatedAt}
		}
	}

	// Текущее устройство тоже будет присутствовать (новая сессия), так что ожидаемое
	// количество девайсов = len(deviceMap) + 1.
	if svcCfg.MaxDevicesUser > 0 && len(deviceMap)+1 > svcCfg.MaxDevicesUser {
		// Надо выкинуть самый старый девайс (НЕ текущий).
		var oldestDevID string
		var oldestTime time.Time

		for devID, info := range deviceMap {
			if oldestDevID == "" || info.Oldest.Before(oldestTime) {
				oldestDevID = devID
				oldestTime = info.Oldest
			}
		}

		if oldestDevID != "" {
			for _, sss := range activeSessions {
				if sss.DeviceID == oldestDevID {
					toDeactivate = append(toDeactivate, sss.ID)
				}
			}
		}
	}

	// ------------ 8) Деактивируем все устаревшие сессии одним запросом ------------

	if len(toDeactivate) > 0 {
		if err := s.userRepo.DeactivateSessionsByIDs(reqCtx, toDeactivate); err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to deactivate old sessions", err)
		}
	}

	// ------------ 6) Создаём новую сессию для этого девайса (rotation) ------------

	refreshTTL := time.Duration(s.config.JWT.RefreshTokenMinute) * time.Minute
	expiresAt := now.Add(refreshTTL)
	// Мы больше НЕ кладём session_id внутрь JWT — он opaque.
	// Внутри только exp/iss/aud (у тебя это уже реализовано в JwtCustomRefreshClaims).
	newRefreshToken, err := utils.CreateRefreshToken(
		"", // idSession — пустой, чтобы ничего не светить наружу
		s.config.JWT.RefreshTokenSecret,
		s.config.App.URL,           // expectedIssuer
		[]string{s.config.App.URL}, // expectedAudience
		s.config.JWT.RefreshTokenMinute,
	)
	if err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "couldn't create new refresh token", err)
	}

	newRefreshHash := utils.HashToken(newRefreshToken)

	newSession := &models.MdUserSession{
		UserID:     userID,
		Domain:     bdName,
		DeviceID:   deviceID,
		DeviceType: common.GetDeviceType(ctx),
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
		IP:         common.GetClientIP(ctx),
		IsActive:   true,
		HashToken:  newRefreshHash,
	}

	// ВАЖНО: CreateUserSession должен делать RETURNING id.
	if err := s.userRepo.CreateUserSession(reqCtx, newSession); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "couldn't create a new session", err)
	}

	// ------------ 9) Генерируем новый access-токен (user_id | workspace_id | role) ------------

	newAccessToken, err := utils.CreateAccessToken(
		userID,      // int64
		workspaceID, // int64
		role,        // string
		[]string{bdName},
		s.config.JWT.AccessTokenSecret,
		s.config.JWT.AccessTokenMinute,
	)

	if err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "couldn't create access token", err)
	}

	// ------------ 10) Формируем ответ ------------

	answer.RefreshToken = newRefreshToken
	answer.AccessToken = newAccessToken

	return answer, nil
}

func (s *ServiceToken) LogoutDeviceAllDomains(
	ctx *gin.Context,
	body *DTO.LogoutDeviceValid,
) (schemes.EmptyResponses, error) {

	var answer schemes.EmptyResponses

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// 1) Валидируем refresh-токен (ИМЕННО RefreshTokenSecret!)
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

	// 2) Ищем сессию по hash(refresh)
	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// 3) Проверяем device_id (чтобы нельзя было чужим refresh’ем вылогинить другой девайс)
	if session.DeviceID == "" || session.DeviceID != body.DeviceID {
		// как и в refresh-флоу — жёстко баним сессии того девайса, от которого пришёл токен
		_ = s.userRepo.DeactivateSessionsByDevice(
			reqCtx,
			session.UserID,
			session.Domain,
			session.DeviceID,
		)

		return answer, schemes.Unauthorized(schemes.ErrAuthDeviceMismatch, "device_id mismatch for logout")
	}

	// 4) Если сессия уже неактивна или протухла — считаем, что всё равно можно
	//    безопасно "логаутнуть" девайс (операция идемпотентная).
	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessionsByUserAndDevice(reqCtx, session.UserID, session.DeviceID)
		return answer, nil
	}

	// 5) Основная операция: выключаем ВСЕ сессии этого юзера и этого девайса по всем доменам.
	if err := s.userRepo.DeactivateSessionsByUserAndDevice(reqCtx, session.UserID, session.DeviceID); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to deactivate sessions for device", err)
	}

	return answer, nil
}

func (s *ServiceToken) AllLogout(
	ctx *gin.Context,
	body *DTO.LogoutAllValid,
) (schemes.EmptyResponses, error) {

	var answer schemes.EmptyResponses

	reqCtx := ctx.Request.Context()
	now := time.Now()

	// 1) Валидируем refresh-токен
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

	// 2) Ищем сессию по hash(refresh)
	refreshHash := utils.HashToken(body.RefreshToken)

	session, err := s.userRepo.GetSessionByHash(reqCtx, refreshHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapKind(schemes.KindUnauthorized, schemes.ErrAuthRefreshNotFound, "refresh token not found", err)
		}
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to lookup session by refresh hash", err)
	}

	// 3) Даже если сессия уже умерла — операция "выход отовсюду" идемпотентна.
	if !session.IsActive || session.ExpiresAt.Before(now) {
		_ = s.userRepo.DeactivateSessions(reqCtx, session.UserID)
		return answer, nil
	}

	// 4) Основная операция — дропаем все сессии юзера по всем доменам и девайсам.
	if err := s.userRepo.DeactivateSessions(reqCtx, session.UserID); err != nil {
		return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "failed to deactivate all sessions for user", err)
	}

	return answer, nil
}
