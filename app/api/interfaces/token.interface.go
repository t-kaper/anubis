package interfaces

import (
	"anubis/app/DTO"
	"anubis/app/core/schemes"

	"github.com/gin-gonic/gin"
)

type TokenUseCase interface {
	ExchangeTicketForTokens(ctx *gin.Context, body *DTO.ExchangeTicketValid) (DTO.AnswerExchangeTicketToken, error)

	// VirtualLogin — вход виртуального пользователя по login/password.
	// Возвращает login-ticket, который затем обменивается на refresh_token через /auth/exchange_ticket.
	VirtualLogin(ctx *gin.Context, body *DTO.VirtualLoginValid) (DTO.AnswerTicketToken, error)

	// Me — информация о текущем пользователе по access-токену.
	Me(ctx *gin.Context) (DTO.MeResponse, error)

	// Introspect — проверка access-токена.
	Introspect(ctx *gin.Context, body *DTO.IntrospectValid) (DTO.IntrospectResponse, error)

	// Revoke — отзыв refresh token (конкретной сессии).
	Revoke(ctx *gin.Context, body *DTO.RevokeValid) (schemes.EmptyResponses, error)

	RefreshTokenDomainFlow(ctx *gin.Context, input *DTO.RefreshTokenDomainValid) (DTO.AnswerToken, error)

	// logout конкретного девайса (во всех доменах)
	LogoutDeviceAllDomains(ctx *gin.Context, input *DTO.LogoutDeviceValid) (schemes.EmptyResponses, error)

	// logout вообще отовсюду (все девайсы + все домены)
	AllLogout(ctx *gin.Context, input *DTO.LogoutAllValid) (schemes.EmptyResponses, error)
}
