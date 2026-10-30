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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	yandex "golang.org/x/oauth2/yandex" // предположим, что у тебя есть такой пакет
)

// ServiceOAuth2Auth отвечает за бизнес-логику OAuth2:
// - выдача redirect URL на провайдера (GetOAuth2RedirectURL)
// - обработка callback'а, привязка аккаунта, создание пользователя и т.д. (ValidCodeUserFlow и прочее).
type ServiceOAuth2Auth struct {
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

// NewServiceOAuth2Auth — конструктор сервиса OAuth2.
// СЮДА мы передаём интерфейсы репозиториев, а не конкретные Mongo/Pg реализации.
// Это позволит позже легко заменить Mongo-репы на Pg-репы, не меняя код бизнес-логики.
func NewServiceOAuth2Auth(
	userRepo repo.UserRepository,
	workspaceRepo repo.WorkspaceRepository,
	authRepo repo.AuthRepository,
	config core.ServiceConfig,
) *ServiceOAuth2Auth {
	return &ServiceOAuth2Auth{
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
		authRepo:      authRepo,
		config:        config,
	}
}

// GetOAuth2RedirectURL формирует URL, на который нужно отправить пользователя,
// чтобы он прошёл авторизацию у внешнего провайдера (google/yandex/...).
//
// Входные данные:
//   - ctx  — gin-контекст, из него мы берём domain (какой сервис: chanki/poster/...)
//   - uri  — { Provider: "google" / "yandex" } из пути /oauth2/login/:provider
//   - query — дополнительные параметры из query (например, device_id, redirect и т.п.)
//
// Возвращает:
//   - redirectURL — ссылку на страничку авторизации провайдера
//   - error       — если что-то пошло не так (неизвестный домен/провайдер и т.д.)
func (s *ServiceOAuth2Auth) GetOAuth2RedirectURL(
	ctx *gin.Context,
	uri *DTO.UriProviderValid,
	query *DTO.OAuth2Query,
) (string, error) {
	// 1. Достаём домен сервиса из контекста.
	domain := ctx.GetString(middlewares.Domain)
	if domain == "" {
		return "", fmt.Errorf("domain not found in context")
	}

	// 2. Находим конфиг для этого сервиса.
	svcCfg, ok := s.config.ListServices[domain]
	if !ok {
		return "", fmt.Errorf("service config not found for domain: %s", domain)
	}

	// 3. Конфиг конкретного OAuth-провайдера.
	providerCfg, ok := svcCfg.Provider[uri.Provider]
	if !ok {
		return "", fmt.Errorf("unsupported provider: %s", uri.Provider)
	}

	// 4. Собираем oauth2.Config.
	var conf *oauth2.Config

	switch uri.Provider {
	case "google":
		conf = &oauth2.Config{
			ClientID:     providerCfg.ClientID,
			ClientSecret: providerCfg.ClientSecret,
			RedirectURL:  providerCfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	case "yandex":
		conf = &oauth2.Config{
			ClientID:     providerCfg.ClientID,
			ClientSecret: providerCfg.ClientSecret,
			RedirectURL:  providerCfg.RedirectURL,
			Scopes: []string{
				"login:default_phone",
				"login:email",
				"login:info",
			},
			Endpoint: yandex.Endpoint,
		}
	default:
		return "", fmt.Errorf("unsupported provider: %s", uri.Provider)
	}

	// 5. Генерируем безопасный state.
	//
	//    В state кодируем:
	//    - device_id (из query.DeviceID)
	//    - domain (из middleware)
	//    - timestamp
	//    и подписываем всё HMAC'ом.
	//
	//    На callback мы сможем достать device_id и domain из state,
	//    не держа вообще никакого состояния в БД/Redis.
	state, err := utils.GenerateOAuthState(
		query.DeviceID,
		domain,
		func() string {
			if query.InviteToken == nil {
				return ""
			}
			return strings.TrimSpace(*query.InviteToken)
		}(),
		svcCfg.StateSecret, // или отдельный secret
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate oauth state: %w", err)
	}

	// 6. Формируем URL на провайдера.
	redirectURL := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)

	return redirectURL, nil
}

// CreateTicketUserFlow — основной OAuth2 callback-флоу под Postgres.
//
// Что делаем:
//
//  1. Разбираем и проверяем state (из него достаём device_id и domain).
//  2. Меняем code -> access token у провайдера.
//  3. Тянем userinfo от провайдера.
//  4. Собираем MdOAuthAccount (provider + provider_user_id + name/email).
//  5. Пытаемся найти пользователя:
//     a) СНАЧАЛА по oauth_accounts (provider + provider_user_id).
//     b) Если не нашли и есть email — пробуем найти по auth_email.email.
//  6. Если и так не нашли — создаём нового MdUser и (по желанию) auth_email.
//  7. Грузим проекты пользователя, выбираем "дефолтный" workspace_id
//     (если домен поддерживает только один проект — берём единственный).
//  8. Генерируем ОДИН специальный токен (login_ticket),
//     в который зашиваем user_id, device_id, workspace_id, domain.
//  9. Возвращаем login_ticket и отдельной строкой workspace_id.
//     Access/refresh-токены будут выдаваться ДРУГИМ эндпоинтом по этому ticket.
func (s *ServiceOAuth2Auth) CreateTicketUserFlow(
	ctx *gin.Context,
	uri *DTO.UriProviderValid,
	input *DTO.OAuth2CallbackQuery,
) (DTO.AnswerTicketToken, error) {

	var answer DTO.AnswerTicketToken
	reqCtx := ctx.Request.Context()

	// -------------------- 0) Валидация State и Config --------------------
	domainFromCtx := ctx.GetString(middlewares.Domain)
	provider := uri.Provider

	if domainFromCtx == "" {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalDomainMissing, "Domain not found in context")
	}

	svcCfg, ok := s.config.ListServices[domainFromCtx]
	if !ok {
		return answer, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalServiceConfigMissing, "Service config not found")
	}

	providerCfg, ok := svcCfg.Provider[provider]
	if !ok {
		return answer, schemes.NewKind(schemes.KindBadRequest, schemes.ErrOAuthUnsupportedProvider, "Unsupported provider")
	}

	// Валидация State
	deviceID, domainFromState, inviteToken, err := utils.ParseOAuthState(input.State, svcCfg.StateSecret, 5*time.Minute)
	if err != nil {
		return answer, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrOAuthInvalidState, "Invalid state", err)
	}
	if domainFromCtx != domainFromState {
		return answer, schemes.Forbidden(schemes.ErrOAuthDomainMismatch, "Domain mismatch")
	}

	// IMPORTANT (security): providerCfg.Redirect настроен (не пользовательский ввод), но ошибки случаются.
	//Мы проверяем её, чтобы избежать превращения конфигурационной ошибки в вектор открытого перенаправления / эксфильтрации данных.
	//В продакшене нам также требуется https и доверенный хост-суффикс.
	if providerCfg.Redirect != "" {
		safeRedirect, err := s.validateFrontendRedirectURL(providerCfg.Redirect)
		if err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "Invalid redirect URL in config", err)
		}
		answer.RedirectURL = safeRedirect
	}

	// -------------------- 1) Получаем данные от провайдера --------------------

	userInfo, err := s.fetchProviderUserInfo(reqCtx, provider, providerCfg, input.Code)
	if err != nil {
		return answer, err // Ошибка уже сформирована внутри fetchProviderUserInfo
	}

	// -------------------- 2) Логика поиска/создания пользователя --------------------

	var (
		userID   int64
		oauthAcc *models.MdOAuthAccount
	)

	// ШАГ 2.1: Ищем существующую OAuth привязку
	oauthAcc, err = s.authRepo.GetOAuth(reqCtx, provider, userInfo.ID)
	if err == nil {
		// Нашли!
		userID = oauthAcc.UserID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// Ошибка БД (не "Not Found")
		return answer, schemes.WrapInternal(schemes.ErrInternalDB, "DB error (oauth)", err)
	}
	// Если err == pgx.ErrNoRows, то userID == 0, oauthAcc == nil

	// ШАГ 2.2: Если OAuth нет, ищем по Email (склейка аккаунтов)
	if userID == 0 && userInfo.Email != nil {
		// 2.2.1) Сначала пробуем по auth_email (классическая email-регистрация).
		emailAuth, err := s.authRepo.GetByEmail(reqCtx, *userInfo.Email)
		if err == nil {
			userID = emailAuth.UserID
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return answer, schemes.WrapInternal(schemes.ErrInternalDB, "DB error (email)", err)
		}

		// 2.2.2) Если auth_email нет — пробуем по oauth_accounts.email (склейка между провайдерами).
		if userID == 0 {
			oacc, err := s.authRepo.GetOAuthByEmail(reqCtx, *userInfo.Email)
			if err == nil {
				userID = oacc.UserID
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return answer, schemes.WrapInternal(schemes.ErrInternalDB, "DB error (oauth email)", err)
			}
		}
	}

	// ШАГ 2.3: Если нигде не нашли — создаем нового пользователя
	if userID == 0 {
		nickname := utils.GenerateUserNickname()
		if userInfo.Name != nil && *userInfo.Name != "" {
			nickname = *userInfo.Name
		}

		user, err := s.userRepo.CreateUser(reqCtx, nickname)
		if err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalDB, "Failed to create user", err)
		}
		userID = user.ID

		// ВАЖНО: не создаём auth_email при OAuth-логине, потому что auth_email.password_hash = NOT NULL
		// и это другой тип авторизации. Email для "склейки" сохраняем в oauth_accounts.email.
	}

	// ШАГ 2.4: Создаем OAuth привязку, если её не было
	if oauthAcc == nil {
		newOAuth := &models.MdOAuthAccount{
			UserID:         userID,
			Provider:       provider,
			ProviderUserID: userInfo.ID,
			Name:           userInfo.Name,
			Email:          userInfo.Email,
		}
		if err := s.authRepo.CreateOAuth(reqCtx, newOAuth); err != nil {
			return answer, schemes.WrapInternal(schemes.ErrInternalUnexpected, "Failed to link oauth", err)
		}
		// OAuth связь создана. Дальше она в этом флоу не нужна — достаточно userID.
	} else {
		// Если аккаунт уже был, обновляем имя/email (Upsert)
		oauthAcc.Name = userInfo.Name
		oauthAcc.Email = userInfo.Email
		if err := s.authRepo.UpsertOAuthProfile(reqCtx, oauthAcc); err != nil {
			// Это не критично, можно просто залогировать ошибку
			common.ZapLogger.Error("failed to update oauth profile", zap.Error(err))
		}
	}

	// -------------------- 3) Генерируем Ticket --------------------

	loginTicket, err := s.generateLoginTicket(userID, deviceID, domainFromCtx, "oauth2", provider, inviteToken)
	if err != nil {
		return answer, schemes.WrapKind(schemes.KindUnauthorized, 97, "Token generation failed", err)
	}

	answer.Ticket = loginTicket

	// Добавляем ticket к URL редиректа безопасно (через url.Parse), чтобы:
	// - не ломать существующий query,
	// - корректно URL-энкодить значения,
	// - не получить "double-??" баги на edge-кейсах.
	if answer.RedirectURL != "" {
		u, err := url.Parse(answer.RedirectURL)
		if err != nil {
			// Если редирект некорректный — не редиректим вообще, возвращаем JSON (с ticket).
			common.ZapLogger.Warn("invalid redirect URL, falling back to JSON response", zap.String("redirect", answer.RedirectURL), zap.Error(err))
			answer.RedirectURL = ""
		} else {
			q := u.Query()
			q.Set("ticket", loginTicket)
			u.RawQuery = q.Encode()
			answer.RedirectURL = u.String()
		}
	}

	return answer, nil
}

