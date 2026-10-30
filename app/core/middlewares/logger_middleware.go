package middlewares

import (
	schemes "anubis/app/core/schemes"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid" // Для генерации Request ID, если нет в заголовках
	"go.uber.org/zap"
)

// RequestLoggerMiddleware - основной middleware для логирования всех деталей запроса/ответа
func RequestLoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		// 1. Request ID
		requestID := c.Request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
			c.Request.Header.Set("X-Request-ID", requestID)
		}
		c.Set("requestID", requestID)

		// Обработка запроса
		c.Next()

		// 2. Фильтр шума (Optional)
		// Если это health-check, можно выйти и не логировать, чтобы не спамить
		if path == "/health" || path == "/metrics" {
			return
		}

		// 3. Сбор метрик
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		dataLength := c.Writer.Size() // Размер ответа в байтах
		if dataLength < 0 {
			dataLength = 0
		}

		// Сбор ошибок Gin (если были)
		errorMessage := ""
		var (
			errCode int
			errMsg  string
			origin  string
			stack   string
		)
		if len(c.Errors) > 0 {
			errorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()
			// Пытаемся извлечь структурированный код/сообщение (если это наша schemes.ErrorResponse).
			// Важно: не логируем "base" ошибку, чтобы не засветить чувствительные данные.
			for _, ge := range c.Errors.ByType(gin.ErrorTypePrivate) {
				var er *schemes.ErrorResponse
				if errors.As(ge.Err, &er) && er != nil {
					errCode = er.Code
					errMsg = er.Err
					origin = er.Origin
					if s, ok := schemes.StackString(er.ErrBase); ok {
						stack = s
					}
					break
				}
			}
		}

		// 4. Определение уровня лога
		logLevel := zap.InfoLevel
		if statusCode >= 500 {
			logLevel = zap.ErrorLevel
		} else if statusCode >= 400 || errorMessage != "" {
			logLevel = zap.WarnLevel
		}

		// 5. Создаем поля (ОПТИМИЗАЦИЯ: pre-allocate slice)
		// Мы знаем, что будет минимум 7-8 полей, выделяем память сразу
		fields := make([]zap.Field, 0, 10)

		fields = append(fields,
			zap.String("req_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("route", c.FullPath()), // шаблон роута Gin (удобнее сопоставлять с кодом)
			zap.String("query", rawQuery),     // Важно: параметры запроса
			zap.String("ip", clientIP),
			zap.Int("status", statusCode),
			zap.Int("size", dataLength), // Важно: размер ответа
			zap.Duration("lat", latency),
			zap.String("ua", c.Request.UserAgent()),
		)

		// Контекстные поля (User ID, Session ID)
		if userId := c.GetString(UserIDKey); userId != "" {
			fields = append(fields, zap.String("user_id", userId))
		}
		if sessionId := c.GetString(SessionIDKey); sessionId != "" {
			fields = append(fields, zap.String("session_id", sessionId))
		}
		// Домен/сервис из DomainMiddleware
		if domain := c.GetString(Domain); domain != "" {
			fields = append(fields, zap.String("domain", domain))
		}

		// Добавляем ошибку только если она есть.
		// NOTE: origin пишем всегда (если есть), stack — только для внутренних ошибок (WrapInternal).
		if errorMessage != "" {
			fields = append(fields, zap.String("error", errorMessage))
			if errCode != 0 {
				fields = append(fields,
					zap.Int("err_code", errCode),
					zap.String("err_msg", errMsg),
				)
			}
			if origin != "" {
				fields = append(fields, zap.String("origin", origin))
			}
			// Стек пишем только для “5xx” (т.е. тех, где ошибка была обёрнута через WrapInternal).
			if stack != "" {
				fields = append(fields, zap.String("stack", stack))
				logLevel = zap.ErrorLevel
			}
		}

		// Для access-логов caller бесполезен и всегда указывает на middleware — отключаем.
		logger.WithOptions(zap.WithCaller(false)).Log(logLevel, "HTTP", fields...)
	}
}
