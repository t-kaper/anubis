package controllers

import (
	"anubis/app/DTO"
	"anubis/app/api/interfaces"
	"anubis/app/core/common"
	"anubis/app/core/schemes"
	"github.com/gin-gonic/gin"
)

// NOTE: The blank references below are only to help swag (swagger generator) resolve types
// used in annotations without impacting runtime behavior.
var (
	_ = DTO.TelegramLoginWidgetDTO{}
	_ = DTO.AnswerTicketToken{}
	_ = schemes.Responses{}
	_ = schemes.HTTPError{}
	_ = schemes.ValidateErrorResponse{}
)

// ControllerTelegramAuth отвечает за HTTP-уровень авторизации через Telegram.
// Здесь только принимаем JSON от фронта и пробрасываем в usecase.
type ControllerTelegramAuth struct {
	tgUC interfaces.TelegramAuthUseCase
}

func NewControllerTelegramAuth(tgUC interfaces.TelegramAuthUseCase) *ControllerTelegramAuth {
	return &ControllerTelegramAuth{tgUC: tgUC}
}

// HandlerPOSTTelegramLoginWidget validates Telegram Login Widget payload and issues login ticket.
// @Summary Telegram Login Widget
// @Description Проверяет подпись Telegram Login Widget и возвращает одноразовый login-ticket.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.TelegramLoginWidgetDTO true "Payload Telegram + device_id"
// @Success 200 {object} schemes.Responses{data=DTO.AnswerTicketToken}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /auth/telegram/login_widget [post]
func (c *ControllerTelegramAuth) HandlerPOSTTelegramLoginWidget(ctx *gin.Context) {
	common.JsonHandler(ctx, c.tgUC.LoginWidget)
}
