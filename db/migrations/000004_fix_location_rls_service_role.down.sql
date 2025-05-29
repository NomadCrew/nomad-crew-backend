-- Revert Location RLS Service Role Policies
-- This migration removes the service role policies for location operations

-- Drop service role policies
DROP POLICY IF EXISTS "Service role can insert locations for users" ON locations;
DROP POLICY IF EXISTS "Service role can update locations for users" ON locations;
DROP POLICY IF EXISTS "Service role can select all locations" ON locations;
DROP POLICY IF EXISTS "Service role can delete locations" ON locations; 