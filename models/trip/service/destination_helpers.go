package service

import (
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// IsDestinationValid checks if a destination has valid coordinates
func IsDestinationValid(dest types.Destination) bool {
	return dest.Coordinates != nil && 
		(dest.Coordinates.Lat != 0 || dest.Coordinates.Lng != 0)
}

// AreDestinationsEqual compares two destinations for equality
func AreDestinationsEqual(dest1, dest2 types.Destination) bool {
	// If one has coordinates and the other doesn't, they're not equal
	if (dest1.Coordinates == nil && dest2.Coordinates != nil) ||
		(dest1.Coordinates != nil && dest2.Coordinates == nil) {
		return false
	}
	
	// If both have coordinates, compare them
	if dest1.Coordinates != nil && dest2.Coordinates != nil {
		if dest1.Coordinates.Lat != dest2.Coordinates.Lat ||
			dest1.Coordinates.Lng != dest2.Coordinates.Lng {
			return false
		}
	}
	
	// Compare other fields
	return dest1.PlaceID == dest2.PlaceID && 
		dest1.Address == dest2.Address
}

// GetDestinationLatitude safely extracts latitude from destination
func GetDestinationLatitude(dest types.Destination) float64 {
	if dest.Coordinates != nil {
		return dest.Coordinates.Lat
	}
	return 0
}

// GetDestinationLongitude safely extracts longitude from destination
func GetDestinationLongitude(dest types.Destination) float64 {
	if dest.Coordinates != nil {
		return dest.Coordinates.Lng
	}
	return 0
} 