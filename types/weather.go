package types

import (
	"context"
	"time"
)

// --- Weather API Response Structures ---

// OpenMeteoCurrentWeatherResponse mirrors the 'current' object from the Open-Meteo API response.
// Based on fields used in weather_service.go
type OpenMeteoCurrentWeatherResponse struct {
	Current struct {
		Time                string  `json:"time"`     // Example: "2023-10-27T10:00"
		Interval            int     `json:"interval"` // Example: 900 (seconds)
		Temperature2m       float64 `json:"temperature_2m"`
		RelativeHumidity2m  int     `json:"relative_humidity_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		IsDay               int     `json:"is_day"` // 1 for day, 0 for night
		Precipitation       float64 `json:"precipitation"`
		Rain                float64 `json:"rain"`
		Showers             float64 `json:"showers"`
		Snowfall            float64 `json:"snowfall"` // API docs say cm
		WeatherCode         int     `json:"weather_code"`
		CloudCover          int     `json:"cloud_cover"`
		PressureMSL         float64 `json:"pressure_msl"`
		SurfacePressure     float64 `json:"surface_pressure"`
		WindSpeed10m        float64 `json:"wind_speed_10m"`
		WindDirection10m    int     `json:"wind_direction_10m"`
		WindGusts10m        float64 `json:"wind_gusts_10m"`
	} `json:"current"`
}

// --- Internal Weather Types ---

// WeatherInfo represents the processed weather information stored or published internally.
// Expanded fields based on usage in weather_service.go
type WeatherInfo struct {
	TripID              string    `json:"tripId"`
	Timestamp           time.Time `json:"timestamp"`
	Latitude            float64   `json:"latitude"`
	Longitude           float64   `json:"longitude"`
	TemperatureCelsius  float64   `json:"temperatureCelsius"`
	ApparentTempCelsius float64   `json:"apparentTempCelsius"`
	HumidityPercent     int       `json:"humidityPercent"`
	IsDay               bool      `json:"isDay"`
	PrecipitationMM     float64   `json:"precipitationMM"`
	RainMM              float64   `json:"rainMM"`
	ShowersMM           float64   `json:"showersMM"`
	SnowfallCM          float64   `json:"snowfallCM"`
	WeatherCode         int       `json:"weatherCode"` // WMO Weather interpretation codes
	Description         string    `json:"description"`
	CloudCoverPercent   int       `json:"cloudCoverPercent"`
	PressureMSLhPa      float64   `json:"pressureMSLhPa"`
	SurfacePressurehPa  float64   `json:"surfacePressurehPa"`
	WindSpeedMS         float64   `json:"windSpeedMS"`
	WindDirectionDeg    int       `json:"windDirectionDeg"`
	WindGustsMS         float64   `json:"windGustsMS"`
}

type WeatherServiceError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type WeatherServiceInterface interface {
	StartWeatherUpdates(ctx context.Context, tripID string, latitude float64, longitude float64)
	IncrementSubscribers(tripID string, latitude float64, longitude float64)
	DecrementSubscribers(tripID string)
	TriggerImmediateUpdate(ctx context.Context, tripID string, latitude float64, longitude float64) error
	GetWeather(ctx context.Context, tripID string) (*WeatherInfo, error)
	GetWeatherByCoords(ctx context.Context, tripID string, latitude, longitude float64) (*WeatherInfo, error)
}

type HourlyWeather struct {
	Timestamp     time.Time `json:"timestamp"`
	Temperature2m float64   `json:"temperature_2m"`
	WeatherCode   int       `json:"weather_code"`
	Precipitation float64   `json:"precipitation"`
}

// GetWeatherDescription converts a WMO Weather interpretation code to a human-readable description
// Based on WMO Code Table 4677: https://www.nodc.noaa.gov/archive/arc0021/0002199/1.1/data/0-data/HTML/WMO-CODE/WMO4677.HTM
func GetWeatherDescription(code int) string {
	descriptions := map[int]string{
		0:  "Clear sky",
		1:  "Mainly clear",
		2:  "Partly cloudy",
		3:  "Overcast",
		45: "Foggy",
		48: "Depositing rime fog",
		51: "Light drizzle",
		53: "Moderate drizzle",
		55: "Dense drizzle",
		56: "Light freezing drizzle",
		57: "Dense freezing drizzle",
		61: "Slight rain",
		63: "Moderate rain",
		65: "Heavy rain",
		66: "Light freezing rain",
		67: "Heavy freezing rain",
		71: "Slight snow fall",
		73: "Moderate snow fall",
		75: "Heavy snow fall",
		77: "Snow grains",
		80: "Slight rain showers",
		81: "Moderate rain showers",
		82: "Violent rain showers",
		85: "Slight snow showers",
		86: "Heavy snow showers",
		95: "Thunderstorm",
		96: "Thunderstorm with slight hail",
		99: "Thunderstorm with heavy hail",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Unknown weather condition"
}
