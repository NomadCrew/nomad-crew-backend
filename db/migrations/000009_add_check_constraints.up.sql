-- Add CHECK constraints for date validation at the database level
ALTER TABLE trips ADD CONSTRAINT chk_trip_dates CHECK (end_date >= start_date);
ALTER TABLE trip_invitations ADD CONSTRAINT chk_invitation_expiry CHECK (expires_at > created_at);
ALTER TABLE polls ADD CONSTRAINT chk_poll_expiry CHECK (expires_at > created_at);
