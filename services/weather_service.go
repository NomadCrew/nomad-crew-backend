package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type WeatherService struct {
	eventService types.EventPublisher
	client       *http.Client
	activeTrips  sync.Map // map[tripID]*tripSubscribers
}

type tripSubscribers struct {
	count       int
	cancel      context.CancelFunc
	destination types.Destination
}

func NewWeatherService(eventService types.EventPublisher) *WeatherService {
	return &WeatherService{
		eventService: eventService,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *WeatherService) IncrementSubscribers(tripID string, dest types.Destination) {
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

func (s *WeatherService) DecrementSubscribers(tripID string) {
	actual, ok := s.activeTrips.Load(tripID)
	if !ok {
		return
	}

	subs := actual.(*tripSubscribers)
	subs.count--

	// Stop updates when last subscriber leaves
	if subs.count <= 0 {
		subs.cancel()
		s.activeTrips.Delete(tripID)
	}
}

func (s *WeatherService) StartWeatherUpdates(ctx context.Context, tripID string, destination types.Destination) {
	s.activeTrips.Store(tripID, destination)

	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		// Initial update
		s.updateWeather(ctx, tripID, destination)

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				s.activeTrips.Delete(tripID)
				return
			case <-ticker.C:
				s.updateWeather(ctx, tripID, destination)
			}
		}
	}()
}

func (s *WeatherService) startUpdates(ctx context.Context, tripID string, dest types.Destination) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// Initial update
	s.updateWeather(ctx, tripID, dest)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateWeather(ctx, tripID, dest)
		}
	}
}

func (s *WeatherService) updateWeather(ctx context.Context, tripID string, destination types.Destination) {
	log := logger.GetLogger()
	log.Infow("Starting weather update", "tripID", tripID, "destination", destination.Address)

	// Get coordinates
	var lat, lon float64
	if destination.Coordinates != nil {
		lat = destination.Coordinates.Lat
		lon = destination.Coordinates.Lng
	} else {
		// Fall back to geocoding if no coordinates
		var err error
		lat, lon, err = s.getCoordinates(destination.Address)
		if err != nil {
			log.Errorw("Failed to get coordinates",
				"destination", destination,
				"error", err,
			)
			return
		}
	}

	weather, err := s.getCurrentWeather(lat, lon)
	log.Infow("Fetching weather data",
		"lat", lat,
		"lon", lon)
	if err != nil {
		log.Errorw("Failed to get weather",
			"destination", destination,
			"error", err,
		)
		return
	}

	log.Infow("Weather data retrieved",
		"temperature", weather.Temperature2m,
		"humidity", weather.RelativeHumidity2m,
		"weatherCode", weather.WeatherCode,
		"timestamp", weather.Timestamp)

	// Add trip ID to weather info
	weather.TripID = tripID

	// Publish update
	payload, _ := json.Marshal(weather)

	if apiErr := s.eventService.Publish(ctx, tripID, types.Event{
		Type:    types.EventTypeWeatherUpdated,
		Payload: payload,
	}); apiErr != nil {
		log.Errorw("Event publish failed",
			"tripID", tripID,
			"error", apiErr,
			"event_type", types.EventTypeWeatherUpdated,
		)
	} else {
		log.Infow("Successfully published weather update",
			"tripID", tripID,
			"event_size", len(payload),
		)
	}

	log.Debugw("Weather update completed",
		"tripID", tripID,
		"temperature", weather.Temperature2m,
		"timestamp", weather.Timestamp,
	)
}

// getCoordinates fetches the latitude and longitude for a given city/place name
func (s *WeatherService) getCoordinates(city string) (float64, float64, error) {
	log := logger.GetLogger()

	// Primary geocoding service
	lat, lon, err := s.getPrimaryCoordinates(city)
	if err == nil {
		return lat, lon, nil
	}

	log.Warnw("Primary geocoding failed, falling back to Nominatim",
		"city", city,
		"error", err)

	// Fallback to Nominatim
	lat, lon, err = s.getNominatimCoordinates(city)
	if err == nil {
		return lat, lon, nil
	}

	log.Errorw("Both geocoding services failed",
		"city", city,
		"error", err)

	return 0, 0, fmt.Errorf("no location found for: %s", city)
}

