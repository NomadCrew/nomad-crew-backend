package types

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total"`
	} `json:"pagination"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// StatusResponse represents a simple status/success response
type StatusResponse struct {
	Status string `json:"status" example:"success"`
}

// PaginationParams defines common pagination query parameters
type PaginationParams struct {
	Limit  int `form:"limit" binding:"omitempty,gte=0"`
	Offset int `form:"offset" binding:"omitempty,gte=0"`
}

type ListTodosParams struct {
	Limit  int `form:"limit,default=20"`
	Offset int `form:"offset,default=0"`
}

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// ChatMessagesResponse defines the response structure for chat messages endpoint
type ChatMessagesResponse struct {
	Messages   []ChatMessage      `json:"messages"`
	Pagination ChatPaginationInfo `json:"pagination"`
}

// ChatPaginationInfo defines pagination info for chat messages
type ChatPaginationInfo struct {
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
	Limit      int     `json:"limit"`
	Before     string  `json:"before,omitempty"`
}

// LocationResponse represents a single location update response
type LocationResponse struct {
	ID           string  `json:"id" example:"user123_trip456_20260214"`
	UserID       string  `json:"user_id" example:"user-uuid-123"`
	TripID       string  `json:"trip_id,omitempty" example:"trip-uuid-456"`
	Latitude     float64 `json:"latitude" example:"40.7128"`
	Longitude    float64 `json:"longitude" example:"-74.0060"`
	Accuracy     float64 `json:"accuracy" example:"10.5"`
	Timestamp    string  `json:"timestamp" example:"2026-02-14T12:00:00Z"`
	PrivacyLevel string  `json:"privacy_level" example:"approximate"`
	CreatedAt    string  `json:"created_at" example:"2026-02-14T12:00:00Z"`
	UpdatedAt    string  `json:"updated_at" example:"2026-02-14T12:00:00Z"`
}

// LocationsResponse defines the response structure for locations endpoint
type LocationsResponse struct {
	Locations  []MemberLocation   `json:"locations"`
	Pagination LocationPagination `json:"pagination"`
}

// LocationPagination defines pagination info for locations
type LocationPagination struct {
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
}
