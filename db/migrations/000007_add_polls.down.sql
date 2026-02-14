-- Rollback polls feature
-- Note: golang-migrate wraps each migration in a transaction automatically.
-- Do NOT add BEGIN/COMMIT here.

DROP TRIGGER IF EXISTS update_polls_updated_at ON polls;
DROP TABLE IF EXISTS poll_votes;
DROP TABLE IF EXISTS poll_options;
DROP TABLE IF EXISTS polls;
DROP TYPE IF EXISTS poll_status;
