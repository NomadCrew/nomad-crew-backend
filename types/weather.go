package types

import (
	"time"
)

type WeatherInfo struct {
    TripID         string    `json:"tripId"`
    Temperature    float64   `json:"temperature"`    // in Celsius
    WindSpeed      float64   `json:"windSpeed"`      // in km/h
    WindDirection  float64   `json:"windDirection"`  // in degrees
    WeatherCode    int       `json:"weatherCode"`
    Timestamp      time.Time `json:"timestamp"`
}

type WeatherServiceError struct {
    Message string `json:"message"`
    Type    string `json:"type"`
}