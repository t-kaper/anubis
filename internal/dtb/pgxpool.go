package dtb

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"time"
)

// NewClientPgx создаёт пул соединений к PostgreSQL с проверкой подключения.
//
// Параметры:
//   - ctx         — контекст верхнего уровня (обычно из main).
//   - maxAttempts — сколько раз пробовать сделать ping к БД.
//   - maxDelay    — пауза между попытками ping (например, 2 * time.Second).
//   - databaseUrl — DSN к storage, например:
//     "storage://user:pass@host:port/dbname?sslmode=disable".
//   - maxpool     — максимальное количество соединений в пуле (MaxConns).
//
// Что делает функция:
//  1. Разбирает databaseUrl в pgxpool.Config через pgxpool.ParseConfig.
//  2. Устанавливает MaxConns (размер пула).
//  3. Создаёт пул pgxpool.NewWithConfig.
//  4. Через dtb.DoWithAttempts() несколько раз пробует pool.Ping(ctx).
//  5. Если все попытки провалены — log.Fatal(...), приложение падает.
//  6. Если всё хорошо — возвращает *pgxpool.Pool (живой, НЕ закрытый).
//
// ВАЖНО:
//   - Пул НЕ закрывается внутри функции.
//   - Закрывать нужно снаружи, обычно в main: defer ClosePostgresConnection(pool).
func NewClientPgx(
	ctx context.Context,
	maxAttempts int,
	maxDelay time.Duration,
	databaseUrl string,
	maxpool int,
) (pool *pgxpool.Pool, err error) {

	// 1. Парсим строку подключения в конфиг пула
	pgxCfg, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		log.Printf("Unable to parse storage config: %v\n", err)
		return nil, err
	}

	// 2. Задаём максимальное количество коннектов в пуле
	pgxCfg.MaxConns = int32(maxpool)

	// 3. Создаём пул соединений
	pool, err = pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		log.Printf("Failed to create PostgreSQL pool: %v\n", err)
		return nil, err
	}

	// 4. Пингуем БД с ретраями, чтобы убедиться, что она доступна.
	//    Используем ту же DoWithAttempts, что и для Mongo.
	err = DoWithAttemptsCtx(ctx, func() error {
		pingErr := pool.Ping(ctx)
		if pingErr != nil {
			log.Printf("Failed to connect to PostgreSQL: %v... Next attempt\n", pingErr)
			return pingErr
		}
		return nil
	}, maxAttempts, maxDelay)

	if err != nil {
		// Не смогли подключиться к Postgres даже после maxAttempts попыток.
		// Перед тем как упасть — корректно закрываем пул.
		pool.Close()
		log.Fatal("All attempts are exceeded. Unable to connect to PostgreSQL")
		// log.Fatal завершит процесс; return для формальности.
		return nil, err
	}

	// Всё ок — возвращаем пул вызывающей стороне.
	return pool, nil
}

// ClosePostgresConnection закрывает пул соединений к PostgreSQL.
//
// Вызывать обычно в main через defer:
//
//	pool, _ := NewClientPgx(...)
//	defer ClosePostgresConnection(pool)
func ClosePostgresConnection(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}

	pool.Close()
	log.Println("Connection pool to PostgreSQL closed.")
}
