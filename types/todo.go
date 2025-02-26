package types

import (
	"time"
)

type TodoStatus string

const (
	TodoStatusComplete   TodoStatus = "COMPLETE"
	TodoStatusIncomplete TodoStatus = "INCOMPLETE"
)

type Todo struct {
	ID        string     `json:"id"`
	TripID    string     `json:"tripId"`
	Text      string     `json:"text"`
	Status    TodoStatus `json:"status"`
	CreatedBy string     `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type TodoCreate struct {
	TripID string `json:"tripId" binding:"required"`
	Text   string `json:"text" binding:"required"`
}

type TodoUpdate struct {
	Status *TodoStatus `json:"status,omitempty"`
	Text   *string     `json:"text,omitempty"`
}

// Add Ptr methods for status constants
func (s TodoStatus) Ptr() *TodoStatus {
	return &s
}