// Вспомогательная функция для получения данных от провайдеров
func (s *ServiceOAuth2Auth) fetchProviderUserInfo(
	ctx context.Context,
	provider string,
	cfg core.ProviderOauth,
	code string,
) (providerInfo, error) {

	var res providerInfo

	// IMPORTANT (reliability): OAuth провайдеры иногда "висят" или отвечают медленно.
	// Ограничиваем всю операцию (exchange + userinfo) таймаутом, чтобы не держать воркеры/соединения бесконечно.
	flowCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Выбираем Endpoint и Scopes
	var endpoint oauth2.Endpoint
	var scopes []string
	var infoURL string

	switch provider {
	case "google":
		endpoint = google.Endpoint
		scopes = []string{"openid", "email", "profile"}
		infoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	case "yandex":
		endpoint = yandex.Endpoint
		scopes = []string{"login:email", "login:info", "login:default_phone"}
		infoURL = "https://login.yandex.ru/info?format=json"
	default:
		return res, schemes.NewKind(schemes.KindInternal, schemes.ErrInternalUnexpected, "Unknown provider logic")
	}

	conf := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     endpoint,
		Scopes:       scopes,
	}

	token, err := conf.Exchange(flowCtx, code)
	if err != nil {
		return res, schemes.WrapKind(schemes.KindBadRequest, schemes.ErrOAuthCodeExchangeFailed, "Code exchange failed", err)
	}

	client := conf.Client(flowCtx, token)
	resp, err := client.Get(infoURL)
	if err != nil {
		return res, schemes.WrapInternal(schemes.ErrOAuthUserInfoFetchFailed, "UserInfo fetch failed", err)
	}
	defer resp.Body.Close()

	// IMPORTANT (security / robustness):
	// 1) Проверяем HTTP статус ДО JSON decode — провайдеры часто возвращают HTML/текст при ошибке.
	// 2) Ограничиваем размер body, чтобы не аллоцировать гигабайты при неожиданных ответах.
	const maxBody = 1 << 20 // 1 MiB
	body := io.LimitReader(resp.Body, maxBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Логируем небольшой кусок тела для отладки, но не отдаём его наружу.
		snippet, _ := io.ReadAll(io.LimitReader(body, 4096))
		common.ZapLogger.Warn("oauth provider userinfo returned non-2xx",
			zap.String("provider", provider),
			zap.Int("status", resp.StatusCode),
			zap.String("body_snippet", string(snippet)),
		)
		return res, schemes.NewKind(schemes.KindInternal, schemes.ErrOAuthUserInfoNon2xx, "Provider userinfo returned non-2xx")
	}

	// Парсинг ответов (они разные у Google и Yandex, поэтому тут мини-ветвление или структуры)
	if provider == "google" {
		var gInfo struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(body).Decode(&gInfo); err != nil {
			return res, schemes.WrapInternal(schemes.ErrOAuthUserInfoJSONDecodeFailed, "JSON decode failed", err)
		}
		res.ID = gInfo.ID
		if gInfo.Name != "" {
			res.Name = &gInfo.Name
		}
		if gInfo.Email != "" {
			e := strings.ToLower(gInfo.Email)
			res.Email = &e
		}

	} else if provider == "yandex" {
		var yInfo struct {
			ID       string `json:"id"`
			Email    string `json:"default_email"`
			Name     string `json:"name"`
			RealName string `json:"real_name"`
		}
		if err := json.NewDecoder(body).Decode(&yInfo); err != nil {
			return res, schemes.WrapInternal(schemes.ErrOAuthUserInfoJSONDecodeFailed, "JSON decode failed", err)
		}
		res.ID = yInfo.ID
		if yInfo.Email != "" {
			e := strings.ToLower(yInfo.Email)
			res.Email = &e
		}
		if yInfo.Name != "" {
			res.Name = &yInfo.Name
		} else if yInfo.RealName != "" {
			res.Name = &yInfo.RealName
		}
	}

	// Минимальная sanity-check: provider user id обязателен.
	if strings.TrimSpace(res.ID) == "" {
		return res, schemes.NewKind(schemes.KindInternal, schemes.ErrOAuthUserInfoMissingID, "Provider userinfo missing id")
	}

	return res, nil
}

