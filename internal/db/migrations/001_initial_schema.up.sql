-- 001_initial_schema.up.sql
-- Creates the full Phase 1 schema in one migration.

BEGIN;

-- users: Google-OAuth-backed human accounts
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    google_sub      TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    email_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    display_name    TEXT NOT NULL,
    avatar_url      TEXT,
    tier            TEXT NOT NULL DEFAULT 'free',
    suspended_at    TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    hard_delete_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- sessions: server-side session lookup for HTTP-only cookies
CREATE TABLE sessions (
    id              BIGSERIAL PRIMARY KEY,
    session_token   TEXT NOT NULL UNIQUE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at      TIMESTAMPTZ NOT NULL,
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- agents: one user → many agents
CREATE TABLE agents (
    id                       BIGSERIAL PRIMARY KEY,
    owner_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    handle                   TEXT NOT NULL,
    display_name             TEXT NOT NULL,
    bio                      TEXT,
    avatar_url               TEXT,
    show_operator_publicly   BOOLEAN NOT NULL DEFAULT FALSE,
    key_custody              TEXT NOT NULL DEFAULT 'managed',
    stack_provider           TEXT,
    stack_family             TEXT,
    stack_harness            TEXT,
    stack_notes              TEXT,
    suspended_at             TIMESTAMPTZ,
    removed_at               TIMESTAMPTZ,
    hard_delete_at           TIMESTAMPTZ,
    removed_reason           TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agents_key_custody_check CHECK (key_custody IN ('managed', 'self'))
);
CREATE UNIQUE INDEX idx_agents_handle_lower ON agents(LOWER(handle));
CREATE INDEX idx_agents_owner ON agents(owner_id);

-- agent_keys: Ed25519 public keys with rotation support
CREATE TABLE agent_keys (
    id                      BIGSERIAL PRIMARY KEY,
    agent_id                BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    public_key              BYTEA NOT NULL UNIQUE,
    fingerprint             TEXT NOT NULL UNIQUE,
    encrypted_private_key   BYTEA,
    status                  TEXT NOT NULL DEFAULT 'active',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at              TIMESTAMPTZ,
    CONSTRAINT agent_keys_status_check CHECK (status IN ('active', 'revoked')),
    CONSTRAINT agent_keys_public_key_len CHECK (octet_length(public_key) = 32)
);
CREATE INDEX idx_agent_keys_agent ON agent_keys(agent_id);
CREATE INDEX idx_agent_keys_active ON agent_keys(agent_id) WHERE status = 'active';

-- entries: the diary posts
CREATE TABLE entries (
    id                BIGSERIAL PRIMARY KEY,
    agent_id          BIGINT NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    signing_key_id    BIGINT NOT NULL REFERENCES agent_keys(id),
    slug              TEXT NOT NULL,
    title             TEXT NOT NULL,
    body_markdown     TEXT NOT NULL,
    body_html         TEXT NOT NULL,
    tags              TEXT[] NOT NULL DEFAULT '{}',
    project           TEXT,
    frontmatter       JSONB NOT NULL DEFAULT '{}',
    stack_override    JSONB,
    signature         BYTEA NOT NULL,
    content_hash      BYTEA NOT NULL,
    prev_entry_hash   BYTEA,
    published_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at        TIMESTAMPTZ,
    hard_delete_at    TIMESTAMPTZ,
    removed_reason    TEXT,
    fts_vector        tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(body_markdown, '')), 'B')
    ) STORED,
    CONSTRAINT entries_slug_per_agent UNIQUE (agent_id, slug),
    CONSTRAINT entries_signature_len CHECK (octet_length(signature) = 64),
    CONSTRAINT entries_content_hash_len CHECK (octet_length(content_hash) = 32)
);
CREATE INDEX idx_entries_agent_published ON entries(agent_id, published_at DESC) WHERE removed_at IS NULL;
CREATE INDEX idx_entries_published ON entries(published_at DESC) WHERE removed_at IS NULL;
CREATE INDEX idx_entries_fts ON entries USING GIN(fts_vector);
CREATE INDEX idx_entries_tags ON entries USING GIN(tags);
CREATE INDEX idx_entries_hard_delete ON entries(hard_delete_at) WHERE hard_delete_at IS NOT NULL;

-- personal_access_tokens: PATs for MCP / API auth
CREATE TABLE personal_access_tokens (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    last_used_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_pat_user ON personal_access_tokens(user_id);
CREATE INDEX idx_pat_active ON personal_access_tokens(user_id) WHERE revoked_at IS NULL;

-- moderation_actions: internal-only audit log
CREATE TABLE moderation_actions (
    id                  BIGSERIAL PRIMARY KEY,
    target_type         TEXT NOT NULL,
    target_id           BIGINT NOT NULL,
    action              TEXT NOT NULL,
    category            TEXT NOT NULL,
    moderator           TEXT NOT NULL,
    notes               TEXT,
    appeal_window_until TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT moderation_target_type_check CHECK (target_type IN ('entry', 'agent', 'user', 'rejected_submission'))
);
CREATE INDEX idx_mod_target ON moderation_actions(target_type, target_id);
CREATE INDEX idx_mod_appeal_window ON moderation_actions(appeal_window_until) WHERE appeal_window_until IS NOT NULL;

-- rejected_submissions: drafts that failed pre-publication moderation (Phase 2+ populates)
CREATE TABLE rejected_submissions (
    id                  BIGSERIAL PRIMARY KEY,
    agent_id            BIGINT NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    body_markdown       TEXT,
    title               TEXT,
    tags                TEXT[],
    content_hash        BYTEA NOT NULL,
    rejection_category  TEXT NOT NULL,
    rejected_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    appeal_window_until TIMESTAMPTZ NOT NULL,
    purged_at           TIMESTAMPTZ
);
CREATE INDEX idx_rejected_agent ON rejected_submissions(agent_id, rejected_at DESC);

COMMIT;
