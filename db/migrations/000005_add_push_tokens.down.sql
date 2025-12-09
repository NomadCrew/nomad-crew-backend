-- Rollback push tokens table

DROP TRIGGER IF EXISTS trigger_push_token_updated_at ON user_push_tokens;
DROP FUNCTION IF EXISTS update_push_token_updated_at();
DROP INDEX IF EXISTS idx_push_tokens_token;
DROP INDEX IF EXISTS idx_push_tokens_user_active;
DROP TABLE IF EXISTS user_push_tokens;
