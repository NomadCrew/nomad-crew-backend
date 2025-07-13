-- Migration: Add performance indexes
-- Purpose: Improve query performance for common operations
-- Date: 2025-07-13

-- Index for listing trips by status and creation date
-- Used in: List trips endpoints, dashboard queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trips_status_created 
ON trips(status, created_at DESC);

-- Composite index for user's active memberships
-- Used in: Get user's trips, membership checks
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trip_memberships_user_status 
ON trip_memberships(user_id, status) 
WHERE status = 'ACTIVE';

-- Index for unread notifications queries
-- Used in: Get unread notifications count, notification list
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_user_unread 
ON notifications(user_id, is_read, created_at DESC) 
WHERE is_read = false;

-- Index for paginated chat messages
-- Used in: Get chat messages with pagination
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_group_created 
ON chat_messages(group_id, created_at DESC) 
WHERE deleted_at IS NULL;

-- Partial index for active trips with location data
-- Used in: Location-based trip queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trips_active_location 
ON trips(destination_latitude, destination_longitude) 
WHERE status = 'ACTIVE';

-- Index for pending invitations by email
-- Used in: Check pending invitations for a user
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trip_invitations_pending_email 
ON trip_invitations(invitee_email, status) 
WHERE status = 'PENDING';

-- Index for trip members count queries
-- Used in: Get trip member counts
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trip_memberships_trip_active 
ON trip_memberships(trip_id) 
WHERE status = 'ACTIVE';

-- Index for user location updates
-- Used in: Get latest location for users
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_locations_user_updated 
ON user_locations(user_id, updated_at DESC);

-- Index for todo items by trip and status
-- Used in: Get incomplete todos for a trip
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_trip_status 
ON todos(trip_id, status) 
WHERE deleted_at IS NULL;

-- Analyze tables to update statistics after adding indexes
ANALYZE trips;
ANALYZE trip_memberships;
ANALYZE notifications;
ANALYZE chat_messages;
ANALYZE trip_invitations;
ANALYZE user_locations;
ANALYZE todos;