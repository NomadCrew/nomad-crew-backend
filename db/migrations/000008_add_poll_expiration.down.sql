DROP INDEX IF EXISTS idx_polls_expires_at;
ALTER TABLE polls DROP COLUMN IF EXISTS expires_at;
