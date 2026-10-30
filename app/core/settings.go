package core

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

type WebhookConfig struct {
	TokenRevoked string `mapstructure:"token_revoked"` // URL для уведомлений о дропе токена
}

type ProviderOauth struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"` // можно переопределять через ENV
	RedirectURL  string `mapstructure:"redirect_url"`
	Redirect     string `mapstructure:"redirect"` // куда редиректить фронт после успешного логина
}

// RoleVariantConfig — конфиг варианта роли (например manager:all).
type RoleVariantConfig struct {
	Code        string `mapstructure:"code"`        // опционально: короткий код для бизнес-сервиса
	Description string `mapstructure:"description"` // описание варианта
}

// RoleConfig — конфиг роли (например manager) и её вариантов (например manager:all).
type RoleConfig struct {
	Code        string                       `mapstructure:"code"`        // опционально: короткий код роли
	Description string                       `mapstructure:"description"` // описание роли
	Variants    map[string]RoleVariantConfig `mapstructure:"variants"`    // опционально
}

type ServiceAuthMethod string

// Например: "phone", "email", "oauth2"
const (
	AuthPhone  ServiceAuthMethod = "phone"
	AuthEmail  ServiceAuthMethod = "email"
	AuthOAuth2 ServiceAuthMethod = "oauth2"
)

// Описание одного сервиса/домена (chanki, yocha, и т.д.)
type ListServices struct {
	BDName          string `mapstructure:"bd_name"` // имя, которое используешь в БД (идентификатор арендатора/сервиса)
	SingleWorkspace bool   `mapstructure:"single_workspace"`
	SingleMember    bool   `mapstructure:"single_member"`
	StateSecret     string `mapstructure:"state_secret"`
	MaxDevicesUser  int    `mapstructure:"max_devices_user"`
	// DefaultMaxMembers — дефолтный лимит участников для НОВЫХ проектов в домене.
	// Если ключ отсутствует в YAML, будет использовано значение по умолчанию (10).
	DefaultMaxMembers *int                     `mapstructure:"default_max_members"`
	Webhook           WebhookConfig            `mapstructure:"webhook"`
	Auth              []ServiceAuthMethod      `mapstructure:"auth"`           // какие типы авторизации разрешены
	Roles             map[string]RoleConfig    `mapstructure:"roles"`          // справочник ролей для домена (включая варианты)
	Provider          map[string]ProviderOauth `mapstructure:"provider_oauth"` // "google", "yandex", "gosuslugi"
}

// Конфиг Postgres
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"sslmode"`  // disable / require / verify-full
	MaxPool  int32  `mapstructure:"max_pool"` // размер пула pgx
}

// Конфиг Mongo
type MongoConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Database string `mapstructure:"database"`  // готовый mongo URI
	MaxPool  uint64 `mapstructure:"max_pool"`  // SetMaxPoolSize
	TimeoutS int    `mapstructure:"timeout_s"` // таймаут коннекта
}

// JWT / токены
type JWTConfig struct {
	AccessTokenSecret  string `mapstructure:"access_token_secret"`
	TicketTokenSecret  string `mapstructure:"ticket_token_secret"`
	RefreshTokenSecret string `mapstructure:"refresh_token_secret"`
	AccessTokenMinute  int    `mapstructure:"access_token_minute"`
	RefreshTokenMinute int    `mapstructure:"refresh_token_minute"`
}

// Общий конфиг приложения
type AppConfig struct {
	IP      string `mapstructure:"ip"` // 0.0.0.0:1012
	URL     string `mapstructure:"url"`
	Mode    string `mapstructure:"mode"`    // development / production
	Version string `mapstructure:"version"` // можно забрать из ENV
	// TrustedHostSuffixes — список доверенных базовых доменов для Host/X-Forwarded-Host.
	// Примеры: ["chanki.ru", "test.ru"].
	// В production рекомендуется требовать непустой список и отклонять запросы с Host вне allowlist.
	TrustedHostSuffixes []string `mapstructure:"trusted_host_suffixes"`
}

// Финальная структура конфига сервиса
type ServiceConfig struct {
	App          AppConfig               `mapstructure:"app"`
	Postgres     PostgresConfig          `mapstructure:"postgres"`
	Mongo        MongoConfig             `mapstructure:"mongo"`
	JWT          JWTConfig               `mapstructure:"jwt"`
	ListServices map[string]ListServices `mapstructure:"list_services"`
}

// Validate performs minimal config validation. We fail fast on misconfiguration,
// because auth service must not start with undefined role model.
func (s *ServiceConfig) Validate() error {
	for domainKey, svc := range s.ListServices {
		if strings.TrimSpace(svc.BDName) == "" {
			return fmt.Errorf("config error: list_services.%s.bd_name is required and must not be empty", domainKey)
		}
		if svc.DefaultMaxMembers != nil && *svc.DefaultMaxMembers <= 0 {
			return fmt.Errorf("config error: list_services.%s.default_max_members must be > 0", domainKey)
		}
		if len(svc.Roles) == 0 {
			return fmt.Errorf("config error: list_services.%s.roles is required and must not be empty", domainKey)
		}

		// IMPORTANT (business rule):
		// - owner/admin roles must exist for every domain
		// - they are "almost equal" by permissions, except owner can delete workspace and admin cannot
		for _, required := range []string{"owner", "admin"} {
			if _, ok := svc.Roles[required]; !ok {
				return fmt.Errorf("config error: list_services.%s.roles.%s is required", domainKey, required)
			}
		}
	}
	return nil
}

func (s *ServiceConfig) ReadConfig(pathYaml string) {
	// ---------- 1. Устанавливаем префикс для ENV переменных ----------
	viper.SetEnvPrefix("ANUBIS")
	// Теперь Viper будет искать переменные с префиксом ANUBIS_
	// Например: ANUBIS_APP_IP, ANUBIS_POSTGRES_USER и т.д.

	// 2. Сначала настраиваем, как точки мапятся в нижнее подчёркивание
	//    list_services.trawler.state_secret -> ANUBIS_LIST_SERVICES_TRAWLER_STATE_SECRET
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// ---------- 2. Читаем YAML и мёржим поверх ----------
	//
	// YAML удобен для вложенных структур вроде list_services, ролей, oauth и т.п.
	// MergeInConfig() не затирает то, что уже прочитано, а ДОБАВЛЯЕТ/ПЕРЕЗАПИСЫВАЕТ.
	if pathYaml != "" {
		viper.SetConfigFile(pathYaml)
		if err := viper.MergeInConfig(); err != nil {
			log.Fatalf("YAML ERROR: %v", err)
		}
	}

	// ---------- 3. Разрешаем использовать реальные ENV-переменные ----------
	//
	// Теперь, если ты в Docker/сервере задашь:
	//   POSTGRES_USER=prod_user
	// то это переопределит значение из файлов.
	viper.AutomaticEnv()

	// ---------- 4. Настраиваем, как мапятся ключи -> ENV ----------
	//
	// Если когда-нибудь появятся вложенные ключи типа:
	//   some.nested.key
	// то Viper будет искать ENV:
	//   SOME_NESTED_KEY
	//

	// ---------- 5. Unmarshal в структуру ----------
	if err := viper.Unmarshal(s); err != nil {
		log.Fatalf("CONFIG ERROR: cannot unmarshal config: %v", err)
	}
}

func (s *ServiceConfig) PrintConfigJSON() {
	jsonData, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling config: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}
