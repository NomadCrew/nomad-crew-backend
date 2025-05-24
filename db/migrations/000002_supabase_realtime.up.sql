-- Supabase Realtime Migration

-- 1. Create new chat_messages table
CREATE TABLE IF NOT EXISTS supabase_chat_messages (
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

-- Indexes for performance
CREATE INDEX idx_supabase_chat_messages_trip_created ON supabase_chat_messages(trip_id, created_at DESC);
CREATE INDEX idx_supabase_chat_messages_user ON supabase_chat_messages(user_id);
CREATE INDEX idx_supabase_chat_messages_reply ON supabase_chat_messages(reply_to_id) WHERE reply_to_id IS NOT NULL;

-- 2. Chat Reactions Table
CREATE TABLE IF NOT EXISTS supabase_chat_reactions (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES supabase_chat_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id),
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
    user_id UUID NOT NULL REFERENCES auth.users(id),
    last_read_message_id UUID REFERENCES supabase_chat_messages(id),
    read_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- One receipt per user per trip
    UNIQUE(trip_id, user_id)
);

CREATE INDEX idx_supabase_read_receipts_trip_user ON supabase_chat_read_receipts(trip_id, user_id);

-- 4. User Presence Table
CREATE TABLE IF NOT EXISTS supabase_user_presence (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth.users(id),
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

-- Enable RLS
ALTER TABLE supabase_chat_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_chat_reactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_chat_read_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE supabase_user_presence ENABLE ROW LEVEL SECURITY;

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

CREATE POLICY "Users can insert their own presence" ON supabase_user_presence FOR INSERT
ON supabase_user_presence FOR INSERT
TO authenticated
WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update their own presence"
ON supabase_user_presence FOR UPDATE
TO authenticated
USING (auth.uid() = user_id)
WITH CHECK (auth.uid() = user_id);

-- Location Policies
ALTER TABLE locations ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view locations of trip members"
ON locations FOR SELECT
TO authenticated
USING (
    -- User can see their own location always
    auth.uid() = user_id OR
    -- User can see others if in same trip and sharing is enabled
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