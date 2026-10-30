package dtb

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"time"
)

// NewClientMongo создаёт и настраивает подключение к MongoDB.
//
// Параметры:
//   - ctx         — контекст верхнего уровня (обычно из main), нужен для connect/ping.
//   - maxAttempts — сколько раз пробовать пропинговать Mongo перед тем, как сдаться.
//   - maxDelay    — пауза между попытками ping (например, 2 * time.Second).
//   - databaseURI — Mongo URI, например:
//     "mongodb://user:pass@host:port/db?authSource=admin".
//   - maxpool     — максимальный размер connection pool (SetMaxPoolSize).
//
// Что делает функция:
//  1. Собирает options.Client() с URI, максимальным пулом и таймаутом подключения.
//  2. Вызывает mongo.Connect() — создаёт клиент и открывает пул соединений.
//  3. Через dtb.DoWithAttempts() несколько раз пытается сделать Ping() к primary.
//  4. Если все попытки провалены — log.Fatal(...) (приложение падает).
//  5. Если всё ок — возвращает *mongo.Client (живой, НЕ закрытый).
//
// ВАЖНО:
//   - Клиент НЕ закрывается внутри функции.
//   - Закрывать подключение нужно снаружи, обычно в main: defer CloseMongoDBConnection(client).
func NewClientMongo(
	ctx context.Context,
	maxAttempts int,
	maxDelay time.Duration,
	databaseURI string,
	maxpool int,
) (client *mongo.Client, err error) {

	// Собираем настройки клиента MongoDB
	clientOptions := options.Client().ApplyURI(databaseURI)

	// Максимальное количество соединений в пуле
	clientOptions.SetMaxPoolSize(uint64(maxpool))

	// Таймаут установки соединения
	clientOptions.SetConnectTimeout(60 * time.Second)

	// Пытаемся создать клиента (ещё не факт, что БД доступна, просто поднимаем пул)
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("failed to create mongo client: %v", err)
		return nil, err
	}

	// Проверяем, что Mongo реально отвечает (ping к primary) с несколькими попытками.
	// DoWithAttemptsCtx вызывает callback несколько раз:
	//   если callback() вернул nil — считаем успешным и выходим,
	//   если все попытки вернули ошибку — возвращаем последнюю ошибку.
	err = DoWithAttemptsCtx(ctx, func() error {
		pingErr := client.Ping(ctx, readpref.Primary())
		if pingErr != nil {
			log.Printf("failed to ping mongo: %v", pingErr)
			return pingErr
		}
		return nil
	}, maxAttempts, maxDelay)

	if err != nil {
		// Все попытки подключения к Mongo провалились — логируем и падаем.
		log.Fatal("All attempts exceeded. Failed to connect to MongoDB")
		// log.Fatal вызовет os.Exit(1), но return оставим для формальной корректности:
		return nil, err
	}

	// Если сюда дошли — клиент рабочий, возвращаем его
	return client, nil
}

// CloseMongoDBConnection аккуратно закрывает подключение к MongoDB.
//
// Вызывать обычно в main через defer:
//
//		mongoURI := fmt.Sprintf(
//	   "mongodb://%s:%s@%s:%s/%s?authSource=admin",
//	   config.Mongo.User,
//	   config.Mongo.Password,
//	   config.Mongo.Host,
//	   config.Mongo.Port,
//	   config.Mongo.Database,
//
// )
//
//	client, _ := NewClientMongo(...)
//	defer CloseMongoDBConnection(client)
func CloseMongoDBConnection(client *mongo.Client) {
	if client == nil {
		return
	}

	if err := client.Disconnect(context.TODO()); err != nil {
		log.Fatal(err)
	}

	log.Println("Connection to MongoDB closed.")
}
