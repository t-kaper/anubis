package usecase

import (
	"anubis/app/DAL/models"
	"anubis/app/DAL/repo"
	"anubis/app/DTO"
	"anubis/app/api/helpers"
	"anubis/app/core"
	"anubis/app/core/middlewares"
	"anubis/app/core/schemes"
	"anubis/internal/utils"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"time"
)

// ServiceTelegramAuth — бизнес-логика Telegram Login Widget.
type ServiceTelegramAuth struct {
	userRepo      repo.UserRepository
	workspaceRepo repo.WorkspaceRepository
	authRepo      repo.AuthRepository
	config        core.ServiceConfig
}

// Политика ошибок (для логов):
// - Ожидаемые/клиентские ошибки: schemes.New / schemes.Wrap (origin есть, stack нет).
// - Внутренние ошибки: schemes.WrapInternal (origin + stack; stack логируется только для 5xx).

// NewServiceTelegramAuth — конструктор.
func NewServiceTelegramAuth(
	userRepo repo.UserRepository,
	workspaceRepo repo.WorkspaceRepository,
	authRepo repo.AuthRepository,
	config core.ServiceConfig,
) *ServiceTelegramAuth {
	return &ServiceTelegramAuth{
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
		authRepo:      authRepo,
		config:        config,
	}
}

// LoginWidget обрабатывает данные, полученные от Telegram Login Widget:
//
// 1. Достаём domain из middleware.
// 2. Берём bot_token из config.ListServices[domain].Provider["telegram"].ClientSecret.
// 3. Проверяем подпись hash по официальному алгоритму (HMAC-SHA256).
// 4. Проверяем, что auth_date не слишком старый (TTL, например, 5 минут).
// 5. Ищем пользователя:
//   - сначала по oauth_accounts (provider="telegram", provider_user_id = telegram ID),
//   - если не нашли — создаём MdUser + привязку OAuth.
//
// 6. Загружаем проекты пользователя, при отсутствии — создаём дефолтный.
// 7. Генерируем login-ticket (JWT с user_id, device_id, domain, TTL 10 минут).
// 8. Возвращаем AnswerTicketToken {ticket, list_workspaces, redirect_url?}.
func (s *ServiceTelegramAuth) LoginWidget(
	ctx *gin.Context,
	body *DTO.TelegramLoginWidgetDTO,
) (DTO.AnswerTicketToken, error) {

	var answer DTO.AnswerTicketToken

	// ---------- 0) Достаём domain и конфиг сервиса ----------

	domainFromCtx := ctx.GetString(middlewares.Domain)
	if domainFromCtx == "" {
		// Внутренняя ошибка: middleware не положил domain в context.
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "domain not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainFromCtx]
	if !ok {
		// Внутренняя ошибка: домен существует, но нет конфига сервиса.
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "service config not found for domain")
	}

	// По соглашению: bot_token лежит в Provider["telegram"].ClientSecret.
	providerCfg, ok := svcCfg.Provider["telegram"]
	if !ok {
		// Внутренняя ошибка: в конфиге домена не описан telegram провайдер.
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalTelegramProviderConfigMissing, "telegram provider config not found for domain")
	}
	if providerCfg.ClientSecret == "" {
		// Внутренняя ошибка: telegram bot token не задан.
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalTelegramBotTokenMissing, "telegram bot token (client_secret) is empty")
	}

	reqCtx := ctx.Request.Context()

	// ---------- 1) Проверка подписи hash от Telegram ----------

	if err := helpers.VerifyTelegramLoginHash(body, providerCfg.ClientSecret); err != nil {
		// 4xx: пользователь прислал некорректную подпись/данные Telegram.
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrTelegramInvalidHash, "invalid telegram login hash", err)
	}

	// ---------- 2) Проверка свежести auth_date ----------

	authTime := time.Unix(body.AuthDate, 0)
	// Защита от timestamp из будущего (clock skew / злоупотребление).
	if authTime.After(time.Now().Add(3 * time.Minute)) {
		// 4xx: защита от timestamp из будущего.
		return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrTelegramAuthDateFuture, "telegram auth_date is in the future")
	}
	if time.Since(authTime) > 5*time.Minute {
		// 4xx: протухшая попытка авторизации.
		return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrTelegramAuthDateTooOld, "telegram auth_date is too old")
	}

	// ---------- 3) Ищем / создаём пользователя ----------

	var (
		userID       int64
		telegramID   = strconv.FormatInt(body.ID, 10)
		provider     = "telegram"
		profileName  *string
		primaryEmail *string // Telegram Login Widget email по умолчанию не даёт
	)

	if body.FirstName != "" || body.LastName != "" {
		full := strings.TrimSpace(body.FirstName + " " + body.LastName)
		if full != "" {
			profileName = &full
		}
	}

	// 3.1) Пытаемся найти по oauth_accounts (provider="telegram", provider_user_id=telegramID).
	oauthAcc, err := s.authRepo.GetOAuth(reqCtx, provider, telegramID)
	if err == nil {
		userID = oauthAcc.UserID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// 5xx: БД/репозиторий.
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to query oauth_accounts (telegram)", err)
	}

	// 3.2) Если не нашли — создаём нового пользователя.
	if userID == 0 {
		// Ник — лучше всего username, если есть, иначе имя.
		nickname := body.Username
		if nickname == "" && profileName != nil && *profileName != "" {
			nickname = *profileName
		}
		if nickname == "" {
			nickname = utils.GenerateUserNickname()
		}

		user, err := s.userRepo.CreateUser(reqCtx, nickname)
		if err != nil {
			// 5xx: не смогли создать пользователя.
			return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to create user for telegram login", err)
		}
		userID = user.ID
	}

	// 3.3) Привязка / обновление Telegram OAuth-аккаунта.
	if oauthAcc == nil {
		oauthAcc = &models.MdOAuthAccount{
			UserID:         userID,
			Provider:       provider,
			ProviderUserID: telegramID,
			Name:           profileName,
			Email:          primaryEmail,
		}
		if err := s.authRepo.CreateOAuth(reqCtx, oauthAcc); err != nil {
			// 5xx: не смогли создать привязку oauth аккаунта.
			return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to create telegram oauth_account", err)
		}
	}

	// Обновляем профиль (username/имя/email) через upsert.
	oauthAcc.Name = profileName
	oauthAcc.Email = primaryEmail

	if err := s.authRepo.UpsertOAuthProfile(reqCtx, oauthAcc); err != nil {
		// 5xx: не смогли обновить профиль oauth.
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "failed to upsert telegram oauth profile", err)
	}

	// ---------- 5) Генерируем login-ticket ----------

	claims := helpers.LoginTicketClaims{
		DeviceID:     body.DeviceID,
		Domain:       domainFromCtx,
		AuthMethod:   "telegram",
		AuthProvider: "",
		InviteToken: func() string {
			if body.InviteToken == nil {
				return ""
			}
			return strings.TrimSpace(*body.InviteToken)
		}(),
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  []string{s.config.App.URL},
			Subject:   strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.config.App.URL,
		},
	}

	loginTicketToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	loginTicket, err := loginTicketToken.SignedString([]byte(s.config.JWT.TicketTokenSecret))
	if err != nil {
		// 5xx: внутренняя ошибка подписи JWT (должна быть редкой).
		return answer, schemes.WrapInternal(schemes.ErrAuthTicketSignFailed, "couldn't create login ticket for telegram", err)
	}

	answer.Ticket = loginTicket

	// Для Telegram Login Widget обычно нет redirect, фронт просто читает JSON.
	answer.RedirectURL = ""

	return answer, nil
}
