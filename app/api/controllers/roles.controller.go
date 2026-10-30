package controllers

import (
	"anubis/app/DTO"
	"anubis/app/api/interfaces"
	"anubis/app/core/common"
	"anubis/app/core/schemes"

	"github.com/gin-gonic/gin"
)

// NOTE: Пустые ссылки ниже предназначены только для помощи swag (генератора swagger)
// используется в аннотациях без влияния на поведение во время выполнения.
var (
	_ = DTO.RolesValid{}
	_ = DTO.RolesResponse{}
	_ = schemes.Responses{}
	_ = schemes.HTTPError{}
	_ = schemes.ValidateErrorResponse{}
)

type ControllerRoles struct {
	rolesUC interfaces.RolesUseCase
}

func NewControllerRoles(rolesUC interfaces.RolesUseCase) *ControllerRoles {
	return &ControllerRoles{rolesUC: rolesUC}
}

// HandlerPOSTRoles returns domain roles dictionary (owner/admin only).
// @Summary Справочник ролей домена
// @Description Возвращает роли домена (code/description/variants). Доступно только owner/admin в указанном workspace_id.
// @Tags Workspace
// @Accept json
// @Produce json
// @Param Domain header string false "Ключ домена (dev). В prod вычисляется из Host."
// @Param payload body DTO.RolesValid true "refresh_token + workspace_id + device_id"
// @Success 200 {object} schemes.Responses{data=DTO.RolesResponse}
// @Failure 400 {object} schemes.ValidateErrorResponse
// @Failure 401 {object} schemes.HTTPError
// @Router /roles [post]
func (c *ControllerRoles) HandlerPOSTRoles(ctx *gin.Context) {
	common.JsonHandler(ctx, c.rolesUC.GetRoles)
}
