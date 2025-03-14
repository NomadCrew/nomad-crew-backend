package types

import "time"

type TripStatus string

const (
	TripStatusPlanning  TripStatus = "PLANNING"  // Initial state when trip is being set up
	TripStatusActive    TripStatus = "ACTIVE"    // Trip is currently ongoing
	TripStatusCompleted TripStatus = "COMPLETED" // Trip has finished normally
	TripStatusCancelled TripStatus = "CANCELLED" // Trip was cancelled before or during
)

type Destination struct {
	Address     string `json:"address"`
	PlaceID     string `json:"placeId"`
	Coordinates *struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"coordinates,omitempty"`
}

type Trip struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	Destination        Destination `json:"destination"`
	StartDate          time.Time   `json:"startDate"`
	EndDate            time.Time   `json:"endDate"`
	Status             TripStatus  `json:"status"`
	CreatedBy          string      `json:"createdBy"`
	CreatedAt          time.Time   `json:"createdAt"`
	UpdatedAt          time.Time   `json:"updatedAt"`
	BackgroundImageURL string      `json:"backgroundImageUrl"`
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
		TripStatusCompleted: {}, // Terminal state
		TripStatusCancelled: {}, // Terminal state
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
	Name        *string      `json:"name,omitempty"`
	Description *string      `json:"description,omitempty"`
	Destination *Destination `json:"destination,omitempty"`
	StartDate   *time.Time   `json:"startDate,omitempty"`
	EndDate     *time.Time   `json:"endDate,omitempty"`
	Status      TripStatus   `json:"status,omitempty"`
}

type TripSearchCriteria struct {
	UserID        string
	StartDate     time.Time
	EndDate       time.Time
	StartDateFrom time.Time
	StartDateTo   time.Time
	Limit         int
	Offset        int
	Destination   string
}

type TripListCriteria struct {
	UserID    string    `json:"userId,omitempty"`
	Status    []string  `json:"status,omitempty"`
	StartDate time.Time `json:"startDate,omitempty"`
	EndDate   time.Time `json:"endDate,omitempty"`
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
}

type PaginatedTrips struct {
	Trips      []*Trip `json:"trips"`
	Pagination struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination"`
}

type TripWithMembers struct {
	Trip    Trip              `json:"trip"`
	Members []*TripMembership `json:"members"`
	// Optional: Add other aggregated trip data
	// Weather   WeatherForecast    `json:"weather,omitempty"`
	// Expenses  []Expense          `json:"expenses,omitempty"`
}

func (s InvitationStatus) IsValid() bool {
	switch s {
	case InvitationStatusPending, InvitationStatusAccepted, InvitationStatusDeclined:
		return true
	}
	return false
}
