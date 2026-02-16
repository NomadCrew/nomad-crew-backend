package types

import "time"

// Feedback represents a feedback entry stored in the database.
type Feedback struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// FeedbackCreate represents the request body for submitting feedback.
type FeedbackCreate struct {
	Name    string `json:"name" binding:"required,min=1,max=100"`
	Email   string `json:"email" binding:"required,email,max=255"`
	Message string `json:"message" binding:"required,min=10,max=5000"`
	Source  string `json:"source,omitempty"`
}
