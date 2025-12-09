-- Add push tokens table for storing user device push notification tokens
-- Supports multiple devices per user (phone + tablet, etc.)

CREATE TABLE IF NOT EXISTS user_push_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL,
    device_type VARCHAR(20) NOT NULL CHECK (device_type IN ('ios', 'android')),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,

    -- Each user can have the same token only once (but can have multiple tokens for different devices)
    UNIQUE(user_id, token)
);

-- Index for looking up active tokens by user (most common query)
CREATE INDEX idx_push_tokens_user_active ON user_push_tokens(user_id) WHERE is_active = TRUE;

-- Index for token lookup (for invalidation)
CREATE INDEX idx_push_tokens_token ON user_push_tokens(token);

-- Trigger to update updated_at on changes
CREATE OR REPLACE FUNCTION update_push_token_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_push_token_updated_at
    BEFORE UPDATE ON user_push_tokens
    FOR EACH ROW
    EXECUTE FUNCTION update_push_token_updated_at();
