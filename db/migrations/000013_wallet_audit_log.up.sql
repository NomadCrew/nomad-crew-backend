-- Wallet Audit Log Migration
-- Append-only table for compliance audit trail of all wallet document access.
-- Note: golang-migrate wraps each migration in a transaction automatically.
-- Do NOT add BEGIN/COMMIT here.

CREATE TABLE IF NOT EXISTS wallet_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    document_id UUID,
    action VARCHAR(20) NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for querying audit trail
CREATE INDEX IF NOT EXISTS idx_wallet_audit_user_created ON wallet_audit_log(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_wallet_audit_document ON wallet_audit_log(document_id);
