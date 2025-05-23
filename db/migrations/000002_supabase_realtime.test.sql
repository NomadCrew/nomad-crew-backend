-- Ensure pgcrypto extension is available for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Supabase Realtime Test Migration
-- This is a modified version of 000002_supabase_realtime.up.sql for tests
-- The main change is replacing auth.users references with users table references

-- Create the users table if it doesn't exist (assuming it's created by other migrations in production)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    profile_picture_url VARCHAR(255),
    supabase_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(email),
    UNIQUE(supabase_id)
);

-- 1. Create new chat_messages table
CREATE TABLE IF NOT EXISTS supabase_chat_messages (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id), -- Changed from auth.users
    message TEXT NOT NULL,
    reply_to_id UUID REFERENCES supabase_chat_messages(id),
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

-- Indexes for performance
CREATE INDEX idx_supabase_chat_messages_trip_created ON supabase_chat_messages(trip_id, created_at DESC);
CREATE INDEX idx_supabase_chat_messages_user ON supabase_chat_messages(user_id);
CREATE INDEX idx_supabase_chat_messages_reply ON supabase_chat_messages(reply_to_id) WHERE reply_to_id IS NOT NULL;

-- 2. Chat Reactions Table
CREATE TABLE IF NOT EXISTS supabase_chat_reactions (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES supabase_chat_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id), -- Changed from auth.users
    emoji VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Ensure one reaction per user per emoji per message
    UNIQUE(message_id, user_id, emoji)
);

CREATE INDEX idx_supabase_reactions_message ON supabase_chat_reactions(message_id);

-- 3. Read Receipts Table
CREATE TABLE IF NOT EXISTS supabase_chat_read_receipts (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id), -- Changed from auth.users
    last_read_message_id UUID REFERENCES supabase_chat_messages(id),
    read_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- One receipt per user per trip
    UNIQUE(trip_id, user_id)
);

CREATE INDEX idx_supabase_read_receipts_trip_user ON supabase_chat_read_receipts(trip_id, user_id);

-- 4. User Presence Table
CREATE TABLE IF NOT EXISTS supabase_user_presence (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id), -- Changed from auth.users
    trip_id UUID REFERENCES trips(id),
    status VARCHAR(20) DEFAULT 'online',
    last_seen TIMESTAMPTZ DEFAULT NOW(),
    is_typing BOOLEAN DEFAULT FALSE,
    typing_in_trip_id UUID REFERENCES trips(id),
    
    UNIQUE(user_id, trip_id)
);

CREATE INDEX idx_supabase_presence_trip ON supabase_user_presence(trip_id) WHERE trip_id IS NOT NULL;
CREATE INDEX idx_supabase_presence_last_seen ON supabase_user_presence(last_seen);

-- 5. Locations Table Updates
ALTER TABLE locations ADD COLUMN IF NOT EXISTS 
    is_sharing_enabled BOOLEAN DEFAULT FALSE;

ALTER TABLE locations ADD COLUMN IF NOT EXISTS 
    sharing_expires_at TIMESTAMPTZ;

-- No RLS policies in test environment since PostgreSQL doesn't support them without extensions
-- All RLS commands have been removed 