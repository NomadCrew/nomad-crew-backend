-- Drop trigger
DROP TRIGGER IF EXISTS update_wallet_documents_updated_at ON wallet_documents;

-- Drop table
DROP TABLE IF EXISTS wallet_documents;

-- Drop enums
DROP TYPE IF EXISTS document_type;
DROP TYPE IF EXISTS wallet_type;