func (s *WeatherService) getPrimaryCoordinates(city string) (float64, float64, error) {
	baseURL := "https://geocoding-api.open-meteo.com/v1/search"
	params := url.Values{}
	params.Add("name", city)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil)
	if err != nil {
		return 0, 0, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("geocoding API error: %s", resp.Status)
	}

	var geoResp struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
		return 0, 0, err
	}

	if len(geoResp.Results) == 0 {
		return 0, 0, fmt.Errorf("no location found for: %s", city)
	}

	return geoResp.Results[0].Latitude, geoResp.Results[0].Longitude, nil
}

func (s *WeatherService) getNominatimCoordinates(city string) (float64, float64, error) {
	baseURL := "https://nominatim.openstreetmap.org/search"
	params := url.Values{}
	params.Add("q", city)
	params.Add("format", "json")
	params.Add("limit", "1")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil)
	if err != nil {
		return 0, 0, err
	}

	// Set a custom User-Agent as required by Nominatim's usage policy
	req.Header.Set("User-Agent", "NomadCrewWeatherService/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("nominatim api error: %s", resp.Status)
	}

	var nominatimResp []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nominatimResp); err != nil {
		return 0, 0, err
	}

	if len(nominatimResp) == 0 {
		return 0, 0, fmt.Errorf("no location found for: %s", city)
	}

	lat, err := strconv.ParseFloat(nominatimResp[0].Lat, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %s", nominatimResp[0].Lat)
	}

	lon, err := strconv.ParseFloat(nominatimResp[0].Lon, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %s", nominatimResp[0].Lon)
	}

	return lat, lon, nil
}

// getCurrentWeather fetches current weather using Open-Meteo API
func (s *WeatherService) getCurrentWeather(lat, lon float64) (*types.WeatherInfo, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	params := url.Values{}
	params.Add("latitude", fmt.Sprintf("%f", lat))
	params.Add("longitude", fmt.Sprintf("%f", lon))
	params.Add("current", "temperature_2m,relative_humidity_2m,apparent_temperature,is_day,rain,showers,snowfall,weather_code,cloud_cover")

	log := logger.GetLogger()
	log.Infow("Fetching detailed weather data",
		"lat", lat,
		"lon", lon,
		"params", params.Encode())

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API error: %s", resp.Status)
	}

	var forecast struct {
		Current struct {
			Time                string  `json:"time"`
			Temperature2m       float64 `json:"temperature_2m"`
			RelativeHumidity2m  int     `json:"relative_humidity_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			IsDay               int     `json:"is_day"`
			Rain                float64 `json:"rain"`
			Showers             float64 `json:"showers"`
			Snowfall            float64 `json:"snowfall"`
			WeatherCode         int     `json:"weather_code"`
			CloudCover          int     `json:"cloud_cover"`
		} `json:"current"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return nil, err
	}

	timestamp, err := time.Parse("2006-01-02T15:04", forecast.Current.Time)
	if err != nil {
		return nil, err
	}

	return &types.WeatherInfo{
		Temperature2m:       forecast.Current.Temperature2m,
		RelativeHumidity2m:  forecast.Current.RelativeHumidity2m,
		ApparentTemperature: forecast.Current.ApparentTemperature,
		IsDay:               forecast.Current.IsDay == 1,
		Rain:                forecast.Current.Rain,
		Showers:             forecast.Current.Showers,
		Snowfall:            forecast.Current.Snowfall,
		WeatherCode:         forecast.Current.WeatherCode,
		CloudCover:          forecast.Current.CloudCover,
		Timestamp:           timestamp,
	}, nil
}

func (s *WeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, destination types.Destination) {
	log := logger.GetLogger()

	// Execute weather update regardless of active status
	log.Infow("Triggering immediate weather update",
		"tripID", tripID,
		"destination", destination)

	s.updateWeather(ctx, tripID, destination)
}
