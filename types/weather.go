package types

import (
	"context"
	"time"
)

type WeatherInfo struct {
    TripID              string          `json:"tripId"`
    Temperature2m       float64         `json:"temperature_2m"`
    WeatherCode         int             `json:"weather_code"`
    HourlyForecast []HourlyWeather `json:"hourly_forecast"`
	Timestamp           time.Time       `json:"timestamp"`
}

type WeatherServiceError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type WeatherServiceInterface interface {
	StartWeatherUpdates(ctx context.Context, tripID string, destination Destination)
	IncrementSubscribers(tripID string, dest Destination)
	DecrementSubscribers(tripID string)
	TriggerImmediateUpdate(ctx context.Context, tripID string, destination Destination)
}

type HourlyWeather struct {
    Timestamp     time.Time `json:"timestamp"`
    Temperature2m float64   `json:"temperature_2m"`
    WeatherCode   int       `json:"weather_code"`
    Precipitation float64   `json:"precipitation"`
}
