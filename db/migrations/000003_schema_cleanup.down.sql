-- 000003_schema_cleanup.down.sql
-- Rollback Stage 3: restore legacy `users` table and remove `user_profiles`.

BEGIN;

-- 1. Recreate legacy `users` table (minimal version – extend if more cols are required by old code).
CREATE TABLE IF NOT EXISTS public.users (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supabase_id  TEXT UNIQUE NOT NULL,
    email        TEXT UNIQUE NOT NULL,
    username     TEXT UNIQUE NOT NULL,
    first_name   TEXT,
    last_name    TEXT,
    avatar_url   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Drop the new `user_profiles` table.
DROP TABLE IF EXISTS public.user_profiles CASCADE;

COMMIT; 