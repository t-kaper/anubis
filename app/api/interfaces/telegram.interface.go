package interfaces

import (
	"anubis/app/DTO"
	"github.com/gin-gonic/gin"
)

// TelegramAuthUseCase описывает бизнес-логику авторизации через Telegram Login Widget.
type TelegramAuthUseCase interface {
	// LoginWidget обрабатывает данные от Telegram Login Widget:
	// - проверяет подпись hash
	// - находит/создаёт пользователя
	// - находит/создаёт проекты
	// - возвращает login-ticket + list_workspaces (+ redirect_url при необходимости)
	LoginWidget(ctx *gin.Context, body *DTO.TelegramLoginWidgetDTO) (DTO.AnswerTicketToken, error)
}
