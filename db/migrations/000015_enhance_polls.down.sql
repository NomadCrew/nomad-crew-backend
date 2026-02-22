-- Revert: Enhance Polls Migration
-- Note: golang-migrate wraps each migration in a transaction automatically.
-- Do NOT add BEGIN/COMMIT here.

-- Drop index
DROP INDEX IF EXISTS idx_polls_poll_type;

-- Re-add the UNIQUE constraint on poll_options
ALTER TABLE poll_options ADD CONSTRAINT poll_options_poll_id_text_key UNIQUE (poll_id, text);

-- Drop option_metadata column
ALTER TABLE poll_options DROP COLUMN IF EXISTS option_metadata;

-- Drop is_blind column
ALTER TABLE polls DROP COLUMN IF EXISTS is_blind;

-- Drop CHECK constraint then column
ALTER TABLE polls DROP CONSTRAINT IF EXISTS chk_poll_type;
ALTER TABLE polls DROP COLUMN IF EXISTS poll_type;