// Вспомогательная для генерации login-ticket (чтобы не загромождать основной метод)
func (s *ServiceOAuth2Auth) generateLoginTicket(
	userID int64,
	deviceID string,
	domain string,
	authMethod string,
	authProvider string,
	inviteToken string,
) (string, error) {
	claims := helpers.LoginTicketClaims{
		DeviceID:     deviceID,
		Domain:       domain,
		AuthMethod:   authMethod,
		AuthProvider: authProvider,
		InviteToken:  inviteToken,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  []string{s.config.App.URL},
			Subject:   strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.config.App.URL,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWT.TicketTokenSecret))
}

type providerInfo struct {
	ID    string
	Name  *string
	Email *string
}

func (s *ServiceOAuth2Auth) validateFrontendRedirectURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("redirect url parse failed: %w", err)
	}
	if !u.IsAbs() {
		return "", fmt.Errorf("redirect url must be absolute")
	}

	mode := strings.ToLower(strings.TrimSpace(s.config.App.Mode))
	if mode != "development" {
		// Production: only https to avoid token leakage over plaintext.
		if strings.ToLower(u.Scheme) != "https" {
			return "", fmt.Errorf("redirect url must use https in production")
		}
		// Production: enforce trusted host suffix allowlist.
		if !isTrustedHostSuffix(u.Hostname(), s.config.App.TrustedHostSuffixes) {
			return "", fmt.Errorf("redirect url host is not trusted")
		}
	}

	// No further normalization needed; keep as string.
	return u.String(), nil
}

func isTrustedHostSuffix(host string, suffixes []string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return false
	}
	for _, s := range suffixes {
		s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
		if s == "" {
			continue
		}
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}
