-- Supabase Migration: Production with RLS and Real-time Features
-- This migration assumes auth schema already exists in Supabase

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create ENUM types
CREATE TYPE trip_status AS ENUM ('PLANNING', 'ACTIVE', 'COMPLETED', 'CANCELLED');
CREATE TYPE membership_role AS ENUM ('OWNER', 'MEMBER', 'ADMIN');
CREATE TYPE membership_status AS ENUM ('ACTIVE', 'INACTIVE');
CREATE TYPE todo_status AS ENUM ('COMPLETE', 'INCOMPLETE');
CREATE TYPE notification_type AS ENUM (
    'TRIP_INVITATION_RECEIVED',
    'TRIP_INVITATION_ACCEPTED',
    'TRIP_INVITATION_DECLINED',
    'TRIP_UPDATE',
    'NEW_CHAT_MESSAGE',
    'EXPENSE_REPORT_SUBMITTED',
    'TASK_ASSIGNED',
    'TASK_COMPLETED',
    'LOCATION_SHARED',
    'MEMBERSHIP_CHANGE'
);
CREATE TYPE invitation_status AS ENUM ('PENDING', 'ACCEPTED', 'DECLINED');
CREATE TYPE location_privacy AS ENUM ('hidden', 'approximate', 'precise');

-- Create users table (local user table for backward compatibility)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supabase_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL UNIQUE,
    encrypted_password TEXT,
    username TEXT UNIQUE NOT NULL,
    name TEXT,
    first_name TEXT,
    last_name TEXT,
    profile_picture_url TEXT,
    raw_user_meta_data JSONB,
    preferences JSONB,
    location_privacy_preference location_privacy NOT NULL DEFAULT 'approximate',
    last_seen_at TIMESTAMPTZ,
    is_online BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create trips table
CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    destination_place_id TEXT,
    destination_address TEXT,
    destination_name TEXT,
    destination_latitude DOUBLE PRECISION NOT NULL,
    destination_longitude DOUBLE PRECISION NOT NULL,
    status trip_status NOT NULL DEFAULT 'PLANNING',
    created_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    background_image_url VARCHAR(512),
    deleted_at TIMESTAMPTZ NULL
);

-- Create expenses table
CREATE TABLE expenses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    amount DECIMAL(10, 2) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    payment_method VARCHAR(50),
    receipt VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL
);

-- Create locations table
CREATE TABLE locations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    accuracy DOUBLE PRECISION NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    location_name VARCHAR(255),
    location_type VARCHAR(50),
    notes TEXT,
    status VARCHAR(50) DEFAULT 'planned',
    is_sharing_enabled BOOLEAN DEFAULT FALSE,
    sharing_expires_at TIMESTAMPTZ,
    privacy location_privacy NOT NULL DEFAULT 'approximate',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL
);

-- Create categories table
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create trip_memberships table
CREATE TABLE trip_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role membership_role NOT NULL DEFAULT 'MEMBER',
    status membership_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE(trip_id, user_id)
);

-- Create trip_invitations table
CREATE TABLE trip_invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    inviter_id UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    invitee_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    invitee_email TEXT NOT NULL,
    role membership_role NOT NULL DEFAULT 'MEMBER',
    token TEXT,
    status invitation_status NOT NULL DEFAULT 'PENDING',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL
);

-- Create todos table
CREATE TABLE todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    status todo_status NOT NULL DEFAULT 'INCOMPLETE',
    created_by UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create notifications table
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    type notification_type NOT NULL,
    metadata JSONB NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Chat Groups Table
CREATE TABLE chat_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Chat Messages Table
CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
    user_id UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'text',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Chat Group Members Table
CREATE TABLE chat_group_members (
    group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_read_message_id UUID REFERENCES chat_messages(id) ON DELETE SET NULL,
    PRIMARY KEY (group_id, user_id)
);

-- Chat Message Reactions Table
CREATE TABLE chat_message_reactions (
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    reaction VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id, reaction)
);

