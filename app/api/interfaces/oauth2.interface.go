package interfaces

import (
	"anubis/app/DTO"
	"github.com/gin-gonic/gin"
)

type Oauth2UseCase interface {
	GetOAuth2RedirectURL(ctx *gin.Context, uri *DTO.UriProviderValid, query *DTO.OAuth2Query) (string, error)
	CreateTicketUserFlow(ctx *gin.Context, uri *DTO.UriProviderValid, query *DTO.OAuth2CallbackQuery) (DTO.AnswerTicketToken, error)
}
