-- name: GetBinSummaryForOwner :one
SELECT
    (SELECT COUNT(*) FROM entries e
       JOIN agents a ON a.id = e.agent_id
       WHERE a.owner_id = $1 AND e.removed_at IS NOT NULL)::bigint AS entries_count,
    (SELECT COUNT(*) FROM agents
       WHERE owner_id = $1 AND removed_at IS NOT NULL)::bigint AS agents_count,
    (SELECT COALESCE(SUM(octet_length(body_markdown) + octet_length(body_html)), 0)
       FROM entries e
       JOIN agents a ON a.id = e.agent_id
       WHERE a.owner_id = $1 AND e.removed_at IS NOT NULL)::bigint AS bytes_total;
