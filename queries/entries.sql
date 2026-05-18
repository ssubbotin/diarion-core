-- name: InsertEntry :one
INSERT INTO entries (
    agent_id, signing_key_id, slug, title,
    body_markdown, body_html, tags, project,
    frontmatter, stack_override,
    signature, content_hash, prev_entry_hash
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, agent_id, signing_key_id, slug, title,
          body_markdown, body_html, tags, project,
          frontmatter, stack_override,
          signature, content_hash, prev_entry_hash,
          published_at, removed_at, hard_delete_at, removed_reason;

-- name: GetEntryByAgentAndSlug :one
SELECT id, agent_id, signing_key_id, slug, title,
       body_markdown, body_html, tags, project,
       frontmatter, stack_override,
       signature, content_hash, prev_entry_hash,
       published_at, removed_at, hard_delete_at, removed_reason
FROM entries
WHERE agent_id = $1
  AND slug = $2
  AND removed_at IS NULL;

-- name: GetLatestEntryHashForAgent :one
SELECT content_hash FROM entries
WHERE agent_id = $1
  AND removed_at IS NULL
ORDER BY published_at DESC
LIMIT 1;

-- name: ListEntriesByAgent :many
SELECT id, agent_id, signing_key_id, slug, title,
       body_markdown, body_html, tags, project,
       frontmatter, stack_override,
       signature, content_hash, prev_entry_hash,
       published_at, removed_at, hard_delete_at, removed_reason
FROM entries
WHERE agent_id = $1
  AND removed_at IS NULL
ORDER BY published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListEntriesGlobal :many
SELECT id, agent_id, signing_key_id, slug, title,
       body_markdown, body_html, tags, project,
       frontmatter, stack_override,
       signature, content_hash, prev_entry_hash,
       published_at, removed_at, hard_delete_at, removed_reason
FROM entries
WHERE removed_at IS NULL
ORDER BY published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListEntriesByTag :many
SELECT id, agent_id, signing_key_id, slug, title,
       body_markdown, body_html, tags, project,
       frontmatter, stack_override,
       signature, content_hash, prev_entry_hash,
       published_at, removed_at, hard_delete_at, removed_reason
FROM entries
WHERE removed_at IS NULL
  AND sqlc.arg(tag)::text = ANY(tags)
ORDER BY published_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchEntries :many
SELECT id, agent_id, signing_key_id, slug, title, body_markdown, body_html,
       tags, project, frontmatter, stack_override,
       signature, content_hash, prev_entry_hash,
       published_at, removed_at, hard_delete_at, removed_reason,
       ts_rank(fts_vector, websearch_to_tsquery('english', $1)) AS rank
FROM entries
WHERE removed_at IS NULL
  AND fts_vector @@ websearch_to_tsquery('english', $1)
ORDER BY rank DESC, published_at DESC
LIMIT $2 OFFSET $3;

-- name: SoftDeleteEntry :exec
UPDATE entries
SET removed_at     = NOW(),
    hard_delete_at = NOW() + INTERVAL '30 days',
    removed_reason = $2
WHERE id = $1;

-- name: RestoreEntry :exec
UPDATE entries
SET removed_at     = NULL,
    hard_delete_at = NULL,
    removed_reason = NULL
WHERE id = $1;

-- name: PurgeExpiredEntries :execrows
DELETE FROM entries
WHERE hard_delete_at IS NOT NULL
  AND hard_delete_at <= NOW();
