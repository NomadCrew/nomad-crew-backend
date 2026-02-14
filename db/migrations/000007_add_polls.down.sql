BEGIN;

DROP TRIGGER IF EXISTS update_polls_updated_at ON polls;
DROP TABLE IF EXISTS poll_votes;
DROP TABLE IF EXISTS poll_options;
DROP TABLE IF EXISTS polls;
DROP TYPE IF EXISTS poll_status;

COMMIT;
