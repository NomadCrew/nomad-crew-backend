package types

import "time"

type TripStatus string

const (
	TripStatusPlanning  TripStatus = "PLANNING"  // Initial state when trip is being set up
	TripStatusActive    TripStatus = "ACTIVE"    // Trip is currently ongoing
	TripStatusCompleted TripStatus = "COMPLETED" // Trip has finished normally
	TripStatusCancelled TripStatus = "CANCELLED" // Trip was cancelled before or during
)

type Trip struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Destination string     `json:"destination"`
	StartDate   time.Time  `json:"startDate"`
	EndDate     time.Time  `json:"endDate"`
	Status      TripStatus `json:"status"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// IsValidTransition checks if a status transition is allowed
func (ts TripStatus) IsValidTransition(newStatus TripStatus) bool {
	transitions := map[TripStatus][]TripStatus{
		TripStatusPlanning: {
			TripStatusActive,
			TripStatusCancelled,
		},
		TripStatusActive: {
			TripStatusCompleted,
			TripStatusCancelled,
		},
		TripStatusCompleted: {},  // Terminal state
		TripStatusCancelled: {},  // Terminal state
	}

	allowedTransitions, exists := transitions[ts]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == newStatus {
			return true
		}
	}
	return false
}

// String provides a string representation of the status
func (ts TripStatus) String() string {
	return string(ts)
}

// IsValid checks if the status is a valid trip status
func (ts TripStatus) IsValid() bool {
	switch ts {
	case TripStatusPlanning, TripStatusActive, TripStatusCompleted, TripStatusCancelled:
		return true
	default:
		return false
	}
}

type TripUpdate struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Destination string     `json:"destination"`
	StartDate   time.Time  `json:"startDate"`
	EndDate     time.Time  `json:"endDate"`
	Status      TripStatus `json:"status"`
}

type TripSearchCriteria struct {
	Destination   string    `json:"destination"`
	StartDateFrom time.Time `json:"startDateFrom"`
	StartDateTo   time.Time `json:"startDateTo"`
}