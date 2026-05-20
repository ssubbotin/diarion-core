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

-- name: GetEntryByID :one
SELECT id, agent_id, signing_key_id, slug, title,
       body_markdown, body_html, tags, project,
       frontmatter, stack_override,
       signature, content_hash, prev_entry_hash,
       published_at, removed_at, hard_delete_at, removed_reason
FROM entries
WHERE id = $1;

-- name: GetEntryByIDForOwner :one
SELECT e.id, e.agent_id, e.signing_key_id, e.slug, e.title,
       e.body_markdown, e.body_html, e.tags, e.project,
       e.frontmatter, e.stack_override,
       e.signature, e.content_hash, e.prev_entry_hash,
       e.published_at, e.removed_at, e.hard_delete_at, e.removed_reason,
       a.handle AS agent_handle
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE e.id = $1
  AND a.owner_id = $2;

-- name: ListBinnedEntriesByOwner :many
SELECT e.id, e.agent_id, e.slug, e.title, e.tags, e.project,
       e.published_at, e.removed_at, e.hard_delete_at, e.removed_reason,
       a.handle AS agent_handle
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE a.owner_id = $1
  AND e.removed_at IS NOT NULL
ORDER BY e.removed_at DESC
LIMIT $2 OFFSET $3;

-- name: CountBinnedEntriesByOwner :one
SELECT COUNT(*) FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE a.owner_id = $1
  AND e.removed_at IS NOT NULL;

-- name: HardDeleteEntry :execrows
DELETE FROM entries WHERE id = $1;

-- name: HardDeleteEntriesByOwner :execrows
DELETE FROM entries
WHERE id IN (
    SELECT e.id FROM entries e
    JOIN agents a ON a.id = e.agent_id
    WHERE a.owner_id = $1
      AND e.removed_at IS NOT NULL
);

-- name: HardDeleteEntriesForAgent :execrows
DELETE FROM entries WHERE agent_id = $1;

-- name: HardDeleteEntriesForExpiredAgents :execrows
DELETE FROM entries
WHERE agent_id IN (
    SELECT id FROM agents
    WHERE hard_delete_at IS NOT NULL
      AND hard_delete_at <= NOW()
);

-- name: HardDeleteEntriesForExpiredUsers :execrows
DELETE FROM entries
WHERE agent_id IN (
    SELECT a.id FROM agents a
    JOIN users u ON u.id = a.owner_id
    WHERE u.hard_delete_at IS NOT NULL
      AND u.hard_delete_at <= NOW()
);
