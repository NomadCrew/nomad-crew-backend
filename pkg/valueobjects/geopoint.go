// pkg/valueobjects/geopoint.go
package valueobjects

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// GeoPoint represents a geographic point with latitude and longitude
type GeoPoint struct {
	latitude  float64
	longitude float64
}

// NewGeoPoint creates a new GeoPoint with validation
func NewGeoPoint(lat, lng float64) (*GeoPoint, error) {
	if err := validateCoordinates(lat, lng); err != nil {
		return nil, err
	}

	return &GeoPoint{
		latitude:  lat,
		longitude: lng,
	}, nil
}

// Latitude returns the latitude value
func (g GeoPoint) Latitude() float64 {
	return g.latitude
}

// Longitude returns the longitude value
func (g GeoPoint) Longitude() float64 {
	return g.longitude
}

// DistanceTo calculates the distance to another point in meters using the Haversine formula
func (g GeoPoint) DistanceTo(other GeoPoint) float64 {
	const earthRadius = 6371000 // Earth's radius in meters

	lat1 := degreesToRadians(g.latitude)
	lng1 := degreesToRadians(g.longitude)
	lat2 := degreesToRadians(other.latitude)
	lng2 := degreesToRadians(other.longitude)

	dlat := lat2 - lat1
	dlng := lng2 - lng1

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlng/2)*math.Sin(dlng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// IsWithinRadius checks if another point is within the specified radius in meters
func (g GeoPoint) IsWithinRadius(other GeoPoint, radius float64) bool {
	if radius < 0 {
		return false
	}
	return g.DistanceTo(other) <= radius
}

// String returns a string representation of the geographic point
func (g GeoPoint) String() string {
	return fmt.Sprintf("(%f, %f)", g.latitude, g.longitude)
}

// private helpers

func validateCoordinates(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return errors.ValidationFailed(
			"invalid latitude",
			fmt.Sprintf("latitude %f is outside valid range [-90, 90]", lat),
		)
	}

	if lng < -180 || lng > 180 {
		return errors.ValidationFailed(
			"invalid longitude",
			fmt.Sprintf("longitude %f is outside valid range [-180, 180]", lng),
		)
	}

	return nil
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// Add new methods to make GeoPoint more useful:
func (g GeoPoint) ToCoordinates() *types.Coordinates {
	return &types.Coordinates{
		Lat: g.latitude,
		Lng: g.longitude,
	}
}

// Add MarshalJSON/UnmarshalJSON to control serialization
func (g GeoPoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Latitude  float64 `json:"lat"`
		Longitude float64 `json:"lng"`
	}{
		Latitude:  g.latitude,
		Longitude: g.longitude,
	})
}

// Add constructor from domain types
func NewGeoPointFromCoordinates(coords *types.Coordinates) (*GeoPoint, error) {
	if coords == nil {
		return nil, errors.ValidationFailed(
			"invalid coordinates",
			"coordinates cannot be nil",
		)
	}
	return NewGeoPoint(coords.Lat, coords.Lng)
}