-- Supabase Realtime Tables
-- 1. Supabase Chat Messages Table
CREATE TABLE supabase_chat_messages (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    message TEXT NOT NULL,
    reply_to_id UUID REFERENCES supabase_chat_messages(id) ON DELETE SET NULL,
    edited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    -- Constraints
    CONSTRAINT valid_message CHECK (
        char_length(message) > 0 AND 
        char_length(message) <= 1000
    ),
    CONSTRAINT valid_timestamps CHECK (
        created_at <= updated_at AND
        (edited_at IS NULL OR edited_at >= created_at) AND
        (deleted_at IS NULL OR deleted_at >= created_at)
    )
);

-- 2. Supabase Chat Reactions Table
CREATE TABLE supabase_chat_reactions (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES supabase_chat_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    emoji VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Ensure one reaction per user per emoji per message
    UNIQUE(message_id, user_id, emoji)
);

-- 3. Supabase Read Receipts Table
CREATE TABLE supabase_chat_read_receipts (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    last_read_message_id UUID REFERENCES supabase_chat_messages(id),
    read_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- One receipt per user per trip
    UNIQUE(trip_id, user_id)
);

-- 4. Supabase User Presence Table
CREATE TABLE supabase_user_presence (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    trip_id UUID REFERENCES trips(id),
    status VARCHAR(20) DEFAULT 'online',
    last_seen TIMESTAMPTZ DEFAULT NOW(),
    is_typing BOOLEAN DEFAULT FALSE,
    typing_in_trip_id UUID REFERENCES trips(id),
    
    UNIQUE(user_id, trip_id)
);

-- Create function for coordinate privacy
CREATE OR REPLACE FUNCTION round_coordinates(
    lat DOUBLE PRECISION, 
    lng DOUBLE PRECISION, 
    privacy location_privacy
) RETURNS TABLE (
    latitude DOUBLE PRECISION, 
    longitude DOUBLE PRECISION
) AS $$
BEGIN
    CASE privacy
        WHEN 'hidden' THEN
            RETURN QUERY SELECT NULL::DOUBLE PRECISION, NULL::DOUBLE PRECISION;
        WHEN 'approximate' THEN
            -- Round to 2 decimal places (~1.1km accuracy)
            RETURN QUERY SELECT ROUND(lat::numeric, 2)::DOUBLE PRECISION, ROUND(lng::numeric, 2)::DOUBLE PRECISION;
        ELSE -- 'precise'
            RETURN QUERY SELECT lat, lng;
    END CASE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Create triggers to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW IS DISTINCT FROM OLD THEN
        NEW.updated_at = CURRENT_TIMESTAMP;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add triggers
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trips_updated_at
    BEFORE UPDATE ON trips
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_expenses_updated_at
    BEFORE UPDATE ON expenses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_locations_updated_at
    BEFORE UPDATE ON locations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_categories_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trip_memberships_updated_at
    BEFORE UPDATE ON trip_memberships
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trip_invitations_updated_at
    BEFORE UPDATE ON trip_invitations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_todos_updated_at
    BEFORE UPDATE ON todos
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_chat_groups_updated_at
    BEFORE UPDATE ON chat_groups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_chat_messages_updated_at
    BEFORE UPDATE ON chat_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add indexes for better performance
CREATE INDEX idx_users_email ON users(email);

