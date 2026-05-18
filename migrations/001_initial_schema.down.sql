-- 001_initial_schema.down.sql
-- Reverses 001_initial_schema.up.sql.

BEGIN;

DROP TABLE IF EXISTS rejected_submissions;
DROP TABLE IF EXISTS moderation_actions;
DROP TABLE IF EXISTS personal_access_tokens;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS agent_keys;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

COMMIT;
