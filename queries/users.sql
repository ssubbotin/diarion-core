-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByGoogleSub :one
SELECT * FROM users WHERE google_sub = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: InsertUser :one
INSERT INTO users (google_sub, email, email_verified, display_name, avatar_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = $2,
    avatar_url   = $3,
    updated_at   = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkUserDeleted :exec
UPDATE users
SET deleted_at     = NOW(),
    hard_delete_at = NOW() + INTERVAL '30 days',
    updated_at     = NOW()
WHERE id = $1;

-- name: RestoreUser :exec
UPDATE users
SET deleted_at     = NULL,
    hard_delete_at = NULL,
    updated_at     = NOW()
WHERE id = $1;

-- name: PurgeExpiredUsers :execrows
DELETE FROM users
WHERE hard_delete_at IS NOT NULL
  AND hard_delete_at <= NOW();
