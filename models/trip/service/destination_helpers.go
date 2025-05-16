package service

import (
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// IsDestinationValid checks if a trip's destination has valid coordinates.
// A destination is considered valid if either latitude or longitude is non-zero.
func IsDestinationValid(trip *types.Trip) bool {
	if trip == nil {
		return false
	}
	return trip.DestinationLatitude != 0 || trip.DestinationLongitude != 0
}

// AreDestinationsEqual compares two trips' destinations for equality.
// It checks latitude, longitude, place ID, and address.
func AreDestinationsEqual(trip1 *types.Trip, trip2 *types.Trip) bool {
	if trip1 == nil && trip2 == nil {
		return true // Both nil, considered equal in this context
	}
	if trip1 == nil || trip2 == nil {
		return false // One nil, the other not, considered unequal
	}

	// Compare coordinates
	if trip1.DestinationLatitude != trip2.DestinationLatitude ||
		trip1.DestinationLongitude != trip2.DestinationLongitude {
		return false
	}

	// Compare PlaceID (pointer comparison)
	if (trip1.DestinationPlaceID == nil && trip2.DestinationPlaceID != nil) ||
		(trip1.DestinationPlaceID != nil && trip2.DestinationPlaceID == nil) {
		return false
	}
	if trip1.DestinationPlaceID != nil && trip2.DestinationPlaceID != nil &&
		*trip1.DestinationPlaceID != *trip2.DestinationPlaceID {
		return false
	}

	// Compare Address (pointer comparison)
	if (trip1.DestinationAddress == nil && trip2.DestinationAddress != nil) ||
		(trip1.DestinationAddress != nil && trip2.DestinationAddress == nil) {
		return false
	}
	if trip1.DestinationAddress != nil && trip2.DestinationAddress != nil &&
		*trip1.DestinationAddress != *trip2.DestinationAddress {
		return false
	}

	// Optionally, compare DestinationName as well if it's part of equality criteria
	// if (trip1.DestinationName == nil && trip2.DestinationName != nil) ||
	// 	(trip1.DestinationName != nil && trip2.DestinationName == nil) {
	// 	return false
	// }
	// if trip1.DestinationName != nil && trip2.DestinationName != nil &&
	// 	*trip1.DestinationName != *trip2.DestinationName {
	// 	return false
	// }

	return true
}

// GetDestinationLatitude safely extracts latitude from a trip.
// Returns 0 if the trip is nil.
func GetDestinationLatitude(trip *types.Trip) float64 {
	if trip != nil {
		return trip.DestinationLatitude
	}
	return 0
}

// GetDestinationLongitude safely extracts longitude from a trip.
// Returns 0 if the trip is nil.
func GetDestinationLongitude(trip *types.Trip) float64 {
	if trip != nil {
		return trip.DestinationLongitude
	}
	return 0
}
