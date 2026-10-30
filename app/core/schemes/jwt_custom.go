package schemes

import (
	"github.com/golang-jwt/jwt/v5"
)

type JwtCustomClaims struct {
	UserID      int64  `json:"user_id"`
	WorkspaceID int64  `json:"workspace_id"`
	Role        string `json:"role"`
	jwt.RegisteredClaims
}

type JwtCustomRefreshClaims struct {
	//ID string `json:"id"`
	jwt.RegisteredClaims
}
