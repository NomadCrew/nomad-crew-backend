-- Restore the original RLS policy
DROP POLICY IF EXISTS "Users can view locations of trip members" ON locations;
CREATE POLICY "Users can view locations of trip members"
ON locations FOR SELECT
TO authenticated
USING (
    -- User can see their own location always
    auth.uid() = user_id OR
    -- User can see others if in same trip and sharing is enabled
    EXISTS (
        SELECT 1 FROM trip_memberships tm1
        JOIN trip_memberships tm2 ON tm1.trip_id = tm2.trip_id
        WHERE tm1.user_id = auth.uid()
        AND tm2.user_id = locations.user_id
        AND tm1.deleted_at IS NULL
        AND tm2.deleted_at IS NULL
        AND locations.is_sharing_enabled = TRUE
        AND (locations.sharing_expires_at IS NULL OR locations.sharing_expires_at > NOW())
    )
);

-- Drop the function
DROP FUNCTION IF EXISTS round_coordinates;

-- Remove the columns
ALTER TABLE users DROP COLUMN IF EXISTS location_privacy_preference;
ALTER TABLE locations DROP COLUMN IF EXISTS privacy;

-- Drop the enum type (must be done after all columns using it are dropped)
DROP TYPE IF EXISTS location_privacy; 