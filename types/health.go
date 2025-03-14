package types

type HealthStatus string

const (
	HealthStatusUp       HealthStatus = "UP"
	HealthStatusDown     HealthStatus = "DOWN"
	HealthStatusDegraded HealthStatus = "DEGRADED"
)

type HealthComponent struct {
	Status  HealthStatus `json:"status"`
	Details string       `json:"details,omitempty"`
}

type HealthCheck struct {
	Status     HealthStatus               `json:"status"`
	Components map[string]HealthComponent `json:"components"`
	Version    string                     `json:"version"`
	Timestamp  string                     `json:"timestamp"`
	Uptime     string                     `json:"uptime"`
}
