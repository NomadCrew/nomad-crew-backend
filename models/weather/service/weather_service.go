package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/events"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	// Add store import if needed for permission checks
	// "github.com/NomadCrew/nomad-crew-backend/store"
)

// Define interface (optional, but good practice if other services depend on it)
// type WeatherServiceInterface interface {
// 	TriggerImmediateUpdate(ctx context.Context, tripID string, destination types.Destination)
// 	// Potentially add Increment/Decrement Subscribers if used externally
// }

type WeatherService struct {
	eventPublisher types.EventPublisher // Renamed from eventService for consistency
	client         *http.Client
	activeTrips    sync.Map // map[tripID]*tripSubscribers
	// Add store dependency if permission checks are needed
	// store store.TripStore
}

type tripSubscribers struct {
	count       int
	cancel      context.CancelFunc
	destination types.Destination
}

// Ensure interface satisfaction if defined
// var _ WeatherServiceInterface = (*WeatherService)(nil)

// NewWeatherService creates a new WeatherService
// Add store.TripStore if needed for permissions
func NewWeatherService(eventPublisher types.EventPublisher) *WeatherService {
	return &WeatherService{
		eventPublisher: eventPublisher,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		// store: store,
	}
}

// --- Internal methods for managing subscriptions and updates ---

// incrementSubscribers handles adding a subscriber for a trip's weather updates.
// It starts the update loop for the trip if it's the first subscriber.
// Renamed from IncrementSubscribers and made internal.
func (s *WeatherService) incrementSubscribers(tripID string, dest types.Destination) {
	// TODO: Permission Check - Verify user requesting this (via an external method)
	// has access to the tripID before incrementing/starting updates.

	actual, _ := s.activeTrips.LoadOrStore(tripID, &tripSubscribers{
		destination: dest,
	})

	subs := actual.(*tripSubscribers)
	subs.count++

	// Start updates on first subscriber
	if subs.count == 1 {
		ctx, cancel := context.WithCancel(context.Background())
		subs.cancel = cancel
		go s.startUpdates(ctx, tripID, dest)
	}
}

// decrementSubscribers handles removing a subscriber.
// It stops the update loop if it's the last subscriber.
// Renamed from DecrementSubscribers and made internal.
func (s *WeatherService) decrementSubscribers(tripID string) {
	// TODO: Permission Check - Verify user requesting this has access to the tripID.

	actual, ok := s.activeTrips.Load(tripID)
	if !ok {
		return
	}

	subs := actual.(*tripSubscribers)
	subs.count--

	// Stop updates when last subscriber leaves
	if subs.count <= 0 {
		if subs.cancel != nil { // Check if cancel function exists
			subs.cancel()
		}
		s.activeTrips.Delete(tripID)
	}
}

// startUpdates runs the periodic weather update loop for a specific trip.
// This is intended to be run as a goroutine.
// Kept StartWeatherUpdates name from original, but made internal.
func (s *WeatherService) startUpdates(ctx context.Context, tripID string, dest types.Destination) {
	log := logger.GetLogger()
	log.Infow("Starting periodic weather updates", "tripID", tripID, "destination", dest.Address)
	ticker := time.NewTicker(15 * time.Minute) // Consider making interval configurable
	defer ticker.Stop()

	// Initial update immediately
	s.updateWeather(ctx, tripID, dest)

	for {
		select {
		case <-ctx.Done():
			log.Infow("Stopping periodic weather updates due to context cancellation", "tripID", tripID)
			return
		case <-ticker.C:
			log.Debugw("Ticker triggered weather update", "tripID", tripID)
			s.updateWeather(ctx, tripID, dest)
		}
	}
}

