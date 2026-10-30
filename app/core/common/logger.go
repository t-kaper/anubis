package common

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"time"
)

var ZapLogger *zap.Logger

func InitLogger(mode string) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder   // Время: 2023-10-05T14:48:00.000Z
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // Уровень: INFO, ERROR

	var core zapcore.Core

	if mode == "development" {
		// --- РАЗРАБОТКА (Красиво в консоли) ---
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // Цветные уровни
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		core = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.DebugLevel)
	} else {
		// --- ПРОДАКШН (JSON для Docker/Kibana) ---
		jsonEncoder := zapcore.NewJSONEncoder(encoderConfig)
		core = zapcore.NewCore(jsonEncoder, zapcore.AddSync(os.Stdout), zap.InfoLevel)
	}

	// Создаем логгер с опцией AddCaller (показывает файл и строку вызова)
	ZapLogger = zap.New(core, zap.AddCaller())
}

// Global logger access for convenience (consider passing logger explicitly in larger apps)
func Info(msg string, fields ...zap.Field) {
	ZapLogger.Info(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	ZapLogger.Debug(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	ZapLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	ZapLogger.Fatal(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	ZapLogger.Panic(msg, fields...)
}

func LogInfo(s string) {
	fmt.Printf("[LogInfo-error] %s | %s\n",
		time.Now().UTC(),
		s)
}
