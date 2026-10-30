package middlewares

import (
	"anubis/app/core"
	"anubis/app/core/schemes"
	"fmt"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"strings"
)

const (
	BDName = "bd_name"
	Domain = "domain"
	Auth   = "auth"
)

// DomainMiddleware — middleware для определения домена по заголовку "Domain" или по Host из запроса.
// Извлекает домен, проверяет его наличие в конфигурации сервисов.
// Если домен не найден — возвращает ошибку 401.
// В случае успеха сохраняет домен, сервис и аутентификацию в контексте запроса.
// Фильтрация происходит следующим образом:
//
//  1. Сначала проверяется заголовок "Domain".
//
//  2. Если заголовок пуст, извлекается Host из запроса, разбивается по точке,
//     Примеры:
//     api.example.com → ["api", "example", "com"] → domain = "example" ✅
//     api.example.co.uk → ["api", "example", "co", "uk"] → domain = "co" ❌ (ожидается "example.co.uk")
//
//  3. По извлечённому домену ищется соответствующий сервис в config.ListServices.
//
//  4. Если сервис не найден — запрос прерывается с ошибкой 401.
func DomainMiddleware(config core.ServiceConfig) gin.HandlerFunc {
	// Возвращаем функцию типа gin.HandlerFunc, которая будет вызвана для каждого запроса.
	return func(c *gin.Context) {
		domainKey := ""

		// DEV: разрешаем руками прокидывать Domain header (удобно для локальной разработки).
		// PROD: игнорируем Domain header, чтобы клиент не мог подменить сервис.
		if strings.ToLower(strings.TrimSpace(config.App.Mode)) == "development" {
			domainKey = strings.ToLower(strings.TrimSpace(c.GetHeader("Domain")))
		}

		// Если Domain не задан — извлекаем ключ сервиса из Host/X-Forwarded-Host.
		if domainKey == "" {
			host := extractHost(c)

			// В production жёстко ограничиваем допустимые базовые домены (anti-spoofing).
			if strings.ToLower(strings.TrimSpace(config.App.Mode)) != "development" {
				if !isTrustedHostSuffix(host, config.App.TrustedHostSuffixes) {
					c.AbortWithStatusJSON(http.StatusUnauthorized, &schemes.HTTPError{Code: 104, Err: "untrusted host"})
					return
				}
			}

			domainKey = extractDomainKeyFromHost(host)
		}
		println(domainKey)
		// Проверяем, существует ли сервис, связанный с извлечённым доменом, в конфигурации.
		lService, ok := config.ListServices[domainKey]
		if !ok {
			// Если сервис не найден, прерываем выполнение и возвращаем ошибку 401.
			c.AbortWithStatusJSON(http.StatusUnauthorized, &schemes.HTTPError{Code: 104, Err: "Domain not found"})
			return
		}

		// Сохраняем информацию о сервисе, домене и аутентификации в контексте запроса,
		// чтобы другие обработчики могли к ним получить доступ.
		c.Set(BDName, lService.BDName)
		c.Set(Domain, domainKey)
		c.Set(Auth, lService.Auth)

		// Продолжаем выполнение цепочки middleware и обработчиков.
		c.Next()
	}
}

func extractHost(c *gin.Context) string {
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}

	// strip port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		// fallback: "host:port" without SplitHostPort parsing
		if i := strings.IndexByte(host, ':'); i >= 0 && !strings.Contains(host, "]") {
			host = host[:i]
		}
	}

	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host
}

// extractDomainKeyFromHost implements:
// - if host has subdomain part => return first label
// - else => return full host (e.g. "chanki.ru" -> "chanki.ru")
func extractDomainKeyFromHost(host string) string {
	if host == "" {
		return ""
	}
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}

func isTrustedHostSuffix(host string, suffixes []string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return false
	}
	for _, s := range suffixes {
		s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
		if s == "" {
			continue
		}
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}

// CheckAuthTypeMiddleware — middleware, который проверяет, разрешён ли указанный тип аутентификации
// для текущего запроса. Извлекает список разрешённых типов аутентификации из контекста (под ключом Auth),
// проверяет, содержится ли переданный typeAuth в этом списке.
// Если аутентификация не найдена в контексте или тип не разрешён — возвращает ошибку 403.
// В противном случае — продолжает выполнение цепочки обработчиков.
func CheckAuthTypeMiddleware(typeAuth string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Извлекаем данные аутентификации из контекста запроса.
		authIface, exists := c.Get(Auth)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, &schemes.HTTPError{Code: 106, Err: "Authorization data missing"})
			return
		}

		authList, ok := authIface.([]core.ServiceAuthMethod)
		if !ok {
			// Логируем для отладки, если вдруг придет что-то совсем неожиданное
			fmt.Printf("Type assertion failed. Expected []core.ServiceAuthMethod, got %T\n", authIface)

			c.AbortWithStatusJSON(http.StatusForbidden, &schemes.HTTPError{Code: 106, Err: "Invalid authorization data type"})
			return
		}

		// 2. ИСПРАВЛЕНИЕ: Проверка наличия.
		// Так как authList - это []core.ServiceAuthMethod, а typeAuth - это string,
		// функция utils.LittleContainsString может не принять этот список (зависит от её реализации).
		// Самый надежный и быстрый способ проверить здесь — простой цикл:

		isAllowed := false
		for _, method := range authList {
			// Сравниваем, приведя к строке, или кастуем typeAuth к ServiceAuthMethod
			if string(method) == typeAuth {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, &schemes.HTTPError{Code: 106, Err: "Authorization method denied"})
			return
		}

		c.Next()
	}
}
