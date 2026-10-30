package utils

import (
	"anubis/app/core/schemes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"strconv"
	"time"
)

func CreateAccessToken(userID int64, workspaceID int64, role string, domains []string, secret string, expiry int) (accessToken string, err error) {
	claims := &schemes.JwtCustomClaims{
		UserID:      userID,      // отдельное поле
		WorkspaceID: workspaceID, // отдельное поле
		Role:        role,        // отдельное поле
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10), // ID как строка для стандартного sub
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(expiry))),
			Audience:  domains,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return t, nil
}

func CreateRefreshToken(
	idSession string,
	secret string,
	expectedIssuer string,
	expectedAudience []string,
	expiry int) (refreshToken string, err error) {
	claims := &schemes.JwtCustomRefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  expectedAudience,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(expiry))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    expectedIssuer,
			ID:        idSession,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return t, err
}

func IsAuthorized(requestToken string, secret string) (bool, error) {
	_, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func ExtractToken[T jwt.Claims](
	tokenStr string,
	secret string,
	claims T,
	expectedIssuer string,
	expectedAudience []string,
) error {
	// Если список допустимых методов не задан — по умолчанию принимаем только HS256.

	var opts []jwt.ParserOption

	// Проверяем iss, если ожидается.
	if expectedIssuer != "" {
		opts = append(opts, jwt.WithIssuer(expectedIssuer))
	}

	// Проверяем aud — твоя версия jwt ожидает ОДИН string в WithAudience,
	// поэтому добавляем опцию для каждого audience отдельно.
	if len(expectedAudience) > 0 {
		for _, aud := range expectedAudience {
			opts = append(opts, jwt.WithAudience(aud))
		}
	}

	parser := jwt.NewParser(opts...)

	parsedToken, err := parser.ParseWithClaims(
		tokenStr,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			// Дополнительная защита: не доверяем неожиданному алгоритму.
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		},
	)
	if err != nil {
		return fmt.Errorf("token parsing failed: %w", err)
	}

	if !parsedToken.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}

type RefreshClaims struct {
	jwt.RegisteredClaims
	//ID string `json:"id"`
	//Roles []string `json:"roles"`
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
