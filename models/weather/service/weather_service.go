package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	count  int
	cancel context.CancelFunc
	// destination types.Destination // Removed
	latitude  float64
	longitude float64
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

// IncrementSubscribers handles adding a subscriber for a trip's weather updates.
// It starts the update loop for the trip if it's the first subscriber.
// NOTE: Permission validation is handled by callers (handlers/models) before
// invoking weather service methods. This service trusts that callers have
// verified trip membership. See TriggerWeatherUpdateHandler, CreateTrip, UpdateTrip.
func (s *WeatherService) IncrementSubscribers(tripID string, latitude float64, longitude float64) {
	actual, _ := s.activeTrips.LoadOrStore(tripID, &tripSubscribers{
		// destination: dest, // Removed
		latitude:  latitude,
		longitude: longitude,
	})

	subs := actual.(*tripSubscribers)
	subs.count++

	// Start updates on first subscriber
	if subs.count == 1 {
		ctx, cancel := context.WithCancel(context.Background())
		subs.cancel = cancel
		// Pass latitude and longitude to startUpdates
		go s.startUpdates(ctx, tripID, subs.latitude, subs.longitude)
	}
}

// DecrementSubscribers handles removing a subscriber.
// It stops the update loop if it's the last subscriber.
// NOTE: Permission validation is handled by callers before invoking this method.
func (s *WeatherService) DecrementSubscribers(tripID string) {
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

// StartWeatherUpdates starts weather updates for a trip.
// This method implements the WeatherServiceInterface.
func (s *WeatherService) StartWeatherUpdates(ctx context.Context, tripID string, latitude float64, longitude float64) {
	s.IncrementSubscribers(tripID, latitude, longitude)
}

// startUpdates runs the periodic weather update loop for a specific trip.
// This is intended to be run as a goroutine.
// Kept StartWeatherUpdates name from original, but made internal.
func (s *WeatherService) startUpdates(ctx context.Context, tripID string, latitude float64, longitude float64) {
	log := logger.GetLogger()
	log.Infow("Starting periodic weather updates", "tripID", tripID, "latitude", latitude, "longitude", longitude)
	ticker := time.NewTicker(15 * time.Minute) // Consider making interval configurable
	defer ticker.Stop()

	// Initial update immediately
	s.updateWeather(ctx, tripID, latitude, longitude)

	for {
		select {
		case <-ctx.Done():
			log.Infow("Stopping periodic weather updates due to context cancellation", "tripID", tripID)
			return
		case <-ticker.C:
			log.Debugw("Ticker triggered weather update", "tripID", tripID)
			s.updateWeather(ctx, tripID, latitude, longitude)
		}
	}
}

// updateWeather fetches and publishes the latest weather information.
// This is the core internal logic called periodically or on demand.
func (s *WeatherService) updateWeather(ctx context.Context, tripID string, latitude float64, longitude float64) {
	log := logger.GetLogger()
	log.Infow("Starting weather update process", "tripID", tripID, "latitude", latitude, "longitude", longitude)

	// The service now directly uses the provided latitude and longitude.
	// Geocoding logic is removed from this service.

	log.Infow("Fetching current weather data", "lat", latitude, "lon", longitude, "tripID", tripID)

	weather, err := s.getCurrentWeather(latitude, longitude) // Use passed lat/lon directly
	if err != nil {
		log.Errorw("Failed to get current weather data", "latitude", latitude, "longitude", longitude, "error", err, "tripID", tripID)
		return
	}

	weather.TripID = tripID
	weather.Latitude = latitude   // Ensure these are set in the WeatherInfo
	weather.Longitude = longitude // Ensure these are set in the WeatherInfo

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
func (s *WeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, latitude float64, longitude float64) error {
	log := logger.GetLogger()

	val, ok := s.activeTrips.Load(tripID)
	if !ok {
		log.Warnw("TriggerImmediateUpdate called for a trip with no active subscribers", "tripID", tripID)
		// Option 1: Start it temporarily (might be complex if it involves full subscription logic)
		// Option 2: Perform a one-off fetch without altering subscription state (simpler)
		// For now, let's perform a one-off fetch. Client should ideally ensure subscription or handle this case.
		s.updateWeather(ctx, tripID, latitude, longitude) // Pass lat/lon
		return nil                                        // Or return an error indicating no active subscription?
	}

	subs := val.(*tripSubscribers)
	// Update the stored lat/lon in case they changed, though ideally this is managed by IncrementSubscribers
	subs.latitude = latitude
	subs.longitude = longitude

	log.Infow("Triggering immediate weather update", "tripID", tripID, "latitude", latitude, "longitude", longitude)
	// Call the internal updateWeather method directly with the provided coordinates.
	// The startUpdates goroutine will continue its own cycle.
	s.updateWeather(ctx, tripID, latitude, longitude)
	return nil
}

// GetWeather fetches the latest weather information for a trip using cached coordinates.
// Returns an error if the trip has no active weather subscription.
func (s *WeatherService) GetWeather(ctx context.Context, tripID string) (*types.WeatherInfo, error) {
	// Check if we have this trip in our active trips
	actual, ok := s.activeTrips.Load(tripID)
	if !ok {
		return nil, fmt.Errorf("no active weather subscription for trip: %s", tripID)
	}

	subs := actual.(*tripSubscribers)
	return s.GetWeatherByCoords(ctx, tripID, subs.latitude, subs.longitude)
}

// GetWeatherByCoords fetches weather directly using provided coordinates.
// Does not require an active subscription — used by the GET handler as fallback.
func (s *WeatherService) GetWeatherByCoords(ctx context.Context, tripID string, latitude, longitude float64) (*types.WeatherInfo, error) {
	weather, err := s.getCurrentWeather(latitude, longitude)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}

	weather.TripID = tripID
	return weather, nil
}
