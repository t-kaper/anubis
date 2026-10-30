package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GenerateOAuthState генерит "безопасный" state для OAuth2.
// Внутри кодируем device_id, domain, timestamp и (опционально) invite_token, а сверху лепим HMAC-подпись.
// Формат (до base64):
//
//	payload = deviceID + "|" + domain + "|" + unix_ts + "|" + inviteToken
//	state   = base64url( payload + "." + hex(HMAC_SHA256(payload, secret)) )
//
// Такой state:
//   - НЕЛЬЗЯ подделать без секрета
//   - можно распарсить на callback и понять device_id + domain + время
func GenerateOAuthState(deviceID, domain, inviteToken, secret string) (string, error) {
	if deviceID == "" {
		return "", fmt.Errorf("empty deviceID")
	}
	if domain == "" {
		return "", fmt.Errorf("empty domain")
	}
	if secret == "" {
		return "", fmt.Errorf("empty secret for state HMAC")
	}

	ts := time.Now().Unix()
	// inviteToken может быть пустым.
	payload := fmt.Sprintf("%s|%s|%d|%s", deviceID, domain, ts, inviteToken)

	// Считаем HMAC-SHA256(payload, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	sigHex := hex.EncodeToString(sig)

	raw := payload + "." + sigHex

	// Кодируем в URL-safe base64, без '=' в конце, чтобы можно было безопасно пихать в URL
	state := base64.RawURLEncoding.EncodeToString([]byte(raw))

	return state, nil
}

// ParseOAuthState делает обратную операцию:
//   - base64url decode
//   - делим на payload и подпись
//   - заново считаем HMAC и сравниваем
//   - парсим device_id, domain, timestamp, invite_token
//
// maxAge — максимальный возраст state (например, 5 минут).
func ParseOAuthState(state, secret string, maxAge time.Duration) (deviceID, domain, inviteToken string, err error) {
	if state == "" {
		return "", "", "", fmt.Errorf("empty state")
	}
	rawBytes, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid state encoding: %w", err)
	}
	raw := string(rawBytes)

	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid state format")
	}
	payload := parts[0]
	sigHex := parts[1]

	// Проверяем подпись
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSig := mac.Sum(nil)
	expectedSigHex := hex.EncodeToString(expectedSig)

	// Важно: использовать hmac.Equal для защиты от тайминговых атак
	if !hmac.Equal([]byte(sigHex), []byte(expectedSigHex)) {
		return "", "", "", fmt.Errorf("invalid state signature")
	}

	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 4 {
		return "", "", "", fmt.Errorf("invalid payload format in state")
	}

	deviceID = payloadParts[0]
	domain = payloadParts[1]
	tsStr := payloadParts[2]
	inviteToken = payloadParts[3]

	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid timestamp in state: %w", err)
	}

	// Проверяем возраст state
	if maxAge > 0 {
		createdAt := time.Unix(tsUnix, 0)
		if time.Since(createdAt) > maxAge {
			return "", "", "", fmt.Errorf("state is expired")
		}
	}

	return deviceID, domain, inviteToken, nil
}
