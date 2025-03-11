-- Drop all tables in reverse order
DROP TABLE IF EXISTS trip_invitations CASCADE;
DROP TABLE IF EXISTS trip_todos CASCADE;
DROP TABLE IF EXISTS trip_memberships CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS locations CASCADE;
DROP TABLE IF EXISTS expenses CASCADE;
DROP TABLE IF EXISTS trips CASCADE;
DROP TABLE IF EXISTS metadata CASCADE;

-- Drop the update_updated_at_column function
DROP FUNCTION IF EXISTS update_updated_at_column CASCADE; 