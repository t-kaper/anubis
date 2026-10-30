package controllers

import (
	"anubis/app/DTO"
	"anubis/app/api/interfaces"
	"anubis/app/core/common"
	"anubis/app/core/schemes"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NOTE: Пустые ссылки ниже предназначены только для помощи в определении типов swag (генератора swagger)
// используется в аннотациях без влияния на поведение во время выполнения.
var (
	_ = DTO.ExchangeTicketValid{}
	_ = DTO.AnswerExchangeTicketToken{}
	_ = DTO.VirtualLoginValid{}
	_ = DTO.AnswerTicketToken{}
	_ = DTO.MeResponse{}
	_ = DTO.IntrospectValid{}
	_ = DTO.IntrospectResponse{}
	_ = DTO.RevokeValid{}
	_ = DTO.RefreshTokenDomainValid{}
	_ = DTO.AnswerToken{}
	_ = DTO.LogoutDeviceValid{}
	_ = DTO.LogoutAllValid{}
	_ = schemes.Responses{}
	_ = schemes.HTTPError{}
	_ = schemes.ValidateErrorResponse{}
	_ = schemes.EmptyResponses{}
)

type ControllerToken struct {
	tokenUC interfaces.TokenUseCase
}

func NewControllerToken(tokenUC interfaces.TokenUseCase) *ControllerToken {
	return &ControllerToken{tokenUC: tokenUC}
}

// HandlerPOSTValidTicketUserFlow exchanges OAuth login ticket for refresh token and workspace list.
// @Summary Обмен login-ticket на refresh
// @Description Обменивает одноразовый login-ticket на refresh_token (и список проектов).
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.ExchangeTicketValid true "ticket + device_id"
// @Success 200 {object} schemes.Responses{data=DTO.AnswerExchangeTicketToken}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/exchange_ticket [post]
func (c *ControllerToken) HandlerPOSTValidTicketUserFlow(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.ExchangeTicketForTokens)
}

// HandlerPOSTVirtualLogin logs in a virtual user with login/password and returns a one-time ticket.
// @Summary Вход виртуального пользователя
// @Description Проверяет login/password виртуального пользователя и возвращает login-ticket (для обмена через /auth/exchange_ticket).
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.VirtualLoginValid true "device_id + login + password"
// @Success 200 {object} schemes.Responses{data=DTO.AnswerTicketToken}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/virtual/login [post]
func (c *ControllerToken) HandlerPOSTVirtualLogin(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.VirtualLogin)
}

// HandlerGETMe returns current user info by access token.
// @Summary Текущий пользователь (me)
// @Description Возвращает информацию из access-токена: user_id, workspace_id, role, expires_at.
// @Tags Auth
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param Authorization header string true "Bearer <access_token>"
// @Success 200 {object} schemes.Responses{data=DTO.MeResponse}
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/me [get]
func (c *ControllerToken) HandlerGETMe(ctx *gin.Context) {
	res, err := c.tokenUC.Me(ctx)
	if err != nil {
		// по текущей архитектуре все ошибки превращаются в HTTPError с 400,
		// но для /me логичнее 401; поэтому возвращаем как есть через общий handler.
		common.HandlerError(ctx, err)
		return
	}
	common.APIResponse(ctx, http.StatusOK, res)
}

// HandlerPOSTIntrospect validates access token and returns claims (active true/false).
// @Summary Introspect access token
// @Description Проверяет access token и возвращает active=true/false и claims.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.IntrospectValid true "token"
// @Success 200 {object} schemes.Responses{data=DTO.IntrospectResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Router /auth/introspect [post]
func (c *ControllerToken) HandlerPOSTIntrospect(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.Introspect)
}

// HandlerPOSTRevoke revokes a refresh token session.
// @Summary Revoke refresh token
// @Description Отзывает конкретный refresh token (деактивирует соответствующую сессию).
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.RevokeValid true "refresh_token (+ optional device_id)"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/revoke [post]
func (c *ControllerToken) HandlerPOSTRevoke(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.Revoke)
}

// HandlerPOSTRefreshTokenDomain refreshes access/refresh tokens for a given workspace within a domain.
// @Summary Обновление токенов
// @Description Ротация refresh_token и выпуск нового access_token для выбранного workspace_id (привязано к device_id).
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.RefreshTokenDomainValid true "refresh_token + workspace_id + device_id"
// @Success 200 {object} schemes.Responses{data=DTO.AnswerToken}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/refresh_token [post]
func (c *ControllerToken) HandlerPOSTRefreshTokenDomain(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.RefreshTokenDomainFlow)
}

// logout конкретного девайса во всех доменах
// @Summary Logout устройства (все домены)
// @Description Деактивирует все сессии этого устройства во всех доменах для пользователя-владельца refresh_token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.LogoutDeviceValid true "refresh_token + device_id"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/logout [post]
func (c *ControllerToken) HandlerPOSTLogoutDeviceAllDomains(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.LogoutDeviceAllDomains)
}

// logout отовсюду (все девайсы + все домены)
// @Summary Logout отовсюду
// @Description Деактивирует все сессии пользователя-владельца refresh_token по всем доменам/устройствам.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.LogoutAllValid true "refresh_token"
// @Success 200 {object} schemes.Responses{data=schemes.EmptyResponses}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/all_logout [post]
func (c *ControllerToken) HandlerPOSTAllLogout(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tokenUC.AllLogout)
}
