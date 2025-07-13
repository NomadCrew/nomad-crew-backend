-- Rollback: Remove performance indexes
-- Date: 2025-07-13

DROP INDEX CONCURRENTLY IF EXISTS idx_trips_status_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_trip_memberships_user_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_user_unread;
DROP INDEX CONCURRENTLY IF EXISTS idx_chat_messages_group_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_trips_active_location;
DROP INDEX CONCURRENTLY IF EXISTS idx_trip_invitations_pending_email;
DROP INDEX CONCURRENTLY IF EXISTS idx_trip_memberships_trip_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_user_locations_user_updated;
DROP INDEX CONCURRENTLY IF EXISTS idx_todos_trip_status;