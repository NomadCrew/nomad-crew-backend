-- Polls Feature Migration
-- Follows existing patterns: UUID PKs, TIMESTAMPTZ, soft deletes, CASCADE
-- Note: golang-migrate wraps each migration in a transaction automatically.
-- Do NOT add BEGIN/COMMIT here.

-- Ensure the updated_at trigger function exists.
-- This was originally defined in 000001_init but may be missing on production
-- databases where that migration was applied manually before golang-migrate.
-- CREATE OR REPLACE is idempotent — safe for all environments.
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW IS DISTINCT FROM OLD THEN
        NEW.updated_at = CURRENT_TIMESTAMP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enum for poll status
DO $$ BEGIN
    CREATE TYPE poll_status AS ENUM ('ACTIVE', 'CLOSED');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Polls table
CREATE TABLE IF NOT EXISTS polls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    question VARCHAR(500) NOT NULL,
    allow_multiple_votes BOOLEAN NOT NULL DEFAULT FALSE,
    status poll_status NOT NULL DEFAULT 'ACTIVE',
    created_by UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    closed_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Poll options table
CREATE TABLE IF NOT EXISTS poll_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    text VARCHAR(200) NOT NULL,
    position SMALLINT NOT NULL DEFAULT 0,
    created_by UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(poll_id, text)
);

-- Poll votes table
CREATE TABLE IF NOT EXISTS poll_votes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(poll_id, option_id, user_id)
);

-- Indexes (IF NOT EXISTS requires PostgreSQL 9.5+)
CREATE INDEX IF NOT EXISTS idx_polls_trip_id ON polls(trip_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_polls_created_by ON polls(created_by) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_polls_status ON polls(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_poll_options_poll_id ON poll_options(poll_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_poll_option ON poll_votes(poll_id, option_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_poll_user ON poll_votes(poll_id, user_id);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_polls_updated_at ON polls;
CREATE TRIGGER update_polls_updated_at
    BEFORE UPDATE ON polls
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
