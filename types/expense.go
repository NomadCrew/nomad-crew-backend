package types

import "time"

// Expense represents a shared expense within a trip.
type Expense struct {
	ID          string    `json:"id"`
	TripID      string    `json:"tripId"`
	PaidBy      string    `json:"paidBy"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Category    string    `json:"category,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CreateExpenseStoreParams struct {
	TripID      string  `json:"tripId"`
	PaidBy      string  `json:"paidBy"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Category    string  `json:"category,omitempty"`
}

type UpdateExpenseStoreParams struct {
	ID          string   `json:"id"`
	TripID      string   `json:"tripId"`
	Description *string  `json:"description,omitempty"`
	Amount      *float64 `json:"amount,omitempty"`
	Currency    *string  `json:"currency,omitempty"`
	Category    *string  `json:"category,omitempty"`
}

type ExpenseParticipant struct {
	ID        string  `json:"id"`
	ExpenseID string  `json:"expenseId"`
	UserID    string  `json:"userId"`
	Share     float64 `json:"share"`
}

type CreateParticipantStoreParams struct {
	ExpenseID string  `json:"expenseId"`
	UserID    string  `json:"userId"`
	Share     float64 `json:"share"`
}

type Settlement struct {
	ID        string    `json:"id"`
	TripID    string    `json:"tripId"`
	PayerID   string    `json:"payerId"`
	PayeeID   string    `json:"payeeId"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Settled   bool      `json:"settled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateSettlementStoreParams struct {
	TripID   string  `json:"tripId"`
	PayerID  string  `json:"payerId"`
	PayeeID  string  `json:"payeeId"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type MemberBalance struct {
	UserID  string  `json:"userId"`
	Balance float64 `json:"balance"`
}

type CreateExpenseActivityStoreParams struct {
	ExpenseID string `json:"expenseId"`
	UserID    string `json:"userId"`
	Action    string `json:"action"`
}
