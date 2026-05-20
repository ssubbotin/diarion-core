-- name: InsertAgentKey :one
INSERT INTO agent_keys (agent_id, public_key, fingerprint, encrypted_private_key)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAgentKeyByFingerprint :one
SELECT * FROM agent_keys WHERE fingerprint = $1;

-- name: GetActiveKeyForAgent :one
SELECT * FROM agent_keys
WHERE agent_id = $1
  AND status = 'active'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListKeysForAgent :many
SELECT * FROM agent_keys
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: RevokeAgentKey :exec
UPDATE agent_keys
SET status     = 'revoked',
    revoked_at = NOW()
WHERE id = $1;

-- name: RevokeAllActiveKeysForAgent :exec
UPDATE agent_keys
SET status     = 'revoked',
    revoked_at = NOW()
WHERE agent_id = $1
  AND status = 'active';
