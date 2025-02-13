package types

// PaginationParams represents common pagination request parameters
type PaginationParams struct {
    Limit  int `json:"limit" form:"limit,default=20"`
    Offset int `json:"offset" form:"offset,default=0"`
}

// PaginationResponse represents the pagination metadata in responses
type PaginationResponse struct {
    Limit  int `json:"limit"`
    Offset int `json:"offset"`
    Total  int `json:"total"`
}

// PaginatedResponse is a generic wrapper for paginated data
type PaginatedResponse[T any] struct {
    Data       T                 `json:"data"`
    Pagination PaginationResponse `json:"pagination"`
}

// response.go
package types

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
    Error   string `json:"error"`   // Error type/code
    Message string `json:"message"` // Human-readable message
    Code    string `json:"code"`    // Application-specific error code
}

// ValidationError represents a validation error response
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

// ValidationErrorResponse represents a response containing validation errors
type ValidationErrorResponse struct {
    Error    string           `json:"error"`
    Message  string           `json:"message"`
    Details  []ValidationError `json:"details"`
}

// SuccessResponse represents a simple success response
type SuccessResponse struct {
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}