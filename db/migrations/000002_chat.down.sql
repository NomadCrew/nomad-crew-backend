-- Drop the trigger first
DROP TRIGGER IF EXISTS create_default_chat_group_trigger ON trips;

-- Drop the function
DROP FUNCTION IF EXISTS create_default_chat_group();

-- Drop chat-related tables in reverse order
DROP TABLE IF EXISTS chat_message_reactions CASCADE;
DROP TABLE IF EXISTS chat_message_attachments CASCADE;
DROP TABLE IF EXISTS chat_group_members CASCADE;
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS chat_groups CASCADE;

-- Drop auth schema and its contents
DROP SCHEMA IF EXISTS auth CASCADE; 