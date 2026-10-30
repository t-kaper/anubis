-- +goose Up
-- +goose StatementBegin

-- Авторизация по email.
CREATE TABLE IF NOT EXISTS auth_email (
                                          id                BIGSERIAL PRIMARY KEY,
                                          user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                          email             TEXT NOT NULL UNIQUE,         -- приводим к lower() в приложении
                                          password_hash     TEXT NOT NULL,
                                          verified          BOOLEAN NOT NULL DEFAULT FALSE,
                                          forgot_code       TEXT,
                                          forgot_expires_at TIMESTAMPTZ,
                                          created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                          CONSTRAINT uq_auth_email_user UNIQUE (user_id)
);

-- Индекс для быстрых джойнов по user_id.
CREATE INDEX IF NOT EXISTS idx_auth_email_user_id
    ON auth_email(user_id);

-- Авторизация по телефону.
CREATE TABLE IF NOT EXISTS auth_phone (
                                          id            BIGSERIAL PRIMARY KEY,
                                          user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                          phone         BIGINT NOT NULL UNIQUE,
                                          country_code  INTEGER NOT NULL,
                                          password_hash TEXT,
                                          verified      BOOLEAN NOT NULL DEFAULT FALSE,
                                          created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                          CONSTRAINT uq_auth_phone_user UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_auth_phone_user_id
    ON auth_phone(user_id);

-- Виртуальная авторизация (логин/пароль без email/phone).
CREATE TABLE IF NOT EXISTS auth_virtual (
                                            id            BIGSERIAL PRIMARY KEY,
                                            user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                            login         TEXT NOT NULL UNIQUE,
                                            password_hash TEXT NOT NULL,
                                            verification  BOOLEAN NOT NULL DEFAULT TRUE,
                                            created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                            CONSTRAINT uq_auth_virtual_user UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_auth_virtual_user_id
    ON auth_virtual(user_id);

-- Привязанные OAuth-аккаунты (google, yandex и т.п.).
CREATE TABLE IF NOT EXISTS oauth_accounts (
                                              id               BIGSERIAL PRIMARY KEY,
                                              user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                              provider         TEXT NOT NULL,                 -- "google", "yandex", ...
                                              provider_user_id TEXT NOT NULL,                 -- sub/uid/id внутри провайдера
                                              name             TEXT,
                                              email            TEXT,
                                              updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                              created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

                                              CONSTRAINT uq_oauth_provider_user UNIQUE (provider, provider_user_id),
                                              CONSTRAINT uq_oauth_user_provider UNIQUE (user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_oauth_user_id
    ON oauth_accounts(user_id);

CREATE INDEX IF NOT EXISTS idx_oauth_provider
    ON oauth_accounts(provider);

-- Таблица с кодами подтверждения email.
CREATE TABLE IF NOT EXISTS email_code_auth (
                                               id            BIGSERIAL PRIMARY KEY,
                                               user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                               email         TEXT NOT NULL,
                                               email_code    TEXT NOT NULL,
                                               email_service TEXT NOT NULL,
                                               send_id       TEXT,
                                               created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                               expires_at    TIMESTAMPTZ NOT NULL
);

-- Индекс для поиска кодов по пользователю и времени.
CREATE INDEX IF NOT EXISTS idx_email_code_auth_user_created
    ON email_code_auth(user_id, created_at);

-- Таблица с кодами подтверждения телефона.
CREATE TABLE IF NOT EXISTS sms_code_auth (
                                             id          BIGSERIAL PRIMARY KEY,
                                             user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                             phone       BIGINT NOT NULL,
                                             sms_code    TEXT NOT NULL,
                                             sms_service TEXT NOT NULL,
                                             send_id     TEXT,
                                             created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                             expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sms_code_auth_user_created
    ON sms_code_auth(user_id, created_at);

-- Сессии пользователей.
CREATE TABLE IF NOT EXISTS user_sessions (
                                             id          BIGSERIAL PRIMARY KEY,
                                             user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                             domain      TEXT NOT NULL,
                                             device_id   TEXT NOT NULL,
                                             hash_token  TEXT NOT NULL,
                                             device_type TEXT NOT NULL,
                                             created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                             expires_at  TIMESTAMPTZ NOT NULL,
                                             ip          TEXT NOT NULL,
                                             is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

-- Индекс для поиска активных сессий пользователя.
CREATE INDEX IF NOT EXISTS idx_user_sessions_active
    ON user_sessions(user_id)
    WHERE is_active = TRUE;

-- Для частых запросов "все сессии юзера" можно добавить обычный индекс.
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id
    ON user_sessions(user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Откатываем в обратном порядке зависимостей/создания.

DROP INDEX IF EXISTS idx_user_sessions_user_id;
DROP INDEX IF EXISTS idx_user_sessions_active;
DROP TABLE IF EXISTS user_sessions;

DROP INDEX IF EXISTS idx_sms_code_auth_user_created;
DROP TABLE IF EXISTS sms_code_auth;

DROP INDEX IF EXISTS idx_email_code_auth_user_created;
DROP TABLE IF EXISTS email_code_auth;

DROP INDEX IF EXISTS idx_oauth_provider;
DROP INDEX IF EXISTS idx_oauth_user_id;
DROP TABLE IF EXISTS oauth_accounts;

DROP INDEX IF EXISTS idx_auth_virtual_user_id;
DROP TABLE IF EXISTS auth_virtual;

DROP INDEX IF EXISTS idx_auth_phone_user_id;
DROP TABLE IF EXISTS auth_phone;

DROP INDEX IF EXISTS idx_auth_email_user_id;
DROP TABLE IF EXISTS auth_email;

-- +goose StatementEnd
