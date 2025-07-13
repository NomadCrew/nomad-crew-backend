-- Consolidated Schema Rollback Migration
-- This migration reverses all changes from the consolidated schema migration
-- Rollback for: 000001_init, 000002_supabase_realtime, 000003_location_privacy, 
--              000004_fix_location_rls_service_role, 000005_fix_user_references_to_auth_schema

-- Remove schema comment
COMMENT ON SCHEMA public IS NULL;

-- Drop all RLS policies first
DROP POLICY IF EXISTS "Service role can delete locations" ON locations;
DROP POLICY IF EXISTS "Service role can select all locations" ON locations;
DROP POLICY IF EXISTS "Service role can update locations for users" ON locations;
DROP POLICY IF EXISTS "Service role can insert locations for users" ON locations;

DROP POLICY IF EXISTS "Users can insert their own location" ON locations;
DROP POLICY IF EXISTS "Users can update their own location" ON locations;
DROP POLICY IF EXISTS "Users can view locations of trip members" ON locations;

DROP POLICY IF EXISTS "Users can update their own presence" ON supabase_user_presence;
DROP POLICY IF EXISTS "Users can insert their own presence" ON supabase_user_presence;
DROP POLICY IF EXISTS "Users can view presence in their trips" ON supabase_user_presence;

DROP POLICY IF EXISTS "Users can update their own read receipts" ON supabase_chat_read_receipts;
DROP POLICY IF EXISTS "Users can insert their own read receipts" ON supabase_chat_read_receipts;
DROP POLICY IF EXISTS "Users can view their own read receipts" ON supabase_chat_read_receipts;

DROP POLICY IF EXISTS "Users can remove their own reactions" ON supabase_chat_reactions;
DROP POLICY IF EXISTS "Users can add their own reactions" ON supabase_chat_reactions;
DROP POLICY IF EXISTS "Users can view reactions in their trips" ON supabase_chat_reactions;

DROP POLICY IF EXISTS "Users can soft delete their own messages" ON supabase_chat_messages;
DROP POLICY IF EXISTS "Users can update their own messages" ON supabase_chat_messages;
DROP POLICY IF EXISTS "Users can insert their own messages" ON supabase_chat_messages;
DROP POLICY IF EXISTS "Users can view messages in their trips" ON supabase_chat_messages;

-- Disable RLS
ALTER TABLE IF EXISTS locations DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS supabase_user_presence DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS supabase_chat_read_receipts DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS supabase_chat_reactions DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS supabase_chat_messages DISABLE ROW LEVEL SECURITY;

-- Drop all indexes
DROP INDEX IF EXISTS idx_supabase_presence_last_seen;
DROP INDEX IF EXISTS idx_supabase_presence_trip;
DROP INDEX IF EXISTS idx_supabase_read_receipts_trip_user;
DROP INDEX IF EXISTS idx_supabase_reactions_message;
DROP INDEX IF EXISTS idx_supabase_chat_messages_reply;
DROP INDEX IF EXISTS idx_supabase_chat_messages_user;
DROP INDEX IF EXISTS idx_supabase_chat_messages_trip_created;

DROP INDEX IF EXISTS idx_chat_message_reactions_user_id;
DROP INDEX IF EXISTS idx_chat_message_reactions_message_id;
DROP INDEX IF EXISTS idx_chat_messages_deleted_at;
DROP INDEX IF EXISTS idx_chat_messages_content_type;
DROP INDEX IF EXISTS idx_chat_messages_user_id;
DROP INDEX IF EXISTS idx_chat_messages_group_id_created_at;
DROP INDEX IF EXISTS idx_chat_group_members_user_id;
DROP INDEX IF EXISTS idx_chat_groups_deleted_at;
DROP INDEX IF EXISTS idx_chat_groups_created_by;
DROP INDEX IF EXISTS idx_chat_groups_trip_id;

DROP INDEX IF EXISTS idx_notifications_type;
DROP INDEX IF EXISTS idx_notifications_user_id_is_read;
DROP INDEX IF EXISTS idx_notifications_user_id_created_at;

DROP INDEX IF EXISTS idx_todos_status;
DROP INDEX IF EXISTS idx_todos_created_by;
DROP INDEX IF EXISTS idx_todos_trip_id;

