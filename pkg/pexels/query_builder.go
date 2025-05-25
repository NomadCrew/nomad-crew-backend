package pexels

import "github.com/NomadCrew/nomad-crew-backend/types"

// BuildSearchQuery creates an intelligent search query for Pexels based on trip destination
// Priority order for building search query:
// 1. DestinationName (most specific)
// 2. DestinationAddress (fallback)
// 3. Trip Name (last resort if it contains location info)
// 4. Empty string if none of the above contain location info
func BuildSearchQuery(trip *types.Trip) string {
	if trip.DestinationName != nil && *trip.DestinationName != "" {
		return *trip.DestinationName
	}

	if trip.DestinationAddress != nil && *trip.DestinationAddress != "" {
		return *trip.DestinationAddress
	}

	// As a last resort, use trip name if it might contain location information
	// This is a simple heuristic - in a production system, you might want more sophisticated parsing
	if trip.Name != "" {
		return trip.Name
	}

	return ""
}
