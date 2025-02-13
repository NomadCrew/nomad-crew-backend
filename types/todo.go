package types

import "time"

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
    Required  bool       `json:"required"`
    CreatedBy string     `json:"createdBy"`
    CreatedAt time.Time  `json:"createdAt"`
    UpdatedAt time.Time  `json:"updatedAt"`
}

// ListTodosParams extends PaginationParams with todo-specific filters
type ListTodosParams struct {
    PaginationParams
    TripID string     `form:"tripId" binding:"required"`
    Status TodoStatus `form:"status"`
}

type TodoCreate struct {
    TripID    string `json:"tripId" binding:"required"`
    Text      string `json:"text" binding:"required"`
    Required  bool   `json:"required"`
}

type TodoUpdate struct {
    Status *TodoStatus `json:"status,omitempty"`
    Text   *string    `json:"text,omitempty"`
}

func (s TodoStatus) Ptr() *TodoStatus {
    return &s
}
