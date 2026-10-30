package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateInviteToken генерирует одноразовый invite token для ссылки-приглашения.
// Формат: base64url без '=' (коротко, безопасно для URL).
func GenerateInviteToken() (string, error) {
	b := make([]byte, 32) // 256-bit entropy
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate invite token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
