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
