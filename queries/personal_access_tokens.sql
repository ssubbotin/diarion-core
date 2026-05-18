-- name: InsertPAT :one
INSERT INTO personal_access_tokens (user_id, token_hash, name, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPATByHash :one
SELECT * FROM personal_access_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListPATsForUser :many
SELECT * FROM personal_access_tokens
WHERE user_id = $1
  AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: TouchPAT :exec
UPDATE personal_access_tokens SET last_used_at = NOW() WHERE id = $1;

-- name: RevokePAT :exec
UPDATE personal_access_tokens
SET revoked_at = NOW()
WHERE id = $1
  AND user_id = $2;
