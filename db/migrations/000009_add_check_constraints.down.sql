-- Remove CHECK constraints for date validation
ALTER TABLE trips DROP CONSTRAINT IF EXISTS chk_trip_dates;
ALTER TABLE trip_invitations DROP CONSTRAINT IF EXISTS chk_invitation_expiry;
ALTER TABLE polls DROP CONSTRAINT IF EXISTS chk_poll_expiry;
