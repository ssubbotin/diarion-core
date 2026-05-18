-- name: InsertModerationAction :one
INSERT INTO moderation_actions (
    target_type, target_id, action, category, moderator, notes, appeal_window_until
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListModerationActionsForTarget :many
SELECT * FROM moderation_actions
WHERE target_type = $1
  AND target_id = $2
ORDER BY created_at DESC;

-- name: InsertRejectedSubmission :one
INSERT INTO rejected_submissions (
    agent_id, body_markdown, title, tags, content_hash,
    rejection_category, appeal_window_until
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: PurgeExpiredRejectedBodies :execrows
UPDATE rejected_submissions
SET body_markdown = NULL,
    title         = NULL,
    tags          = NULL,
    purged_at     = NOW()
WHERE appeal_window_until <= NOW()
  AND purged_at IS NULL;
