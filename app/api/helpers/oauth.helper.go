package helpers

import (
	"anubis/app/DTO"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"sort"
	"strconv"
	"strings"
)

type LoginTicketClaims struct {
	DeviceID     string `json:"did"`
	Domain       string `json:"dom"`
	AuthMethod   string `json:"am"`            // "oauth2" | "telegram" | "virtual" | ...
	AuthProvider string `json:"ap,omitempty"`  // для oauth2: "yandex"/"google"/...
	InviteToken  string `json:"inv,omitempty"` // одноразовый инвайт (если есть)
	jwt.RegisteredClaims
}

// VerifyTelegramLoginHash проверяет подпись данных от Telegram Login Widget.
// Алгоритм:
//  1. Берём все поля (кроме hash) → "key=value".
//  2. Сортируем по key, джойним через "\n".
//  3. secretKey = sha256(bot_token).
//  4. hmac = HMAC-SHA256(secretKey, dataString).
//  5. hex(hmac) должно совпасть с hash.
func VerifyTelegramLoginHash(body *DTO.TelegramLoginWidgetDTO, botToken string) error {
	// Собираем пары key=value (без hash).
	var dataPairs []string

	dataPairs = append(dataPairs, "auth_date="+strconv.FormatInt(body.AuthDate, 10))
	dataPairs = append(dataPairs, "id="+strconv.FormatInt(body.ID, 10))

	if body.FirstName != "" {
		dataPairs = append(dataPairs, "first_name="+body.FirstName)
	}
	if body.LastName != "" {
		dataPairs = append(dataPairs, "last_name="+body.LastName)
	}
	if body.Username != "" {
		dataPairs = append(dataPairs, "username="+body.Username)
	}
	if body.PhotoURL != "" {
		dataPairs = append(dataPairs, "photo_url="+body.PhotoURL)
	}

	// Сортируем по ключу (Telegram этого требует).
	sort.Slice(dataPairs, func(i, j int) bool {
		// "key=value" → сравниваем по key
		ki := strings.SplitN(dataPairs[i], "=", 2)[0]
		kj := strings.SplitN(dataPairs[j], "=", 2)[0]
		return ki < kj
	})

	dataCheckString := strings.Join(dataPairs, "\n")

	// secretKey = sha256(bot_token)
	secret := sha256.Sum256([]byte(botToken))

	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(dataCheckString))
	expectedMAC := mac.Sum(nil)
	expectedHash := hex.EncodeToString(expectedMAC)

	// Telegram присылает hash в hex, обычно в lowercase.
	if !hmac.Equal([]byte(expectedHash), []byte(strings.ToLower(body.Hash))) {
		return fmt.Errorf("telegram login hash mismatch")
	}

	return nil
}
