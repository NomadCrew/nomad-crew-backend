-- 000003_schema_cleanup.up.sql
-- Stage 3: Schema Cleanup – remove legacy `users` table/view and finalise single-ID architecture.

BEGIN;

-- 1. Remove the legacy `users` artefact (it could be a table **or** a view, depending on env).
DROP TABLE IF EXISTS public.users CASCADE;
DROP VIEW  IF EXISTS public.users CASCADE;

-- 2. Ensure canonical `user_profiles` table exists (keyed to Supabase `auth.users.id`).
CREATE TABLE IF NOT EXISTS public.user_profiles (
    id          UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    email       TEXT UNIQUE NOT NULL,
    username    TEXT UNIQUE NOT NULL,
    first_name  TEXT,
    last_name   TEXT,
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Hard-enforce NOT NULL constraints on identity columns (if coming from older schemas).
ALTER TABLE public.user_profiles
    ALTER COLUMN email    SET NOT NULL,
    ALTER COLUMN username SET NOT NULL;

-- 4. Drop obsolete `supabase_id` column if it still lingers from previous migrations.
ALTER TABLE public.user_profiles
    DROP COLUMN IF EXISTS supabase_id;

COMMIT; 