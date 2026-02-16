-- Extend notification_type enum with types used by notification service callers
-- These types are referenced in buildPushNotification and CreateAndPublishNotification
-- but were missing from the database enum.

ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'TRIP_INVITATION';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'TRIP_MEMBER_JOINED';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'TRIP_MEMBER_LEFT';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'TRIP_UPDATED';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'CHAT_MESSAGE';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'TODO_ASSIGNED';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'TODO_COMPLETED';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'MEMBER_ADDED';
