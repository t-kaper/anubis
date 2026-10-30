# Миграции БД с помощью `goose`

## 🔑 Доступ через ENV переменные

```bash
# Обязательные переменные
export GOOSE_DRIVER=storage
export GOOSE_DBSTRING='user=anubis_admin password=7f4327f6c$8sdd-98bd-7agdf6c-324a dbname=anubis host=127.0.0.1 port=25432 sslmode=disable'
export GOOSE_MIGRATION_DIR=./app/migrations
```

---

## 🚀 Основные команды

### Статус миграций
```bash
goose status
```

### Применить все новые миграции
```bash
goose up
```

### Применить одну миграцию
```bash
goose up-by-one
```

### Откатить последнюю миграцию
```bash
goose down
```

### Откатить до конкретной версии
```bash
goose down-to 0002  # откатит всё после 0002_
```

### Полный сброс (удалить все таблицы)
```bash
goose reset
```

### Создать новую миграцию
```bash
goose create add_users_table sql
```

---

## 📁 Структура миграций

```
app/migrations/
├── 0001_create_users.sql      # пользователи
├── 0002_create_auth.sql       # авторизация  
└── 0003_create_workspaces.sql   # проекты
```

**Нумерация по блокам:**
- `0001` - пользователи
- `0002` - авторизация
- `0003` - проекты

---

## 📝 Формат SQL-файла

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE
);
-- +goose StatementEnd

-- +goose Down  
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

---

## ⚡ Быстрые сценарии

### Развернуть БД с нуля
```bash
goose up
```

### Откатить только проекты (оставить users + auth)
```bash
goose down-to 0002
```

### Откатить авторизацию (оставить только users)
```bash
goose down-to 0001
```

### Проверить что применено
```bash
goose status
```

---

## 🔧 Makefile (опционально)

```makefile
migrate-status:
	goose status

migrate-up:
	goose up

migrate-down:
	goose down

migrate-down-to:
	goose down-to $(VERSION)

migrate-reset:
	goose reset
```

Использование:
```bash
make migrate-up
make migrate-down-to VERSION=0002
```