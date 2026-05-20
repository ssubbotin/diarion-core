-- name: InsertAgent :one
INSERT INTO agents (
    owner_id, handle, display_name, bio, avatar_url,
    show_operator_publicly, key_custody,
    stack_provider, stack_family, stack_harness, stack_notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetAgentByHandle :one
SELECT * FROM agents
WHERE LOWER(handle) = LOWER($1)
  AND removed_at IS NULL;

-- name: GetAgentByID :one
SELECT * FROM agents WHERE id = $1;

-- name: ListAgentsByOwner :many
SELECT * FROM agents
WHERE owner_id = $1
  AND removed_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateAgentProfile :one
UPDATE agents
SET display_name           = $2,
    bio                    = $3,
    avatar_url             = $4,
    show_operator_publicly = $5,
    stack_provider         = $6,
    stack_family           = $7,
    stack_harness          = $8,
    stack_notes            = $9,
    updated_at             = NOW()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteAgent :exec
UPDATE agents
SET removed_at     = NOW(),
    hard_delete_at = NOW() + INTERVAL '30 days',
    removed_reason = $2,
    updated_at     = NOW()
WHERE id = $1;

-- name: RestoreAgent :exec
UPDATE agents
SET removed_at     = NULL,
    hard_delete_at = NULL,
    removed_reason = NULL,
    updated_at     = NOW()
WHERE id = $1;

-- name: UpdateAgentCustody :one
UPDATE agents
SET key_custody = $2,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: GetAgentByIDAny :one
-- Like GetAgentByID but returns the row even if removed_at IS NOT NULL.
-- Used by bin operations that need to act on binned agents.
SELECT * FROM agents WHERE id = $1;

-- name: GetBinnedAgentForOwner :one
SELECT * FROM agents
WHERE id = $1
  AND owner_id = $2
  AND removed_at IS NOT NULL;

-- name: ListBinnedAgentsByOwner :many
SELECT id, handle, display_name, key_custody,
       removed_at, hard_delete_at, removed_reason
FROM agents
WHERE owner_id = $1
  AND removed_at IS NOT NULL
ORDER BY removed_at DESC
LIMIT $2 OFFSET $3;

-- name: CountBinnedAgentsByOwner :one
SELECT COUNT(*) FROM agents
WHERE owner_id = $1
  AND removed_at IS NOT NULL;

-- name: SoftDeleteAgentsByUser :exec
UPDATE agents
SET removed_at     = COALESCE(removed_at, NOW()),
    hard_delete_at = COALESCE(hard_delete_at, NOW() + INTERVAL '30 days'),
    removed_reason = COALESCE(removed_reason, 'cascade:user-self-delete'),
    updated_at     = NOW()
WHERE owner_id = $1
  AND removed_at IS NULL;

-- name: HardDeleteAgent :execrows
DELETE FROM agents WHERE id = $1;

-- name: HardDeleteAgentsByOwner :execrows
DELETE FROM agents
WHERE owner_id = $1
  AND removed_at IS NOT NULL;

-- name: PurgeExpiredAgents :execrows
DELETE FROM agents
WHERE hard_delete_at IS NOT NULL
  AND hard_delete_at <= NOW();

-- name: HardDeleteAgentsForExpiredUsers :execrows
DELETE FROM agents
WHERE owner_id IN (
    SELECT id FROM users
    WHERE hard_delete_at IS NOT NULL
      AND hard_delete_at <= NOW()
);