DROP INDEX IF EXISTS idx_trip_invitations_deleted_at;
DROP INDEX IF EXISTS idx_trip_invitations_invitee_id;
DROP INDEX IF EXISTS idx_trip_invitations_invitee_email;
DROP INDEX IF EXISTS idx_trip_invitations_trip_id;

DROP INDEX IF EXISTS idx_trip_memberships_deleted_at;
DROP INDEX IF EXISTS idx_trip_memberships_user;
DROP INDEX IF EXISTS idx_trip_memberships_trip_user;

DROP INDEX IF EXISTS idx_locations_deleted_at;
DROP INDEX IF EXISTS idx_locations_timestamp;
DROP INDEX IF EXISTS idx_locations_trip_id;
DROP INDEX IF EXISTS idx_locations_user_id;

DROP INDEX IF EXISTS idx_expenses_deleted_at;
DROP INDEX IF EXISTS idx_expenses_trip_id;
DROP INDEX IF EXISTS idx_expenses_user_id;

DROP INDEX IF EXISTS idx_trips_deleted_at;
DROP INDEX IF EXISTS idx_trips_destination_coordinates;
DROP INDEX IF EXISTS idx_trips_destination_place_id;
DROP INDEX IF EXISTS idx_trips_status;
DROP INDEX IF EXISTS idx_trips_created_by;

DROP INDEX IF EXISTS idx_users_email;

-- Drop all triggers
DROP TRIGGER IF EXISTS update_chat_messages_updated_at ON chat_messages;
DROP TRIGGER IF EXISTS update_chat_groups_updated_at ON chat_groups;
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
DROP TRIGGER IF EXISTS update_todos_updated_at ON todos;
DROP TRIGGER IF EXISTS update_trip_invitations_updated_at ON trip_invitations;
DROP TRIGGER IF EXISTS update_trip_memberships_updated_at ON trip_memberships;
DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;
DROP TRIGGER IF EXISTS update_locations_updated_at ON locations;
DROP TRIGGER IF EXISTS update_expenses_updated_at ON expenses;
DROP TRIGGER IF EXISTS update_trips_updated_at ON trips;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop all functions
DROP FUNCTION IF EXISTS round_coordinates CASCADE;
DROP FUNCTION IF EXISTS update_updated_at_column CASCADE;
DROP FUNCTION IF EXISTS auth.uid CASCADE;

-- Drop all tables in reverse order of dependency
DROP TABLE IF EXISTS supabase_user_presence CASCADE;
DROP TABLE IF EXISTS supabase_chat_read_receipts CASCADE;
DROP TABLE IF EXISTS supabase_chat_reactions CASCADE;
DROP TABLE IF EXISTS supabase_chat_messages CASCADE;

DROP TABLE IF EXISTS chat_message_reactions CASCADE;
DROP TABLE IF EXISTS chat_group_members CASCADE;
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS chat_groups CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS todos CASCADE;
DROP TABLE IF EXISTS trip_invitations CASCADE;
DROP TABLE IF EXISTS trip_memberships CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS locations CASCADE;
DROP TABLE IF EXISTS expenses CASCADE;
DROP TABLE IF EXISTS trips CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Drop mock auth schema and its contents
DROP TABLE IF EXISTS auth.users CASCADE;
DROP SCHEMA IF EXISTS auth CASCADE;

-- Drop all ENUM types in reverse order
DROP TYPE IF EXISTS location_privacy CASCADE;
DROP TYPE IF EXISTS invitation_status CASCADE;
DROP TYPE IF EXISTS notification_type CASCADE;
DROP TYPE IF EXISTS todo_status CASCADE;
DROP TYPE IF EXISTS membership_status CASCADE;
DROP TYPE IF EXISTS membership_role CASCADE;
DROP TYPE IF EXISTS trip_status CASCADE;

-- Drop extensions (usually not needed as they might be used by other schemas)
-- DROP EXTENSION IF EXISTS "pgcrypto"; -- Comment out to avoid affecting other schemas
-- DROP EXTENSION IF EXISTS "uuid-ossp"; -- Comment out to avoid affecting other schemas 