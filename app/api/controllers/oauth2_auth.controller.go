package controllers

import (
	"anubis/app/DTO"
	"anubis/app/api/interfaces"
	"anubis/app/core/common"
	"anubis/app/core/schemes"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ControllerOAuth2Auth отвечает за HTTP-слой для OAuth2:
// - генерация redirect URL на провайдера
// - обработка callback'а от провайдера.
type ControllerOAuth2Auth struct {
	oAuth2UC interfaces.Oauth2UseCase
}

// NewControllerOAuth2Auth создаёт новый контроллер и прокидывает в него usecase.
// В usecase уже зашита вся бизнес-логика и работа с БД/провайдерами.
func NewControllerOAuth2Auth(oAuth2UC interfaces.Oauth2UseCase) *ControllerOAuth2Auth {
	return &ControllerOAuth2Auth{oAuth2UC: oAuth2UC}
}

// HandlerGETOAuth2Login
// GET /oauth2/login/:provider?state=...&redirect=...
// @Summary OAuth2: редирект на провайдера
// @Description Редиректит пользователя на страницу логина OAuth2-провайдера. Требуется query-параметр device_id. Возвращает 302.
// @Tags Auth
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param provider path string true "Провайдер (google|yandex)"
// @Param device_id query string true "Идентификатор устройства" minlength(12) maxlength(64)
// @Success 302 {string} string "Редирект на провайдера"
// @Success 200 {object} schemes.Responses{data=object} "Fallback JSON (ok: true)"
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/oauth2/login/{provider} [get]
//
// Логика:
//  1. Валидируем :provider (из URI) и query-параметры.
//  2. Дёргаем usecase, который вернёт redirectURL на провайдера.
//  3. Если redirectURL не пустой — делаем 302 Redirect.
//  4. Если пустой — отдаём JSON {"ok": true} (можешь потом поменять контракт).
func (c *ControllerOAuth2Auth) HandlerGETOAuth2Login(ctx *gin.Context) {
	var uri DTO.UriProviderValid
	if err := ctx.ShouldBindUri(&uri); err != nil {
		// Единообразная обработка ошибок валидации
		common.ValidateErrorResponse(ctx, uri, err)
		return
	}

	var query DTO.OAuth2Query
	if err := ctx.ShouldBindQuery(&query); err != nil {
		common.ValidateErrorResponse(ctx, query, err)
		return
	}

	// Вызываем бизнес-логику.
	redirectURL, err := c.oAuth2UC.GetOAuth2RedirectURL(ctx, &uri, &query)
	if err != nil {
		// Любая бизнес-ошибка/ошибка БД — через общий HandlerError.
		common.HandlerError(ctx, err)
		return
	}

	// Если usecase вернул URL — считаем, что нужно редиректить на провайдера.
	if redirectURL != "" {
		ctx.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Fallback-вариант: ничего не надо редиректить → просто успешный JSON.
	common.APIResponse(ctx, http.StatusOK, gin.H{"ok": true})
}

// HandlerGETOAuth2Callback
// GET /oauth2/callback/:provider?code=...&state=...
// @Summary OAuth2: callback
// @Description Callback от OAuth2-провайдера. Валидирует state, создаёт/линкует пользователя и возвращает (или редиректит) login ticket.
// @Tags Auth
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param provider path string true "Провайдер (google|yandex)"
// @Param code query string true "Код авторизации OAuth2"
// @Param state query string true "OAuth2 state"
// @Success 302 {string} string "Редирект на фронт (ticket или error_code)"
// @Success 200 {object} schemes.Responses{data=DTO.AnswerTicketToken}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/oauth2/callback/{provider} [get]
//
// Логика:
//  1. Валидируем :provider и query.
//  2. Вызываем usecase.ValidCodeUserFlow.
//     Он должен вернуть структуру типа OAuth2CallbackResult с полем RedirectURL.
//  3. Если возникла ошибка:
//     - если RedirectURL пустой → отдаём стандартную JSON-ошибку через HandlerError
//     - если RedirectURL не пустой → редиректим на него с параметром error=...
//  4. Если ошибки нет:
//     - если RedirectURL пустой → отдаём JSON c данными (например, selection_token, workspaces, session_refresh_token)
//     - если RedirectURL есть → редиректим на фронт (классический OAuth usecase).
func (c *ControllerOAuth2Auth) HandlerGETOAuth2Callback(ctx *gin.Context) {
	var uri DTO.UriProviderValid
	if err := ctx.ShouldBindUri(&uri); err != nil {
		common.ValidateErrorResponse(ctx, uri, err)
		return
	}

	var query DTO.OAuth2CallbackQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		common.ValidateErrorResponse(ctx, query, err)
		return
	}

	// Вызываем бизнес-логику: обмен кода на токены/пользователя/проекты.
	res, err := c.oAuth2UC.CreateTicketUserFlow(ctx, &uri, &query)
	if err != nil {
		// Если usecase вернул URL для редиректа — считаем, что фронт ожидает редирект.
		if res.RedirectURL != "" {
			// IMPORTANT (security): Никогда не перенаправляйте с помощью RAW, ЭЭЭ. Ошибка():
			//- он может содержать внутренние детали,
			//- он должен быть закодирован по URL,
			//- Это может нарушать строки запроса.
			//
			//Вместо этого мы проходим стабильный error_code и необязательный request_id для отладки.
			redirectURL := res.RedirectURL
			errorCode := "oauth_failed"
			var er *schemes.ErrorResponse
			if errors.As(err, &er) {
				errorCode = "code_" + strconv.Itoa(er.Code)
			}
			reqID := strings.TrimSpace(ctx.GetHeader("X-Request-ID"))

			u, pErr := url.Parse(redirectURL)
			if pErr == nil && u.IsAbs() {
				q := u.Query()
				q.Set("error_code", errorCode)
				if reqID != "" {
					q.Set("request_id", reqID)
				}
				u.RawQuery = q.Encode()
				ctx.Redirect(http.StatusFound, u.String())
				return
			}

			// If redirect URL is malformed, fall back to JSON error (safer than redirecting).
			common.HandlerError(ctx, err)
			return
		}

		// Иначе отдаём типичную JSON-ошибку.
		common.HandlerError(ctx, err)
		return
	}

	// Если редирект не нужен — отдаём JSON-ответ.
	if res.RedirectURL == "" {
		common.APIResponse(ctx, http.StatusOK, res)
		return
	}

	// Иначе редиректим на фронт (успешный логин).
	ctx.Redirect(http.StatusFound, res.RedirectURL)
}
