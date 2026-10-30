package interfaces

import (
	"anubis/app/DTO"
	"github.com/gin-gonic/gin"
)

type RolesUseCase interface {
	GetRoles(ctx *gin.Context, body *DTO.RolesValid) (DTO.RolesResponse, error)
}
