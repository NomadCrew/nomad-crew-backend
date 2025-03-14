// pkg/valueobjects/geopoint_test.go
package valueobjects

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGeoPoint(t *testing.T) {
	tests := []struct {
		name        string
		latitude    float64
		longitude   float64
		shouldError bool
	}{
		{
			name:        "valid coordinates",
			latitude:    51.5074,
			longitude:   -0.1278,
			shouldError: false,
		},
		{
			name:        "invalid latitude - too high",
			latitude:    91.0,
			longitude:   0.0,
			shouldError: true,
		},
		{
			name:        "invalid latitude - too low",
			latitude:    -91.0,
			longitude:   0.0,
			shouldError: true,
		},
		{
			name:        "invalid longitude - too high",
			latitude:    0.0,
			longitude:   181.0,
			shouldError: true,
		},
		{
			name:        "invalid longitude - too low",
			latitude:    0.0,
			longitude:   -181.0,
			shouldError: true,
		},
		{
			name:        "edge case - max valid values",
			latitude:    90.0,
			longitude:   180.0,
			shouldError: false,
		},
		{
			name:        "edge case - min valid values",
			latitude:    -90.0,
			longitude:   -180.0,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			point, err := NewGeoPoint(tt.latitude, tt.longitude)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, point)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, point)
				assert.Equal(t, tt.latitude, point.Latitude())
				assert.Equal(t, tt.longitude, point.Longitude())
			}
		})
	}
}

func TestGeoPointDistance(t *testing.T) {
	// Test cases with known distances
	tests := []struct {
		name         string
		point1       GeoPoint
		point2       GeoPoint
		expectDist   float64
		expectMargin float64 // Acceptable margin of error in meters
	}{
		{
			name:         "London to Paris",
			point1:       GeoPoint{51.5074, -0.1278}, // London
			point2:       GeoPoint{48.8566, 2.3522},  // Paris
			expectDist:   343457.0,                   // ~343.5 km
			expectMargin: 100.0,                      // 100m margin of error
		},
		{
			name:         "Same point",
			point1:       GeoPoint{0.0, 0.0},
			point2:       GeoPoint{0.0, 0.0},
			expectDist:   0.0,
			expectMargin: 0.1,
		},
		{
			name:         "Antipodes",
			point1:       GeoPoint{0.0, 0.0},
			point2:       GeoPoint{0.0, 180.0},
			expectDist:   20015087.0, // ~20,015 km (half Earth's circumference)
			expectMargin: 1000.0,     // 1km margin of error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := tt.point1.DistanceTo(tt.point2)
			assert.InDelta(t, tt.expectDist, distance, tt.expectMargin)

			// Distance should be the same in reverse
			reverseDistance := tt.point2.DistanceTo(tt.point1)
			assert.InDelta(t, distance, reverseDistance, 0.1)
		})
	}
}

func TestGeoPointIsWithinRadius(t *testing.T) {
	// Create some test points
	london, err := NewGeoPoint(51.5074, -0.1278)
	require.NoError(t, err)

	paris, err := NewGeoPoint(48.8566, 2.3522)
	require.NoError(t, err)

	tests := []struct {
		name     string
		point1   *GeoPoint
		point2   *GeoPoint
		radius   float64
		expected bool
	}{
		{
			name:     "Within radius",
			point1:   london,
			point2:   paris,
			radius:   344000.0, // 344km
			expected: true,
		},
		{
			name:     "Outside radius",
			point1:   london,
			point2:   paris,
			radius:   340000.0, // 340km
			expected: false,
		},
		{
			name:     "Exactly on radius",
			point1:   london,
			point2:   paris,
			radius:   london.DistanceTo(*paris),
			expected: true,
		},
		{
			name:     "Negative radius",
			point1:   london,
			point2:   paris,
			radius:   -1.0,
			expected: false,
		},
		{
			name:     "Zero radius",
			point1:   london,
			point2:   london,
			radius:   0.0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.point1.IsWithinRadius(*tt.point2, tt.radius)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGeoPointString(t *testing.T) {
	point, err := NewGeoPoint(51.5074, -0.1278)
	require.NoError(t, err)

	expected := "(51.507400, -0.127800)"
	assert.Equal(t, expected, point.String())
}
