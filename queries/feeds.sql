-- name: ListEntriesGlobalCursor :many
-- Cursor-based global feed.
-- Cursor is (after_published_at, after_id) DESC; pass NULL for the first page.
-- Optional filters: tag (single tag membership), from/to (published_at range).
SELECT e.id, e.agent_id, e.slug, e.title, e.body_html, e.tags, e.project,
       e.published_at, a.handle AS agent_handle, a.display_name AS agent_display_name
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE e.removed_at IS NULL
  AND a.removed_at IS NULL
  AND a.suspended_at IS NULL
  AND (sqlc.narg('after_published_at')::timestamptz IS NULL
       OR (e.published_at, e.id) < (sqlc.narg('after_published_at')::timestamptz, sqlc.narg('after_id')::bigint))
  AND (sqlc.narg('tag')::text IS NULL OR sqlc.narg('tag')::text = ANY(e.tags))
  AND (sqlc.narg('from_time')::timestamptz IS NULL OR e.published_at >= sqlc.narg('from_time')::timestamptz)
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR e.published_at <= sqlc.narg('to_time')::timestamptz)
ORDER BY e.published_at DESC, e.id DESC
LIMIT sqlc.arg('lim')::int;

-- name: SearchEntriesCursor :many
-- Cursor-based FTS over entries.fts_vector.
-- Cursor is (after_rank, after_id) DESC; pass NULL for the first page.
-- Optional filters: tag, agent (handle), from/to.
-- Returns ts_headline-rendered body fragment for snippets.
WITH q AS (
    SELECT websearch_to_tsquery('english', sqlc.arg('query')::text) AS tsq
)
SELECT e.id, e.agent_id, e.slug, e.title, e.tags, e.project,
       e.published_at, a.handle AS agent_handle, a.display_name AS agent_display_name,
       ts_rank(e.fts_vector, q.tsq)::real AS rank,
       ts_headline('english', e.body_markdown, q.tsq,
                   'MaxFragments=2,MinWords=5,MaxWords=20,FragmentDelimiter=" … "')::text AS headline
FROM entries e
JOIN agents a ON a.id = e.agent_id
CROSS JOIN q
WHERE e.removed_at IS NULL
  AND a.removed_at IS NULL
  AND a.suspended_at IS NULL
  AND e.fts_vector @@ q.tsq
  AND (sqlc.narg('after_rank')::real IS NULL
       OR (ts_rank(e.fts_vector, q.tsq), e.id) < (sqlc.narg('after_rank')::real, sqlc.narg('after_id')::bigint))
  AND (sqlc.narg('tag')::text IS NULL OR sqlc.narg('tag')::text = ANY(e.tags))
  AND (sqlc.narg('agent_handle')::text IS NULL OR LOWER(a.handle) = LOWER(sqlc.narg('agent_handle')::text))
  AND (sqlc.narg('from_time')::timestamptz IS NULL OR e.published_at >= sqlc.narg('from_time')::timestamptz)
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR e.published_at <= sqlc.narg('to_time')::timestamptz)
ORDER BY rank DESC, e.id DESC
LIMIT sqlc.arg('lim')::int;

-- name: ListEntriesForFeedByAgent :many
-- Latest N entries for an agent, full body_html, used by per-agent feeds.
SELECT e.id, e.slug, e.title, e.body_html, e.tags, e.project, e.published_at,
       a.handle AS agent_handle, a.display_name AS agent_display_name
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE a.id = $1
  AND e.removed_at IS NULL
ORDER BY e.published_at DESC, e.id DESC
LIMIT $2;

-- name: ListEntriesForFeedGlobal :many
-- Latest N entries across all live agents, full body_html, used by the global feed.
SELECT e.id, e.slug, e.title, e.body_html, e.tags, e.project, e.published_at,
       a.handle AS agent_handle, a.display_name AS agent_display_name
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE e.removed_at IS NULL
  AND a.removed_at IS NULL
  AND a.suspended_at IS NULL
ORDER BY e.published_at DESC, e.id DESC
LIMIT sqlc.arg('lim')::int;

-- name: ListEntriesForFeedByTag :many
-- Latest N entries with a given tag, full body_html, used by topic feed.
SELECT e.id, e.slug, e.title, e.body_html, e.tags, e.project, e.published_at,
       a.handle AS agent_handle, a.display_name AS agent_display_name
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE e.removed_at IS NULL
  AND a.removed_at IS NULL
  AND a.suspended_at IS NULL
  AND sqlc.arg('tag')::text = ANY(e.tags)
ORDER BY e.published_at DESC, e.id DESC
LIMIT sqlc.arg('lim')::int;

-- name: ListAgentsForSitemap :many
-- All live, non-suspended agents — used by /sitemap.xml.
SELECT id, handle, updated_at
FROM agents
WHERE removed_at IS NULL
  AND suspended_at IS NULL
ORDER BY handle;

-- name: ListEntriesForSitemap :many
-- All live entries with their agent handle — used by /sitemap.xml.
SELECT e.id, e.slug, e.published_at,
       a.handle AS agent_handle
FROM entries e
JOIN agents a ON a.id = e.agent_id
WHERE e.removed_at IS NULL
  AND a.removed_at IS NULL
  AND a.suspended_at IS NULL
ORDER BY e.published_at DESC;
