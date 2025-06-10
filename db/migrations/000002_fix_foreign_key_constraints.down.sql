-- Rollback Migration: Revert Foreign Key Constraints to Reference public.users
-- This migration reverts the changes made in 000002_fix_foreign_key_constraints.up.sql

-- Step 1: Drop foreign key constraints that point to auth.users
DO $$
BEGIN
    -- Drop all foreign key constraints pointing to auth.users
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trips_created_by_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trips DROP CONSTRAINT trips_created_by_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'expenses_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE expenses DROP CONSTRAINT expenses_user_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'locations_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE locations DROP CONSTRAINT locations_user_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_memberships_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trip_memberships DROP CONSTRAINT trip_memberships_user_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_inviter_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trip_invitations DROP CONSTRAINT trip_invitations_inviter_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_invitee_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE trip_invitations DROP CONSTRAINT trip_invitations_invitee_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'todos_created_by_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE todos DROP CONSTRAINT todos_created_by_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'notifications_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE notifications DROP CONSTRAINT notifications_user_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_groups_created_by_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_groups DROP CONSTRAINT chat_groups_created_by_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_messages_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_messages DROP CONSTRAINT chat_messages_user_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_group_members_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_group_members DROP CONSTRAINT chat_group_members_user_id_fkey;
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_message_reactions_user_id_fkey' AND confrelid = 'auth.users'::regclass) THEN
        ALTER TABLE chat_message_reactions DROP CONSTRAINT chat_message_reactions_user_id_fkey;
    END IF;
END $$;

-- Step 2: Revert data to use internal UUIDs instead of Supabase IDs

-- Revert trips.created_by to use internal UUIDs
UPDATE trips 
SET created_by = u.id 
FROM users u 
WHERE trips.created_by = u.supabase_id::uuid;

-- Revert expenses.user_id to use internal UUIDs
UPDATE expenses 
SET user_id = u.id 
FROM users u 
WHERE expenses.user_id = u.supabase_id::uuid;

-- Revert locations.user_id to use internal UUIDs
UPDATE locations 
SET user_id = u.id 
FROM users u 
WHERE locations.user_id = u.supabase_id::uuid;

-- Revert trip_memberships.user_id to use internal UUIDs
UPDATE trip_memberships 
SET user_id = u.id 
FROM users u 
WHERE trip_memberships.user_id = u.supabase_id::uuid;

-- Revert trip_invitations.inviter_id to use internal UUIDs
UPDATE trip_invitations 
SET inviter_id = u.id 
FROM users u 
WHERE trip_invitations.inviter_id = u.supabase_id::uuid;

-- Revert trip_invitations.invitee_id to use internal UUIDs
UPDATE trip_invitations 
SET invitee_id = u.id 
FROM users u 
WHERE trip_invitations.invitee_id = u.supabase_id::uuid;

-- Revert todos.created_by to use internal UUIDs
UPDATE todos 
SET created_by = u.id 
FROM users u 
WHERE todos.created_by = u.supabase_id::uuid;

-- Revert notifications.user_id to use internal UUIDs
UPDATE notifications 
SET user_id = u.id 
FROM users u 
WHERE notifications.user_id = u.supabase_id::uuid;

-- Revert chat_groups.created_by to use internal UUIDs
UPDATE chat_groups 
SET created_by = u.id 
FROM users u 
WHERE chat_groups.created_by = u.supabase_id::uuid;

-- Revert chat_messages.user_id to use internal UUIDs
UPDATE chat_messages 
SET user_id = u.id 
FROM users u 
WHERE chat_messages.user_id = u.supabase_id::uuid;

-- Revert chat_group_members.user_id to use internal UUIDs
UPDATE chat_group_members 
SET user_id = u.id 
FROM users u 
WHERE chat_group_members.user_id = u.supabase_id::uuid;

-- Revert chat_message_reactions.user_id to use internal UUIDs
UPDATE chat_message_reactions 
SET user_id = u.id 
FROM users u 
WHERE chat_message_reactions.user_id = u.supabase_id::uuid;

-- Step 3: Add back foreign key constraints pointing to public.users
DO $$
BEGIN
    -- Add foreign key constraints pointing to public.users
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trips_created_by_fkey') THEN
        ALTER TABLE trips ADD CONSTRAINT trips_created_by_fkey 
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'expenses_user_id_fkey') THEN
        ALTER TABLE expenses ADD CONSTRAINT expenses_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'locations_user_id_fkey') THEN
        ALTER TABLE locations ADD CONSTRAINT locations_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_memberships_user_id_fkey') THEN
        ALTER TABLE trip_memberships ADD CONSTRAINT trip_memberships_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_inviter_id_fkey') THEN
        ALTER TABLE trip_invitations ADD CONSTRAINT trip_invitations_inviter_id_fkey 
        FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'trip_invitations_invitee_id_fkey') THEN
        ALTER TABLE trip_invitations ADD CONSTRAINT trip_invitations_invitee_id_fkey 
        FOREIGN KEY (invitee_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'todos_created_by_fkey') THEN
        ALTER TABLE todos ADD CONSTRAINT todos_created_by_fkey 
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'notifications_user_id_fkey') THEN
        ALTER TABLE notifications ADD CONSTRAINT notifications_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_groups_created_by_fkey') THEN
        ALTER TABLE chat_groups ADD CONSTRAINT chat_groups_created_by_fkey 
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_messages_user_id_fkey') THEN
        ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_group_members_user_id_fkey') THEN
        ALTER TABLE chat_group_members ADD CONSTRAINT chat_group_members_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_message_reactions_user_id_fkey') THEN
        ALTER TABLE chat_message_reactions ADD CONSTRAINT chat_message_reactions_user_id_fkey 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;

-- Step 4: Clear auth.users table (optional - only if it was populated by the up migration)
-- DELETE FROM auth.users WHERE id IN (SELECT supabase_id::uuid FROM users WHERE supabase_id IS NOT NULL); 