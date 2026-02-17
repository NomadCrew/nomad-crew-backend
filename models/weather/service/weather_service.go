package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// WeatherService fetches current weather from the Open-Meteo API.
// Stateless — no subscriptions, no goroutines, no in-memory state.
// Each call to GetWeather makes a fresh API request.
type WeatherService struct {
	client *http.Client
}

func NewWeatherService() *WeatherService {
	return &WeatherService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetWeather fetches current weather for the given coordinates.
func (s *WeatherService) GetWeather(ctx context.Context, tripID string, latitude, longitude float64) (*types.WeatherInfo, error) {
	weather, err := s.fetchCurrentWeather(ctx, latitude, longitude)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}
	weather.TripID = tripID
	return weather, nil
}

// fetchCurrentWeather calls the Open-Meteo API and returns parsed weather data.
func (s *WeatherService) fetchCurrentWeather(ctx context.Context, lat, lon float64) (*types.WeatherInfo, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	params := url.Values{}
	params.Add("latitude", fmt.Sprintf("%.6f", lat))
	params.Add("longitude", fmt.Sprintf("%.6f", lon))
	params.Add("current", "temperature_2m,relative_humidity_2m,apparent_temperature,is_day,precipitation,rain,showers,snowfall,weather_code,cloud_cover,pressure_msl,surface_pressure,wind_speed_10m,wind_direction_10m,wind_gusts_10m")
	params.Add("temperature_unit", "celsius")
	params.Add("wind_speed_unit", "ms")
	params.Add("precipitation_unit", "mm")
	params.Add("timezone", "GMT")

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create weather request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned %s", resp.Status)
	}

	var apiResp types.OpenMeteoCurrentWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	return &types.WeatherInfo{
		Timestamp:           time.Now().UTC(),
		Latitude:            lat,
		Longitude:           lon,
		TemperatureCelsius:  apiResp.Current.Temperature2m,
		ApparentTempCelsius: apiResp.Current.ApparentTemperature,
		HumidityPercent:     apiResp.Current.RelativeHumidity2m,
		IsDay:               apiResp.Current.IsDay == 1,
		PrecipitationMM:     apiResp.Current.Precipitation,
		RainMM:              apiResp.Current.Rain,
		ShowersMM:           apiResp.Current.Showers,
		SnowfallCM:          apiResp.Current.Snowfall,
		WeatherCode:         apiResp.Current.WeatherCode,
		CloudCoverPercent:   apiResp.Current.CloudCover,
		PressureMSLhPa:     apiResp.Current.PressureMSL,
		SurfacePressurehPa: apiResp.Current.SurfacePressure,
		WindSpeedMS:         apiResp.Current.WindSpeed10m,
		WindDirectionDeg:    apiResp.Current.WindDirection10m,
		WindGustsMS:         apiResp.Current.WindGusts10m,
		Description:         types.GetWeatherDescription(apiResp.Current.WeatherCode),
	}, nil
}
