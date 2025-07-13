package types

import "time"

// StandardResponse is the unified response format for all API endpoints
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *MetaInfo   `json:"meta,omitempty"`
}

// ErrorInfo contains structured error information
type ErrorInfo struct {
	Code    string                 `json:"code"`              // Machine-readable error code
	Message string                 `json:"message"`           // Human-readable error message
	Details map[string]interface{} `json:"details,omitempty"` // Additional error context
	TraceID string                 `json:"trace_id,omitempty"`// Request trace ID for debugging
}

// MetaInfo contains metadata about the response
type MetaInfo struct {
	RequestID  string     `json:"request_id,omitempty"`
	Timestamp  time.Time  `json:"timestamp"`
	Version    string     `json:"version,omitempty"`
	Pagination *PageInfo  `json:"pagination,omitempty"`
}

// PageInfo contains pagination information
type PageInfo struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasMore    bool  `json:"has_more"`
}

// PaginatedResponse is a helper for paginated data responses
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Pagination *PageInfo   `json:"pagination"`
}

// Error codes for standardized error handling
const (
	// Client errors (4xx)
	ErrCodeBadRequest          = "BAD_REQUEST"
	ErrCodeValidationFailed    = "VALIDATION_FAILED"
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeForbidden           = "FORBIDDEN"
	ErrCodeNotFound            = "NOT_FOUND"
	ErrCodeConflict            = "CONFLICT"
	ErrCodeTooManyRequests     = "TOO_MANY_REQUESTS"
	
	// Server errors (5xx)
	ErrCodeInternalError       = "INTERNAL_ERROR"
	ErrCodeDatabaseError       = "DATABASE_ERROR"
	ErrCodeExternalServiceError = "EXTERNAL_SERVICE_ERROR"
	ErrCodeTimeout             = "TIMEOUT"
	ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
)