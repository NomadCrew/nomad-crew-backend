package types

import "time"

type Trip struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"startDate"`
    EndDate     time.Time `json:"endDate"`
    CreatedBy   string    `json:"createdBy"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}

type TripUpdate struct {
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"startDate"`
    EndDate     time.Time `json:"endDate"`
}

type TripSearchCriteria struct {
    Destination   string    `json:"destination"`
    StartDateFrom time.Time `json:"startDateFrom"`
    StartDateTo   time.Time `json:"startDateTo"`
}