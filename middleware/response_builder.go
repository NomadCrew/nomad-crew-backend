package middleware

import (
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// ResponseBuilder provides methods for building standardized API responses
type ResponseBuilder struct {
	requestID string
	version   string
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder(c *gin.Context, version string) *ResponseBuilder {
	return &ResponseBuilder{
		requestID: c.GetString(string(RequestIDKey)),
		version:   version,
	}
}

// Success creates a successful response
func (rb *ResponseBuilder) Success(c *gin.Context, data interface{}) {
	response := types.StandardResponse{
		Success: true,
		Data:    data,
		Meta: &types.MetaInfo{
			RequestID: rb.requestID,
			Timestamp: time.Now().UTC(),
			Version:   rb.version,
		},
	}
	c.JSON(http.StatusOK, response)
}

// SuccessWithPagination creates a successful paginated response
func (rb *ResponseBuilder) SuccessWithPagination(c *gin.Context, items interface{}, pageInfo *types.PageInfo) {
	response := types.StandardResponse{
		Success: true,
		Data: types.PaginatedResponse{
			Items:      items,
			Pagination: pageInfo,
		},
		Meta: &types.MetaInfo{
			RequestID:  rb.requestID,
			Timestamp:  time.Now().UTC(),
			Version:    rb.version,
			Pagination: pageInfo,
		},
	}
	c.JSON(http.StatusOK, response)
}

// Created creates a resource created response
func (rb *ResponseBuilder) Created(c *gin.Context, data interface{}) {
	response := types.StandardResponse{
		Success: true,
		Data:    data,
		Meta: &types.MetaInfo{
			RequestID: rb.requestID,
			Timestamp: time.Now().UTC(),
			Version:   rb.version,
		},
	}
	c.JSON(http.StatusCreated, response)
}

// NoContent creates a no content response
func (rb *ResponseBuilder) NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error creates an error response
func (rb *ResponseBuilder) Error(c *gin.Context, err error) {
	appErr, status := rb.processError(err)
	
	response := types.StandardResponse{
		Success: false,
		Error: &types.ErrorInfo{
			Code:    appErr.Type,
			Message: appErr.Message,
			Details: appErr.Details,
			TraceID: rb.requestID,
		},
		Meta: &types.MetaInfo{
			RequestID: rb.requestID,
			Timestamp: time.Now().UTC(),
			Version:   rb.version,
		},
	}
	
	c.JSON(status, response)
}

// ValidationError creates a validation error response
func (rb *ResponseBuilder) ValidationError(c *gin.Context, message string, details map[string]interface{}) {
	response := types.StandardResponse{
		Success: false,
		Error: &types.ErrorInfo{
			Code:    types.ErrCodeValidationFailed,
			Message: message,
			Details: details,
			TraceID: rb.requestID,
		},
		Meta: &types.MetaInfo{
			RequestID: rb.requestID,
			Timestamp: time.Now().UTC(),
			Version:   rb.version,
		},
	}
	
	c.JSON(http.StatusBadRequest, response)
}

// processError converts various error types to AppError and determines HTTP status
func (rb *ResponseBuilder) processError(err error) (*errors.AppError, int) {
	// Check if it's already an AppError
	if appErr, ok := err.(*errors.AppError); ok {
		return appErr, appErr.GetHTTPStatus()
	}
	
	// Convert to AppError
	appErr := errors.InternalServerError("An unexpected error occurred", err)
	return appErr, http.StatusInternalServerError
}

// Helper functions for common responses

// SuccessMessage creates a simple success response with a message
func SuccessMessage(c *gin.Context, message string) {
	rb := NewResponseBuilder(c, "1.0")
	rb.Success(c, gin.H{"message": message})
}

// ErrorMessage creates a simple error response with a message
func ErrorMessage(c *gin.Context, status int, code string, message string) {
	response := types.StandardResponse{
		Success: false,
		Error: &types.ErrorInfo{
			Code:    code,
			Message: message,
			TraceID: c.GetString(string(RequestIDKey)),
		},
		Meta: &types.MetaInfo{
			RequestID: c.GetString(string(RequestIDKey)),
			Timestamp: time.Now().UTC(),
		},
	}
	
	c.JSON(status, response)
}