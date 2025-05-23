-- Create location privacy enum
CREATE TYPE location_privacy AS ENUM ('hidden', 'approximate', 'precise');

-- Add privacy column to locations table
ALTER TABLE locations ADD COLUMN IF NOT EXISTS 
    privacy location_privacy NOT NULL DEFAULT 'approximate';

-- Add privacy preference column to users table for default preference
ALTER TABLE users ADD COLUMN IF NOT EXISTS 
    location_privacy_preference location_privacy NOT NULL DEFAULT 'approximate';

-- Create function to round coordinates for approximate privacy
CREATE OR REPLACE FUNCTION round_coordinates(
    lat DOUBLE PRECISION, 
    lng DOUBLE PRECISION, 
    privacy location_privacy
) RETURNS TABLE (
    latitude DOUBLE PRECISION, 
    longitude DOUBLE PRECISION
) AS $$
BEGIN
    CASE privacy
        WHEN 'hidden' THEN
            RETURN QUERY SELECT NULL::DOUBLE PRECISION, NULL::DOUBLE PRECISION;
        WHEN 'approximate' THEN
            -- Round to 2 decimal places (~1.1km accuracy)
            RETURN QUERY SELECT ROUND(lat::numeric, 2)::DOUBLE PRECISION, ROUND(lng::numeric, 2)::DOUBLE PRECISION;
        ELSE -- 'precise'
            RETURN QUERY SELECT lat, lng;
    END CASE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Update RLS policy to respect privacy settings
DROP POLICY IF EXISTS "Users can view locations of trip members" ON locations;

CREATE POLICY "Users can view locations of trip members"
ON locations FOR SELECT
TO authenticated
USING (
    -- User can see their own location always
    auth.uid() = user_id OR
    -- User can see others if in same trip, sharing is enabled, and respecting privacy
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