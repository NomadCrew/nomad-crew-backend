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

	// Validate new destination fields (coordinates are mandatory)
	if trip.DestinationLatitude == 0 && trip.DestinationLongitude == 0 {
		validationErrors = append(validationErrors, "trip destination coordinates (latitude and longitude) are required")
	}
	// Optional: Validate ranges for latitude and longitude if needed
	// if trip.DestinationLatitude < -90 || trip.DestinationLatitude > 90 {
	// 	validationErrors = append(validationErrors, "invalid latitude value")
	// }
	// if trip.DestinationLongitude < -180 || trip.DestinationLongitude > 180 {
	// 	validationErrors = append(validationErrors, "invalid longitude value")
	// }

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

	if trip.CreatedBy == nil || *trip.CreatedBy == "" { // Updated for pointer
		validationErrors = append(validationErrors, "trip creator ID is required")
	}

	if trip.Status == "" { // This check might need adjustment if Status can be empty string from DB (unlikely for ENUM)
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
		return errors.ValidationFailed("name_required", "trip name is required")
	}

	// Validate new destination fields (coordinates are mandatory)
	if trip.DestinationLatitude == 0 && trip.DestinationLongitude == 0 {
		return errors.ValidationFailed("destination_coordinates_required", "trip destination coordinates (latitude and longitude) are required")
	}
	// Optional: Validate ranges for latitude and longitude if needed

	// Check if start date is in the past (compare dates only, not time)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tripStartDay := time.Date(trip.StartDate.Year(), trip.StartDate.Month(), trip.StartDate.Day(), 0, 0, 0, 0, trip.StartDate.Location())

	if tripStartDay.Before(today) {
		return errors.ValidationFailed("invalid_start_date", "start date cannot be in the past")
	}

	if trip.EndDate.Before(trip.StartDate) {
		return errors.ValidationFailed("invalid_end_date", "trip end date cannot be before start date")
	}
	if trip.CreatedBy == nil || *trip.CreatedBy == "" { // Updated for pointer
		return errors.ValidationFailed("creator_required", "trip creator ID is required")
	}
	if trip.Status != "" && !trip.Status.IsValid() { // This check might need adjustment
		return errors.ValidationFailed("invalid_status", "invalid trip status")
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
	if update.Status != nil && *update.Status != "" { // Check pointer and its value
		if err := ValidateStatusTransition(originalTrip, *update.Status); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
	}

	// Validate new destination fields (if one coordinate is provided, the other must also be)
	if (update.DestinationLatitude != nil && update.DestinationLongitude == nil) ||
		(update.DestinationLatitude == nil && update.DestinationLongitude != nil) {
		validationErrors = append(validationErrors, "both latitude and longitude must be provided if one is updated")
	}
	// Optional: Add range validation for lat/lon if values are present

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
