package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type WeatherService struct {
	eventService types.EventPublisher
	client       *http.Client
	activeTrips  sync.Map
}

func NewWeatherService(eventService types.EventPublisher) *WeatherService {
	return &WeatherService{
		eventService: eventService,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *WeatherService) StartWeatherUpdates(ctx context.Context, tripID string, destination string) {
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

func (s *WeatherService) updateWeather(ctx context.Context, tripID string, destination string) {
    log := logger.GetLogger()

    log.Debugw("Initiating weather update",
        "tripID", tripID,
        "destination", destination)

    // Get coordinates
    lat, lon, err := s.getCoordinates(destination)
    if err != nil {
        log.Errorw("Failed to get coordinates",
            "destination", destination,
            "error", err,
        )
        return
    }

    // Get weather
    weather, err := s.getCurrentWeather(lat, lon)
    if err != nil {
        log.Errorw("Failed to get weather",
            "destination", destination,
            "error", err,
        )
        return
    }

    log.Infow("Weather data retrieved",
        "temp", weather.Temperature,
        "weatherCode", weather.WeatherCode)

    // Add trip ID to weather info
    weather.TripID = tripID

    // Publish update
    payload, _ := json.Marshal(weather)
    err = s.eventService.Publish(ctx, tripID, types.Event{
        Type:      types.EventTypeWeatherUpdated,
        Payload:   payload,
        Timestamp: time.Now(),
    })

    if err != nil {
        log.Errorw("Failed to publish weather update",
            "tripID", tripID,
            "error", err,
        )
    }

    log.Debugw("Published weather update event",
        "tripID", tripID,
        "payloadSize", len(payload))
}

// getCoordinates fetches the latitude and longitude for a given city/place name
func (s *WeatherService) getCoordinates(city string) (float64, float64, error) {
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

// getCurrentWeather fetches current weather using Open-Meteo API
func (s *WeatherService) getCurrentWeather(lat, lon float64) (*types.WeatherInfo, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	params := url.Values{}
	params.Add("latitude", fmt.Sprintf("%f", lat))
	params.Add("longitude", fmt.Sprintf("%f", lon))
	params.Add("current_weather", "true")

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
		CurrentWeather struct {
			Temperature   float64 `json:"temperature"`
			WindSpeed     float64 `json:"windspeed"`
			WindDirection float64 `json:"winddirection"`
			WeatherCode   int     `json:"weathercode"`
			Time          string  `json:"time"`
		} `json:"current_weather"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return nil, err
	}

	timestamp, err := time.Parse("2006-01-02T15:04", forecast.CurrentWeather.Time)
	if err != nil {
		return nil, err
	}

	return &types.WeatherInfo{
		Temperature:   forecast.CurrentWeather.Temperature,
		WindSpeed:     forecast.CurrentWeather.WindSpeed,
		WindDirection: forecast.CurrentWeather.WindDirection,
		WeatherCode:   forecast.CurrentWeather.WeatherCode,
		Timestamp:     timestamp,
	}, nil
}

func (s *WeatherService) TriggerImmediateUpdate(ctx context.Context, tripID string, destination string) {
    log := logger.GetLogger()
    
    // Execute weather update regardless of active status
    log.Infow("Triggering immediate weather update", 
        "tripID", tripID,
        "destination", destination)
        
    s.updateWeather(ctx, tripID, destination)
}