CREATE INDEX idx_trips_created_by ON trips(created_by);
CREATE INDEX idx_trips_status ON trips(status);
CREATE INDEX idx_trips_destination_place_id ON trips(destination_place_id);
CREATE INDEX idx_trips_destination_coordinates ON trips(destination_latitude, destination_longitude);
CREATE INDEX idx_trips_deleted_at ON trips(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_expenses_user_id ON expenses(user_id);
CREATE INDEX idx_expenses_trip_id ON expenses(trip_id);
CREATE INDEX idx_expenses_deleted_at ON expenses(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_locations_user_id ON locations(user_id);
CREATE INDEX idx_locations_trip_id ON locations(trip_id);
CREATE INDEX idx_locations_timestamp ON locations("timestamp");
CREATE INDEX idx_locations_deleted_at ON locations(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_trip_memberships_trip_user ON trip_memberships(trip_id, user_id);
CREATE INDEX idx_trip_memberships_user ON trip_memberships(user_id);
CREATE INDEX idx_trip_memberships_deleted_at ON trip_memberships(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_trip_invitations_trip_id ON trip_invitations(trip_id);
CREATE INDEX idx_trip_invitations_invitee_email ON trip_invitations(invitee_email);
CREATE INDEX idx_trip_invitations_invitee_id ON trip_invitations(invitee_id);
CREATE INDEX idx_trip_invitations_deleted_at ON trip_invitations(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_todos_trip_id ON todos(trip_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_created_by ON todos(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_status ON todos(status) WHERE deleted_at IS NULL;

CREATE INDEX idx_notifications_user_id_created_at ON notifications (user_id, created_at DESC);
CREATE INDEX idx_notifications_user_id_is_read ON notifications (user_id, is_read);
CREATE INDEX idx_notifications_type ON notifications(type);

CREATE INDEX idx_chat_groups_trip_id ON chat_groups(trip_id);
CREATE INDEX idx_chat_groups_created_by ON chat_groups(created_by);
CREATE INDEX idx_chat_groups_deleted_at ON chat_groups(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_chat_group_members_user_id ON chat_group_members(user_id);

CREATE INDEX idx_chat_messages_group_id_created_at ON chat_messages(group_id, created_at DESC);
CREATE INDEX idx_chat_messages_user_id ON chat_messages(user_id);
CREATE INDEX idx_chat_messages_content_type ON chat_messages(content_type);
CREATE INDEX idx_chat_messages_deleted_at ON chat_messages(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_chat_message_reactions_message_id ON chat_message_reactions(message_id);
CREATE INDEX idx_chat_message_reactions_user_id ON chat_message_reactions(user_id);

-- Supabase indexes
CREATE INDEX idx_supabase_chat_messages_trip_created ON supabase_chat_messages(trip_id, created_at DESC);
CREATE INDEX idx_supabase_chat_messages_user ON supabase_chat_messages(user_id);
CREATE INDEX idx_supabase_chat_messages_reply ON supabase_chat_messages(reply_to_id) WHERE reply_to_id IS NOT NULL;

CREATE INDEX idx_supabase_reactions_message ON supabase_chat_reactions(message_id);

CREATE INDEX idx_supabase_read_receipts_trip_user ON supabase_chat_read_receipts(trip_id, user_id);

CREATE INDEX idx_supabase_presence_trip ON supabase_user_presence(trip_id) WHERE trip_id IS NOT NULL;
CREATE INDEX idx_supabase_presence_last_seen ON supabase_user_presence(last_seen);

-- Enable RLS on Supabase tables
ALTER TABLE supabase_chat_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_chat_reactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_chat_read_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_user_presence ENABLE ROW LEVEL SECURITY;
ALTER TABLE locations ENABLE ROW LEVEL SECURITY;

-- Chat Messages Policies
CREATE POLICY "Users can view messages in their trips"
ON supabase_chat_messages FOR SELECT
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM trip_memberships tm
        WHERE tm.trip_id = supabase_chat_messages.trip_id 
        AND tm.user_id = auth.uid()
        AND tm.deleted_at IS NULL
    )
);

CREATE POLICY "Users can insert their own messages"
ON supabase_chat_messages FOR INSERT
TO authenticated
WITH CHECK (
    auth.uid() = user_id AND
    EXISTS (
        SELECT 1 FROM trip_memberships tm
        WHERE tm.trip_id = supabase_chat_messages.trip_id 
        AND tm.user_id = auth.uid()
        AND tm.deleted_at IS NULL
    )
);

CREATE POLICY "Users can update their own messages"
ON supabase_chat_messages FOR UPDATE
TO authenticated
USING (auth.uid() = user_id)
WITH CHECK (
    auth.uid() = user_id AND 
    deleted_at IS NULL AND
    created_at > NOW() - INTERVAL '15 minutes'
);

CREATE POLICY "Users can soft delete their own messages"
ON supabase_chat_messages FOR UPDATE
TO authenticated
USING (auth.uid() = user_id)
WITH CHECK (
    auth.uid() = user_id AND
    deleted_at IS NOT NULL
);

-- Reactions Policies
CREATE POLICY "Users can view reactions in their trips"
ON supabase_chat_reactions FOR SELECT
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM supabase_chat_messages cm
        JOIN trip_memberships tm ON tm.trip_id = cm.trip_id
        WHERE cm.id = supabase_chat_reactions.message_id
        AND tm.user_id = auth.uid()
        AND tm.deleted_at IS NULL
    )
);

CREATE POLICY "Users can add their own reactions"
ON supabase_chat_reactions FOR INSERT
TO authenticated
WITH CHECK (
    auth.uid() = user_id AND
    EXISTS (
        SELECT 1 FROM supabase_chat_messages cm
        JOIN trip_memberships tm ON tm.trip_id = cm.trip_id
        WHERE cm.id = supabase_chat_reactions.message_id
        AND tm.user_id = auth.uid()
        AND tm.deleted_at IS NULL
    )
);

CREATE POLICY "Users can remove their own reactions"
ON supabase_chat_reactions FOR DELETE
TO authenticated
USING (auth.uid() = user_id);

-- Read Receipts Policies
CREATE POLICY "Users can view their own read receipts"
ON supabase_chat_read_receipts FOR SELECT
TO authenticated
USING (auth.uid() = user_id);

CREATE POLICY "Users can insert their own read receipts"
ON supabase_chat_read_receipts FOR INSERT
TO authenticated
WITH CHECK (
    auth.uid() = user_id AND
    EXISTS (
        SELECT 1 FROM trip_memberships tm
        WHERE tm.trip_id = supabase_chat_read_receipts.trip_id 
        AND tm.user_id = auth.uid()
        AND tm.deleted_at IS NULL
    )
);

CREATE POLICY "Users can update their own read receipts"
ON supabase_chat_read_receipts FOR UPDATE
TO authenticated
USING (auth.uid() = user_id)
WITH CHECK (auth.uid() = user_id);

-- Presence Policies
CREATE POLICY "Users can view presence in their trips"
ON supabase_user_presence FOR SELECT
TO authenticated
USING (
    auth.uid() = user_id OR
    EXISTS (
        SELECT 1 FROM trip_memberships tm
        WHERE tm.trip_id = supabase_user_presence.trip_id
        AND tm.user_id = auth.uid()
        AND tm.deleted_at IS NULL
    )
);

CREATE POLICY "Users can insert their own presence"
ON supabase_user_presence FOR INSERT
TO authenticated
WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update their own presence"
ON supabase_user_presence FOR UPDATE
TO authenticated
USING (auth.uid() = user_id)
WITH CHECK (auth.uid() = user_id);

-- Location Policies with Privacy Support
CREATE POLICY "Users can view locations of trip members"
ON locations FOR SELECT
TO authenticated
USING (
    -- User can see their own location always
    auth.uid() = user_id OR
    -- User can see others if in same trip, sharing is enabled, and respecting privacy
    EXISTS (
        SELECT 1 FROM trip_memberships tm1
        JOIN trip_memberships tm2 ON tm1.trip_id = tm2.trip_id
        WHERE tm1.user_id = auth.uid()
        AND tm2.user_id = locations.user_id
        AND tm1.deleted_at IS NULL
        AND tm2.deleted_at IS NULL
        AND locations.is_sharing_enabled = TRUE
        AND (locations.sharing_expires_at IS NULL OR locations.sharing_expires_at > NOW())
    )
);

CREATE POLICY "Users can update their own location"
ON locations FOR UPDATE
TO authenticated
USING (auth.uid() = user_id)
WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can insert their own location"
ON locations FOR INSERT
TO authenticated
WITH CHECK (auth.uid() = user_id);

-- Service Role Policies for Backend Operations
CREATE POLICY "Service role can insert locations for users"
ON locations FOR INSERT
TO service_role
WITH CHECK (true);

CREATE POLICY "Service role can update locations for users"
ON locations FOR UPDATE
TO service_role
USING (true)
WITH CHECK (true);

CREATE POLICY "Service role can select all locations"
ON locations FOR SELECT
TO service_role
USING (true);

CREATE POLICY "Service role can delete locations"
ON locations FOR DELETE
TO service_role
USING (true);

-- Add schema comment to track migration
COMMENT ON SCHEMA public IS 'Supabase Production Schema with RLS'; 