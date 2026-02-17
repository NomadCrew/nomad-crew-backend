-- Wallet Documents Feature Migration
-- Follows existing patterns: UUID PKs, TIMESTAMPTZ, soft deletes, CASCADE
-- Note: golang-migrate wraps each migration in a transaction automatically.
-- Do NOT add BEGIN/COMMIT here.

-- Ensure the updated_at trigger function exists.
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW IS DISTINCT FROM OLD THEN
        NEW.updated_at = CURRENT_TIMESTAMP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enum for wallet type
DO $$ BEGIN
    CREATE TYPE wallet_type AS ENUM ('personal', 'group');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Enum for document type
DO $$ BEGIN
    CREATE TYPE document_type AS ENUM (
        'passport', 'visa', 'insurance', 'vaccination',
        'loyalty_card', 'flight_booking', 'hotel_booking',
        'reservation', 'receipt', 'other'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Wallet documents table
CREATE TABLE IF NOT EXISTS wallet_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    trip_id UUID REFERENCES trips(id) ON DELETE CASCADE,
    wallet_type wallet_type NOT NULL,
    document_type document_type NOT NULL DEFAULT 'other',
    name VARCHAR(255) NOT NULL,
    description TEXT,
    file_path VARCHAR(512) NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    mime_type VARCHAR(100) NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT wallet_trip_check CHECK (
        (wallet_type = 'personal' AND trip_id IS NULL) OR
        (wallet_type = 'group' AND trip_id IS NOT NULL)
    )
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_wallet_docs_user ON wallet_documents(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_wallet_docs_trip ON wallet_documents(trip_id) WHERE deleted_at IS NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_wallet_documents_updated_at ON wallet_documents;
CREATE TRIGGER update_wallet_documents_updated_at
    BEFORE UPDATE ON wallet_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
