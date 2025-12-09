-- Rollback: Remove contact_email column
DROP INDEX IF EXISTS idx_user_profiles_contact_email;
ALTER TABLE user_profiles DROP COLUMN IF EXISTS contact_email;
