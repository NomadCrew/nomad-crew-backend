package types

import (
	"time"
)

type WeatherInfo struct {
	TripID              string    `json:"tripId"`
	Temperature2m       float64   `json:"temperature_2m"`
	RelativeHumidity2m  int       `json:"relative_humidity_2m"`
	ApparentTemperature float64   `json:"apparent_temperature"`
	IsDay               bool      `json:"is_day"`
	Rain                float64   `json:"rain"`
	Showers             float64   `json:"showers"`
	Snowfall            float64   `json:"snowfall"`
	WeatherCode         int       `json:"weather_code"`
	CloudCover          int       `json:"cloud_cover"`
	Timestamp           time.Time `json:"timestamp"`
}

type WeatherServiceError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
