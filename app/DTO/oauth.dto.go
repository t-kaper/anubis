package DTO

type UriProviderValid struct {
	Provider string `uri:"provider"   binding:"required"`
}

type OAuth2Query struct {
	DeviceID    string  `form:"device_id" json:"device_id" binding:"required,min=12,max=64"`
	InviteToken *string `form:"invite_token" json:"invite_token,omitempty" binding:"omitempty,min=10,max=128"`
}

type OAuth2CallbackQuery struct {
	Code  string `form:"code" binding:"required"`
	State string `form:"state" binding:"required"`
}

type AnswerTicketToken struct {
	Ticket      string `json:"ticket"`
	RedirectURL string `json:"redirect_url"`
}

// TelegramLoginWidgetDTO Фронт получает из виджета данные и получает device_id (генерирует его)
// и отправляет к нас на бекенд
type TelegramLoginWidgetDTO struct {
	ID          int64   `json:"id"         binding:"required"` // telegram user_id
	FirstName   string  `json:"first_name" binding:"required"`
	LastName    string  `json:"last_name"`                                   // опционально
	Username    string  `json:"username"`                                    // опционально
	PhotoURL    string  `json:"photo_url"`                                   // опционально
	AuthDate    int64   `json:"auth_date"  binding:"required"`               // unix timestamp
	Hash        string  `json:"hash"       binding:"required"`               // подпись от Telegram
	DeviceID    string  `json:"device_id"  binding:"required,min=12,max=64"` // твой device_id
	InviteToken *string `json:"invite_token,omitempty" binding:"omitempty,min=10,max=128"`

	// можно добавить доп. поля, если будешь их использовать:
	// LanguageCode string `json:"language_code"`
	// AllowsWriteToPm bool `json:"allows_write_to_pm"`
}
