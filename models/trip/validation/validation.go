package validation

import (
    "strings"
    "time"

    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/types"
)

func ValidateTrip(trip *types.Trip) error {
    var validationErrors []string
    now := time.Now().UTC()

    if trip.Name == "" {
        validationErrors = append(validationErrors, "trip name is required")
    }

    if trip.Destination.Address == "" {
        validationErrors = append(validationErrors, "trip destination is required")
    }

    if trip.StartDate.IsZero() {
        validationErrors = append(validationErrors, "trip start date is required")
    }

    if trip.EndDate.IsZero() {
        validationErrors = append(validationErrors, "trip end date is required")
    }

    // Only validate start date being in past for new trips (where ID is empty)
    if trip.ID == "" && !trip.StartDate.IsZero() {
        startDate := trip.StartDate.Truncate(24 * time.Hour)
        if startDate.Before(now) {
            validationErrors = append(validationErrors, "start date cannot be in the past")
        }
    }

    if !trip.StartDate.IsZero() && !trip.EndDate.IsZero() && trip.EndDate.Before(trip.StartDate) {
        validationErrors = append(validationErrors, "trip end date cannot be before start date")
    }

    if trip.CreatedBy == "" {
        validationErrors = append(validationErrors, "trip creator ID is required")
    }

    if trip.Status == "" {
        trip.Status = types.TripStatusPlanning
    } else if !trip.Status.IsValid() {
        validationErrors = append(validationErrors, "invalid trip status")
    }

    if len(validationErrors) > 0 {
        return errors.ValidationFailed(
            "Invalid trip data",
            strings.Join(validationErrors, "; "),
        )
    }
    return nil
}

func ValidateNewTrip(trip *types.Trip) error {
	if trip.Name == "" {
		return errors.ValidationFailed("name_required", "Trip name is required")
	}
	if trip.StartDate.Before(time.Now().AddDate(0, 0, -1)) {
		return errors.ValidationFailed("invalid_start_date", "Start date must be in the future")
	}
	if trip.EndDate.Before(trip.StartDate) {
		return errors.ValidationFailed("invalid_end_date", "End date must be after start date")
	}
	return nil
}

func ValidateTripUpdate(update *types.TripUpdate, originalTrip *types.Trip) error {
    var validationErrors []string

    // Date consistency checks
    if update.StartDate != nil {
        effectiveEnd := originalTrip.EndDate
        if update.EndDate != nil {
            effectiveEnd = *update.EndDate
        }
        if update.StartDate.After(effectiveEnd) {
            validationErrors = append(validationErrors, "start date cannot be after end date")
        }
    }

    if update.EndDate != nil {
        effectiveStart := originalTrip.StartDate
        if update.StartDate != nil {
            effectiveStart = *update.StartDate
        }
        if update.EndDate.Before(effectiveStart) {
            validationErrors = append(validationErrors, "end date cannot be before start date")
        }
    }

    // Status transition validation with date checks
    if update.Status != "" {
        if err := ValidateStatusTransition(originalTrip, update.Status); err != nil {
            validationErrors = append(validationErrors, err.Error())
        }
    }

    // Null field handling validation
    if update.Destination != nil && update.Destination.Address == "" {
        validationErrors = append(validationErrors, "destination address cannot be empty")
    }

    if len(validationErrors) > 0 {
        return errors.ValidationFailed(
            "invalid_trip_update",
            strings.Join(validationErrors, ", "),
        )
    }
    return nil
}

func ValidateStatusTransition(trip *types.Trip, newStatus types.TripStatus) error {
    if !trip.Status.IsValidTransition(newStatus) {
        return errors.InvalidStatusTransition(
            trip.Status.String(),
            newStatus.String(),
        )
    }

    // Validate time-based constraints
    switch newStatus {
    case types.TripStatusActive:
        if trip.EndDate.Before(time.Now()) {
            return errors.ValidationFailed(
                "invalid status transition",
                "cannot activate a trip that has already ended",
            )
        }
    case types.TripStatusCompleted:
        if trip.EndDate.After(time.Now()) {
            return errors.ValidationFailed(
                "invalid status transition",
                "cannot complete a trip before its end date",
            )
        }
    }

    return nil
}

func ValidateInvitation(invitation *types.TripInvitation) error {
    var validationErrors []string

    if invitation.TripID == "" {
        validationErrors = append(validationErrors, "trip ID is required")
    }

    if invitation.InviterID == "" {
        validationErrors = append(validationErrors, "inviter ID is required")
    }

    if invitation.InviteeEmail == "" {
        validationErrors = append(validationErrors, "invitee email is required")
    }

    // Validate email format
    if !strings.Contains(invitation.InviteeEmail, "@") {
        validationErrors = append(validationErrors, "invalid email format")
    }

    if invitation.ExpiresAt.IsZero() {
        validationErrors = append(validationErrors, "expiration time is required")
    }

    if invitation.ExpiresAt.Before(time.Now()) {
        validationErrors = append(validationErrors, "expiration time cannot be in the past")
    }

    if len(validationErrors) > 0 {
        return errors.ValidationFailed(
            "Invalid invitation data",
            strings.Join(validationErrors, "; "),
        )
    }

    return nil
}