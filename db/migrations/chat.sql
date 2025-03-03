-- Chat tables for NomadCrew

-- Create auth schema if it doesn't exist
CREATE SCHEMA IF NOT EXISTS auth;

-- Create auth.users table if it doesn't exist
CREATE TABLE IF NOT EXISTS auth.users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    raw_user_meta_data JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_sign_in_at TIMESTAMP,
    confirmed_at TIMESTAMP,
    email_confirmed_at TIMESTAMP,
    banned_until TIMESTAMP,
    reauthentication_token VARCHAR(255),
    is_sso_user BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP
);

-- Create chat_groups table
CREATE TABLE chat_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create chat_messages table
CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE
);

-- Create chat_group_members table to track who is in which chat group
CREATE TABLE chat_group_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_read_message_id UUID REFERENCES chat_messages(id),
    UNIQUE(group_id, user_id)
);

-- Create chat_message_attachments table for file attachments
CREATE TABLE chat_message_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    file_url VARCHAR(512) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create chat_message_reactions table for emoji reactions
CREATE TABLE chat_message_reactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    reaction VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(message_id, user_id, reaction)
);

-- Add triggers for updated_at columns
CREATE TRIGGER update_chat_groups_updated_at
    BEFORE UPDATE ON chat_groups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_chat_messages_updated_at
    BEFORE UPDATE ON chat_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add indexes for better performance
CREATE INDEX idx_chat_groups_trip_id ON chat_groups(trip_id);
CREATE INDEX idx_chat_messages_group_id ON chat_messages(group_id);
CREATE INDEX idx_chat_messages_user_id ON chat_messages(user_id);
CREATE INDEX idx_chat_messages_created_at ON chat_messages(created_at);
CREATE INDEX idx_chat_group_members_group_id ON chat_group_members(group_id);
CREATE INDEX idx_chat_group_members_user_id ON chat_group_members(user_id);
CREATE INDEX idx_chat_message_attachments_message_id ON chat_message_attachments(message_id);
CREATE INDEX idx_chat_message_reactions_message_id ON chat_message_reactions(message_id);
CREATE INDEX idx_chat_message_reactions_user_id ON chat_message_reactions(user_id);

-- Create a default group chat for each trip
CREATE OR REPLACE FUNCTION create_default_chat_group()
RETURNS TRIGGER AS $$
BEGIN
    -- Create a default chat group for the trip
    INSERT INTO chat_groups (trip_id, name, description, created_by)
    VALUES (NEW.id, 'General', 'General chat for the trip', NEW.created_by);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add trigger to create default chat group when a trip is created
CREATE TRIGGER create_default_chat_group_trigger
    AFTER INSERT ON trips
    FOR EACH ROW
    EXECUTE FUNCTION create_default_chat_group(); 