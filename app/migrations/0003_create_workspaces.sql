-- +goose Up
-- +goose StatementBegin

-- Таблица проектов (группы/пулы по доменам/сервисам).
CREATE TABLE IF NOT EXISTS workspaces (
                                        id          BIGSERIAL PRIMARY KEY,
                                        name        TEXT NOT NULL,
                                        domain      TEXT NOT NULL,                      -- сервис/домен (chanki/yocha/...)
                                        max_members INTEGER NOT NULL DEFAULT 10,         -- лимит участников проекта (по умолчанию 10)
                                        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                        CONSTRAINT uq_workspaces_domain_name UNIQUE (domain, name)
);

-- Индекс на domain для быстрых выборок проектов по сервису.
CREATE INDEX IF NOT EXISTS idx_workspaces_domain
    ON workspaces(domain);

-- Защита от некорректного лимита.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_workspaces_max_members_positive'
    ) THEN
        ALTER TABLE workspaces
            ADD CONSTRAINT chk_workspaces_max_members_positive CHECK (max_members > 0);
    END IF;
END$$;

-- Участники проектов (многие-ко-многим: user <-> workspace).
CREATE TABLE IF NOT EXISTS workspace_members (
                                               workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
                                               user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                               role       TEXT NOT NULL,
                                               joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                               PRIMARY KEY (workspace_id, user_id)
);

-- Индекс для поиска всех проектов пользователя.
CREATE INDEX IF NOT EXISTS idx_workspace_members_user_id
    ON workspace_members(user_id);

-- Инвайты в проекты (одноразовые, с TTL).
CREATE TABLE IF NOT EXISTS workspace_invites (
    id              BIGSERIAL PRIMARY KEY,
    workspace_id      BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Храним ХЭШ токена (sha256 hex), сырой invite_token возвращаем только в API-ответе при создании.
    token_hash      TEXT NOT NULL UNIQUE,

    -- Роль, которая будет выдана пользователю при принятии инвайта (owner запрещаем в коде).
    role            TEXT NOT NULL,

    -- Ограничение по типу авторизации (NULL = любой).
    -- Например: "oauth2", "telegram".
    auth_method     TEXT,

    -- Если auth_method == "oauth2", можно ограничить конкретным провайдером.
    -- Например: "yandex", "google".
    oauth2_provider TEXT,

    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ,
    used_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_invites_workspace_id
    ON workspace_invites(workspace_id);

CREATE INDEX IF NOT EXISTS idx_workspace_invites_token_hash
    ON workspace_invites(token_hash);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_workspace_members_user_id;
DROP TABLE IF EXISTS workspace_members;

DROP INDEX IF EXISTS idx_workspace_invites_token_hash;
DROP INDEX IF EXISTS idx_workspace_invites_workspace_id;
DROP TABLE IF EXISTS workspace_invites;

DROP INDEX IF EXISTS idx_workspaces_domain;
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS chk_workspaces_max_members_positive;
DROP TABLE IF EXISTS workspaces;

-- +goose StatementEnd
