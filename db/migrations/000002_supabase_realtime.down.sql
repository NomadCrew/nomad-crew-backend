-- Revert Supabase Realtime Migration

-- Drop all related policies first
DROP POLICY IF EXISTS "Users can view messages in their trips" ON supabase_chat_messages;
DROP POLICY IF EXISTS "Users can insert their own messages" ON supabase_chat_messages;
DROP POLICY IF EXISTS "Users can update their own messages" ON supabase_chat_messages;
DROP POLICY IF EXISTS "Users can soft delete their own messages" ON supabase_chat_messages;

DROP POLICY IF EXISTS "Users can view reactions in their trips" ON supabase_chat_reactions;
DROP POLICY IF EXISTS "Users can add their own reactions" ON supabase_chat_reactions;
DROP POLICY IF EXISTS "Users can remove their own reactions" ON supabase_chat_reactions;

DROP POLICY IF EXISTS "Users can view their own read receipts" ON supabase_chat_read_receipts;
DROP POLICY IF EXISTS "Users can update their own read receipts" ON supabase_chat_read_receipts;

DROP POLICY IF EXISTS "Users can view presence in their trips" ON supabase_user_presence;
DROP POLICY IF EXISTS "Users can update their own presence" ON supabase_user_presence;

DROP POLICY IF EXISTS "Users can view locations of trip members" ON locations;
DROP POLICY IF EXISTS "Users can update their own location" ON locations;
DROP POLICY IF EXISTS "Users can insert their own location" ON locations;

-- Disable RLS
ALTER TABLE supabase_chat_messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_chat_reactions DISABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_chat_read_receipts DISABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_user_presence DISABLE ROW LEVEL SECURITY;
ALTER TABLE locations DISABLE ROW LEVEL SECURITY;

-- Drop new tables in reverse order of creation
DROP TABLE IF EXISTS supabase_user_presence;
DROP TABLE IF EXISTS supabase_chat_read_receipts;
DROP TABLE IF EXISTS supabase_chat_reactions;
DROP TABLE IF EXISTS supabase_chat_messages;

-- Revert locations table changes
ALTER TABLE locations DROP COLUMN IF EXISTS is_sharing_enabled;
ALTER TABLE locations DROP COLUMN IF EXISTS sharing_expires_at; 