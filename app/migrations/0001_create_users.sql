-- +goose Up
-- +goose StatementBegin

-- Таблица пользователей.
CREATE TABLE IF NOT EXISTS users (
                                     id          BIGSERIAL PRIMARY KEY,
                                     nickname    TEXT,
                                     is_virtual  BOOLEAN NOT NULL DEFAULT FALSE,     -- виртуальный пользователь (логин/пароль без email/phone)
                                     created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS users;

-- +goose StatementEnd
