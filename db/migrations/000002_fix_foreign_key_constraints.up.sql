-- Migration: Fix Foreign Key Constraints to Reference auth.users
-- This migration fixes the schema mismatch where some foreign keys were pointing to public.users instead of auth.users

-- Step 1: Populate auth.users table with existing user data
INSERT INTO auth.users (id, email, created_at, updated_at) 
SELECT supabase_id::uuid, email, created_at, updated_at 
FROM users 
WHERE supabase_id IS NOT NULL 
ON CONFLICT (id) DO NOTHING;

-- Step 2: Update existing data to use Supabase IDs instead of internal UUIDs

-- Update trips.created_by to use Supabase IDs
UPDATE trips 
SET created_by = u.supabase_id::uuid 
FROM users u 
WHERE trips.created_by = u.id;

-- Update expenses.user_id to use Supabase IDs
UPDATE expenses 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE expenses.user_id = u.id;

-- Update locations.user_id to use Supabase IDs
UPDATE locations 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE locations.user_id = u.id;

-- Update trip_memberships.user_id to use Supabase IDs
UPDATE trip_memberships 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE trip_memberships.user_id = u.id;

-- Update trip_invitations.inviter_id to use Supabase IDs
UPDATE trip_invitations 
SET inviter_id = u.supabase_id::uuid 
FROM users u 
WHERE trip_invitations.inviter_id = u.id;

-- Update trip_invitations.invitee_id to use Supabase IDs
UPDATE trip_invitations 
SET invitee_id = u.supabase_id::uuid 
FROM users u 
WHERE trip_invitations.invitee_id = u.id;

-- Update todos.created_by to use Supabase IDs
UPDATE todos 
SET created_by = u.supabase_id::uuid 
FROM users u 
WHERE todos.created_by = u.id;

-- Update notifications.user_id to use Supabase IDs
UPDATE notifications 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE notifications.user_id = u.id;

-- Update chat_groups.created_by to use Supabase IDs
UPDATE chat_groups 
SET created_by = u.supabase_id::uuid 
FROM users u 
WHERE chat_groups.created_by = u.id;

-- Update chat_messages.user_id to use Supabase IDs
UPDATE chat_messages 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE chat_messages.user_id = u.id;

-- Update chat_group_members.user_id to use Supabase IDs
UPDATE chat_group_members 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE chat_group_members.user_id = u.id;

-- Update chat_message_reactions.user_id to use Supabase IDs
UPDATE chat_message_reactions 
SET user_id = u.supabase_id::uuid 
FROM users u 
WHERE chat_message_reactions.user_id = u.id;

-- Step 3: Drop existing foreign key constraints that point to public.users

-- Check and drop constraints if they exist and point to wrong table
DO $$
BEGIN
    -- Drop trips foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trips_created_by_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE trips DROP CONSTRAINT trips_created_by_fkey;
    END IF;
    
    -- Drop expenses foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'expenses_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE expenses DROP CONSTRAINT expenses_user_id_fkey;
    END IF;
    
    -- Drop locations foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'locations_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE locations DROP CONSTRAINT locations_user_id_fkey;
    END IF;
    
    -- Drop trip_memberships foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_memberships_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE trip_memberships DROP CONSTRAINT trip_memberships_user_id_fkey;
    END IF;
    
    -- Drop trip_invitations inviter_id foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_inviter_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE trip_invitations DROP CONSTRAINT trip_invitations_inviter_id_fkey;
    END IF;
    
    -- Drop trip_invitations invitee_id foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_invitee_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE trip_invitations DROP CONSTRAINT trip_invitations_invitee_id_fkey;
    END IF;
    
    -- Drop todos foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'todos_created_by_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE todos DROP CONSTRAINT todos_created_by_fkey;
    END IF;
    
    -- Drop notifications foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'notifications_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE notifications DROP CONSTRAINT notifications_user_id_fkey;
    END IF;
    
    -- Drop chat_groups foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_groups_created_by_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE chat_groups DROP CONSTRAINT chat_groups_created_by_fkey;
    END IF;
    
    -- Drop chat_messages foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_messages_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE chat_messages DROP CONSTRAINT chat_messages_user_id_fkey;
    END IF;
    
    -- Drop chat_group_members foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_group_members_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE chat_group_members DROP CONSTRAINT chat_group_members_user_id_fkey;
    END IF;
    
    -- Drop chat_message_reactions foreign key if it points to public.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_message_reactions_user_id_fkey' AND confrelid = 'users'::regclass) THEN
        ALTER TABLE chat_message_reactions DROP CONSTRAINT chat_message_reactions_user_id_fkey;
    END IF;
END $$;

-- Step 4: Add correct foreign key constraints pointing to auth.users

-- Add foreign key constraints (only if they don't already exist and point to auth.users)
DO $$
BEGIN
    -- Add trips foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trips_created_by_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trips ADD CONSTRAINT trips_created_by_fkey 
        FOREIGN KEY (created_by) REFERENCES auth.users(id) ON DELETE SET NULL;
    END IF;
    
    -- Add expenses foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'expenses_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE expenses ADD CONSTRAINT expenses_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add locations foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'locations_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE locations ADD CONSTRAINT locations_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add trip_memberships foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_memberships_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trip_memberships ADD CONSTRAINT trip_memberships_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add trip_invitations inviter_id foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_inviter_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trip_invitations ADD CONSTRAINT trip_invitations_inviter_id_fkey 
        FOREIGN KEY (inviter_id) REFERENCES auth.users(id) ON DELETE SET NULL;
    END IF;
    
    -- Add trip_invitations invitee_id foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_invitee_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trip_invitations ADD CONSTRAINT trip_invitations_invitee_id_fkey 
        FOREIGN KEY (invitee_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add todos foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'todos_created_by_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE todos ADD CONSTRAINT todos_created_by_fkey 
        FOREIGN KEY (created_by) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add notifications foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'notifications_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE notifications ADD CONSTRAINT notifications_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add chat_groups foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_groups_created_by_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_groups ADD CONSTRAINT chat_groups_created_by_fkey 
        FOREIGN KEY (created_by) REFERENCES auth.users(id) ON DELETE SET NULL;
    END IF;
    
    -- Add chat_messages foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_messages_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE SET NULL;
    END IF;
    
    -- Add chat_group_members foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_group_members_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_group_members ADD CONSTRAINT chat_group_members_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
    
    -- Add chat_message_reactions foreign key
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_message_reactions_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_message_reactions ADD CONSTRAINT chat_message_reactions_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;
    END IF;
END $$; 