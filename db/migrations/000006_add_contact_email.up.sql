-- Add contact_email column for searchable user email
-- This allows Apple Sign-In users (with private relay emails) to provide
-- a real email address that friends can use to find and invite them.

ALTER TABLE user_profiles ADD COLUMN contact_email TEXT;

-- Index for efficient email lookups during user search
CREATE INDEX idx_user_profiles_contact_email ON user_profiles(contact_email);

-- Add comment explaining the column's purpose
COMMENT ON COLUMN user_profiles.contact_email IS 'User-provided discoverable email for invitations (distinct from auth email which may be Apple private relay)';
