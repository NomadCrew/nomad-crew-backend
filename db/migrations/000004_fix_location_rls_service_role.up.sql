-- Fix Location RLS Policy for Service Role
-- This migration adds a service role policy to allow the backend service key
-- to insert location records on behalf of users, bypassing the auth.uid() requirement

-- Add service role policy for location inserts
-- This allows the service role (backend) to insert locations for any user
CREATE POLICY "Service role can insert locations for users"
ON locations FOR INSERT
TO service_role
WITH CHECK (true);

-- Add service role policy for location updates  
-- This allows the service role (backend) to update locations for any user
CREATE POLICY "Service role can update locations for users"
ON locations FOR UPDATE
TO service_role
USING (true)
WITH CHECK (true);

-- Add service role policy for location selects
-- This allows the service role (backend) to read all locations
CREATE POLICY "Service role can select all locations"
ON locations FOR SELECT
TO service_role
USING (true);

-- Add service role policy for location deletes
-- This allows the service role (backend) to delete any location
CREATE POLICY "Service role can delete locations"
ON locations FOR DELETE
TO service_role
USING (true); 