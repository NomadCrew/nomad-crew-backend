package types

import "time"

type Trip struct {
    ID          string     `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"start_date"`
    EndDate     time.Time `json:"end_date"`
    CreatedBy   string     `json:"created_by"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type TripUpdate struct {
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Destination string    `json:"destination"`
    StartDate   time.Time `json:"start_date"`
    EndDate     time.Time `json:"end_date"`
}

type TripSearchCriteria struct {
    Destination   string    `json:"destination"`
    StartDateFrom time.Time `json:"start_date_from"`
    StartDateTo   time.Time `json:"start_date_to"`
}