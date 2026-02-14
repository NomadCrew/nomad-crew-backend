-- Poll Expiration Migration
-- Note: golang-migrate wraps each migration in a transaction automatically.
-- Do NOT add BEGIN/COMMIT here.

-- Add expires_at column
ALTER TABLE polls ADD COLUMN expires_at TIMESTAMPTZ;

-- Backfill existing polls with 24h expiration from creation time
UPDATE polls SET expires_at = created_at + INTERVAL '24 hours' WHERE expires_at IS NULL;

-- Make it required after backfill
ALTER TABLE polls ALTER COLUMN expires_at SET NOT NULL;

-- Index for finding expired active polls efficiently
CREATE INDEX IF NOT EXISTS idx_polls_expires_at ON polls(expires_at) WHERE status = 'ACTIVE' AND deleted_at IS NULL;
