# Swagger / OpenAPI документация

Этот проект использует [`swag`](https://github.com/swaggo/swag) (swaggo) для генерации OpenAPI/Swagger спецификации из аннотаций в Go-коде.

## Где лежит Swagger UI

- В режиме **development** Swagger UI доступен по:
  - `/swagger/` (делает редирект на `/swagger/index.html`)
  - `/swagger/index.html`
- В режиме **production** Swagger UI **выключен** (роут не регистрируется), чтобы не светить документацию наружу.

Режим задаётся в конфиге: `app.mode`.

## Как генерировать документацию

Документация генерируется в пакет `swagger/` (это именно Go-пакет, который импортируется из `cmd/server/main.go` через `_ "anubis/swagger"`).

### Установка `swag` CLI

```bash
go install github.com/swaggo/swag/cmd/swag@v1.8.12
```

Проверь, что бинарь доступен:

```bash
$(go env GOPATH)/bin/swag --version
```

### Генерация (основная команда)

Запускать из корня репозитория:

```bash
rm -rf swagger
$(go env GOPATH)/bin/swag init -g cmd/server/main.go -o swagger --parseDependency --parseInternal
```

После этого появятся/обновятся файлы:
- `swagger/docs.go`
- `swagger/swagger.json`
- `swagger/swagger.yaml`

## Основные флаги `swag init`

Ниже — те флаги, которые чаще всего используются в этом проекте.

- **`-g <file>`**: entrypoint файл, где лежит общий блок аннотаций API (у нас `cmd/server/main.go`).
- **`-o <dir>`**: директория вывода (у нас `swagger`).
- **`--parseDependency`**: парсить зависимости (нужно, когда типы DTO/ответов и т.п. лежат не в том же пакете, что и хендлеры).
- **`--parseInternal`**: разрешить парсинг пакетов из `internal/`.

Полный список флагов:

```bash
$(go env GOPATH)/bin/swag init -h
```

## Где править описание эндпоинтов

Аннотации добавляются над хендлерами (контроллерами) и над общим описанием сервиса:

- **Общая информация API** (title/description/tags/security): `cmd/server/main.go`
- **OAuth2**: `app/api/controllers/oauth2_auth.controller.go`
- **Токены**: `app/api/controllers/token.controller.go`
- **Telegram**: `app/api/controllers/telegram_auth.controller.go`
- **Проекты**: `app/api/controllers/workspace.controller.go`
- **Health**: `app/api/routes/health_handlers.go`

## Типы в аннотациях и “blank references”

`swag` иногда не может найти типы, указанные в аннотациях (`DTO.X`, `schemes.Y`), если они не импортированы в файле.
Поэтому в некоторых контроллерах есть “пустые ссылки” вида:

- `_ = DTO.SomeType{}`
- `_ = schemes.SomeType{}`

Это сделано **только** для того, чтобы генератор Swagger корректно резолвил типы; на рантайм это не влияет.