// updateWeather fetches and publishes the latest weather information.
// This is the core internal logic called periodically or on demand.
func (s *WeatherService) updateWeather(ctx context.Context, tripID string, destination types.Destination) {
	// Get the logger once at the beginning in a thread-safe way
	log := logger.GetLogger()
	log.Infow("Starting weather update process", "tripID", tripID, "destination", destination.Address)

	var lat, lon float64
	var err error

	if destination.Coordinates != nil {
		lat = destination.Coordinates.Lat
		lon = destination.Coordinates.Lng
		log.Debugw("Using provided coordinates", "tripID", tripID, "lat", lat, "lon", lon)
	} else {
		log.Debugw("Coordinates not provided, attempting geocoding", "tripID", tripID, "address", destination.Address)
		lat, lon, err = s.getCoordinates(destination.Address)
		if err != nil {
			log.Errorw("Failed to get coordinates for weather update", "error", err, "address", destination.Address, "tripID", tripID)
			// Optional: Publish an error event?
			return
		}
		log.Debugw("Geocoding successful", "tripID", tripID, "lat", lat, "lon", lon)
	}

	// Use the same log instance throughout the function
	log.Infow("Fetching current weather data", "lat", lat, "lon", lon, "tripID", tripID)

	weather, err := s.getCurrentWeather(lat, lon)
	if err != nil {
		log.Errorw("Failed to get current weather data", "latitude", lat, "longitude", lon, "error", err, "tripID", tripID)
		// Optional: Publish an error event?
		return
	}

	weather.TripID = tripID // Ensure TripID is set on the weather info

	// Publish event using the centralized helper
	var payloadMap map[string]interface{}
	weatherJSON, err := json.Marshal(weather)
	if err != nil {
		log.Errorw("Failed to marshal weather data for event payload", "error", err, "tripId", tripID)
		return // Don't proceed if payload can't be created
	}
	if err := json.Unmarshal(weatherJSON, &payloadMap); err != nil {
		log.Errorw("Failed to unmarshal weather data into map for event payload", "error", err, "tripId", tripID)
		return // Don't proceed if payload map creation fails
	}

	log.Debugw("Publishing weather update event", "tripID", tripID, "payloadSize", len(weatherJSON))

	if pubErr := events.PublishEventWithContext(
		s.eventPublisher,
		ctx,
		string(types.EventTypeWeatherUpdated),
		tripID,
		"system", // Weather updates are triggered by the system
		payloadMap,
		"weather-service", // Service name identifier
	); pubErr != nil {
		log.Errorw("Weather update event publish failed", "tripID", tripID, "error", pubErr, "event_type", types.EventTypeWeatherUpdated)
		// Return or handle the publish error as appropriate for this service
		return
	}

	log.Infow("Weather update published successfully", "tripID", tripID)
}

// --- Geocoding (Internal Helpers) ---

// getCoordinates fetches the latitude and longitude for a given city/place name using primary and fallback services.
func (s *WeatherService) getCoordinates(city string) (float64, float64, error) {
	log := logger.GetLogger()

	// Primary geocoding service (Open-Meteo)
	lat, lon, err := s.getPrimaryCoordinates(city)
	if err == nil {
		log.Debugw("Primary geocoding successful", "city", city, "lat", lat, "lon", lon)
		return lat, lon, nil
	}

	log.Warnw("Primary geocoding failed, falling back to Nominatim",
		"city", city,
		"error", err)

	// Fallback to Nominatim
	lat, lon, err = s.getNominatimCoordinates(city)
	if err == nil {
		log.Debugw("Fallback geocoding successful (Nominatim)", "city", city, "lat", lat, "lon", lon)
		return lat, lon, nil
	}

	log.Errorw("Both geocoding services failed",
		"city", city,
		"error", err)

	return 0, 0, fmt.Errorf("geocoding failed for: %s", city)
}

// getPrimaryCoordinates uses the Open-Meteo geocoding API.
func (s *WeatherService) getPrimaryCoordinates(city string) (float64, float64, error) {
	baseURL := "https://geocoding-api.open-meteo.com/v1/search"
	params := url.Values{}
	params.Add("name", city)
	params.Add("count", "1") // Only need the top result

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(context.Background(), "GET", requestURL, nil) // Use background context for external API call
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create primary geocoding request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("primary geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("primary geocoding API error (%s): %s", requestURL, resp.Status)
	}

	var geoResp struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
		return 0, 0, fmt.Errorf("failed to decode primary geocoding response: %w", err)
	}

	if len(geoResp.Results) == 0 {
		return 0, 0, fmt.Errorf("no primary geocoding results found for: %s", city)
	}

	return geoResp.Results[0].Latitude, geoResp.Results[0].Longitude, nil
}

// getNominatimCoordinates uses the Nominatim (OpenStreetMap) geocoding API as a fallback.
func (s *WeatherService) getNominatimCoordinates(city string) (float64, float64, error) {
	baseURL := "https://nominatim.openstreetmap.org/search"
	params := url.Values{}
	params.Add("q", city)
	params.Add("format", "json")
	params.Add("limit", "1")

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(context.Background(), "GET", requestURL, nil) // Use background context
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create nominatim request: %w", err)
	}

	// Set a custom User-Agent as required by Nominatim's usage policy
	req.Header.Set("User-Agent", "NomadCrew Backend (https://nomadcrew.uk)") // Replace with your app info

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("nominatim request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("nominatim API error (%s): %s", requestURL, resp.Status)
	}

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, fmt.Errorf("failed to decode nominatim response: %w", err)
	}

	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no nominatim results found for: %s", city)
	}

	lat, errLat := strconv.ParseFloat(results[0].Lat, 64)
	lon, errLon := strconv.ParseFloat(results[0].Lon, 64)
	if errLat != nil || errLon != nil {
		return 0, 0, fmt.Errorf("failed to parse nominatim coordinates: lat_err=%v, lon_err=%v", errLat, errLon)
	}

	return lat, lon, nil
}

// --- Weather Fetching (Internal Helper) ---

// getCurrentWeather fetches the current weather data from Open-Meteo API.
func (s *WeatherService) getCurrentWeather(lat, lon float64) (*types.WeatherInfo, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	params := url.Values{}
	params.Add("latitude", fmt.Sprintf("%.6f", lat))
	params.Add("longitude", fmt.Sprintf("%.6f", lon))
	params.Add("current", "temperature_2m,relative_humidity_2m,apparent_temperature,is_day,precipitation,rain,showers,snowfall,weather_code,cloud_cover,pressure_msl,surface_pressure,wind_speed_10m,wind_direction_10m,wind_gusts_10m")
	params.Add("temperature_unit", "celsius")
	params.Add("wind_speed_unit", "ms")
	params.Add("precipitation_unit", "mm")
	params.Add("timezone", "GMT") // Use GMT/UTC

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(context.Background(), "GET", requestURL, nil) // Use background context
	if err != nil {
		return nil, fmt.Errorf("failed to create weather request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API error (%s): %s", requestURL, resp.Status)
	}

	var apiResp types.OpenMeteoCurrentWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	// Convert API response to our internal WeatherInfo type
	weatherInfo := &types.WeatherInfo{
		Timestamp:           time.Now().UTC(), // Use current time as fetch time
		Latitude:            lat,
		Longitude:           lon,
		TemperatureCelsius:  apiResp.Current.Temperature2m,
		ApparentTempCelsius: apiResp.Current.ApparentTemperature,
		HumidityPercent:     apiResp.Current.RelativeHumidity2m,
		IsDay:               apiResp.Current.IsDay == 1,
		PrecipitationMM:     apiResp.Current.Precipitation,
		RainMM:              apiResp.Current.Rain,
		ShowersMM:           apiResp.Current.Showers,
		SnowfallCM:          apiResp.Current.Snowfall, // API gives cm
		WeatherCode:         apiResp.Current.WeatherCode,
		CloudCoverPercent:   apiResp.Current.CloudCover,
		PressureMSLhPa:      apiResp.Current.PressureMSL,
		SurfacePressurehPa:  apiResp.Current.SurfacePressure,
		WindSpeedMS:         apiResp.Current.WindSpeed10m,
		WindDirectionDeg:    apiResp.Current.WindDirection10m,
		WindGustsMS:         apiResp.Current.WindGusts10m,
		Description:         types.GetWeatherDescription(apiResp.Current.WeatherCode), // Get description from code
	}

	return weatherInfo, nil
}

// --- Public Method (if needed for external triggers) ---

// TriggerImmediateUpdate fetches and publishes weather data immediately for a trip.
// This can be called externally, e.g., when a destination changes.
func (s *WeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, destination types.Destination) {
	log := logger.GetLogger()

	// TODO: Permission Check - Verify the user/caller has permission to trigger updates for this tripID.
	// Requires access to user context and potentially store.TripStore.
	// Example (requires store injection and user ID from context):
	// userID := ctx.Value("userID").(string) // Get user ID
	// role, err := s.store.GetUserRole(ctx, tripID, userID)
	// if err != nil || role == types.MemberRoleNone {
	// 	 log.Warnw("Permission denied for TriggerImmediateUpdate", "tripID", tripID, "userID", userID)
	// 	 return // Or return an error
	// }

	log.Infow("Triggering immediate weather update", "tripID", tripID, "destination", destination.Address)
	// Run updateWeather directly (potentially in a goroutine if it's long-running)
	s.updateWeather(ctx, tripID, destination)
}
