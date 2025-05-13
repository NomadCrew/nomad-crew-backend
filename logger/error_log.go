// Package logger provides logging utilities for the application.
package logger

import (
	"context"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorLog contains structured information about an error occurrence
type ErrorLog struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	ErrorType   string                 `json:"error_type,omitempty"`
	StatusCode  int                    `json:"status_code,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Method      string                 `json:"method,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	CodeContext string                 `json:"code_context,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LogError logs a detailed error with contextual information
func LogError(ctx context.Context, err error, message string, metadata map[string]interface{}) {
	log := GetLogger()

	// Create structured error log
	errorLog := ErrorLog{
		Timestamp: time.Now().UTC(),
		Level:     "error",
		Message:   message,
		Metadata:  metadata,
	}

	if err != nil {
		errorLog.ErrorType = getErrorType(err)
	}

	// Add stack trace in non-production environments
	if os.Getenv("ENVIRONMENT") != "production" {
		errorLog.StackTrace = getStackTrace(2) // Skip the first 2 frames (this function and its caller)
	}

	// Extract additional context for HTTP requests
	if ginCtx, ok := ctx.(*gin.Context); ok {
		errorLog.RequestID = ginCtx.GetString("request_id")
		errorLog.UserID = ginCtx.GetString("user_id")
		errorLog.Path = ginCtx.Request.URL.Path
		errorLog.Method = ginCtx.Request.Method
		errorLog.IPAddress = ginCtx.ClientIP()

		// Extract status code if set
		if ginCtx.Writer.Status() != 0 {
			errorLog.StatusCode = ginCtx.Writer.Status()
		}
	}

	fields := []zap.Field{
		zap.Error(err),
		zap.String("error_type", errorLog.ErrorType),
	}

	if errorLog.RequestID != "" {
		fields = append(fields, zap.String("request_id", errorLog.RequestID))
	}

	if errorLog.UserID != "" {
		fields = append(fields, zap.String("user_id", errorLog.UserID))
	}

	if errorLog.Path != "" {
		fields = append(fields, zap.String("path", errorLog.Path))
		fields = append(fields, zap.String("method", errorLog.Method))
	}

	if errorLog.IPAddress != "" {
		fields = append(fields, zap.String("ip_address", errorLog.IPAddress))
	}

	if errorLog.StatusCode != 0 {
		fields = append(fields, zap.Int("status_code", errorLog.StatusCode))
	}

	if errorLog.StackTrace != "" {
		fields = append(fields, zap.String("stack_trace", errorLog.StackTrace))
	}

	// Add any additional metadata
	for k, v := range metadata {
		fields = append(fields, zap.Any(k, v))
	}

	// Log the error
	log.Desugar().Error(message, fields...)
}

// LogHTTPError logs an HTTP request error with context from a gin.Context
func LogHTTPError(c *gin.Context, err error, statusCode int, message string) {
	userID, _ := c.Get("user_id")
	requestID, _ := c.Get("request_id")

	metadata := map[string]interface{}{
		"status_code": statusCode,
		"path":        c.Request.URL.Path,
		"method":      c.Request.Method,
		"client_ip":   c.ClientIP(),
		"headers":     filterSensitiveHeaders(c.Request.Header),
	}

	if userID != nil {
		metadata["user_id"] = userID
	}

	if requestID != nil {
		metadata["request_id"] = requestID
	}

	LogError(c, err, message, metadata)
}

// getErrorType extracts a clean type name from an error
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Get the type of the error
	errType := runtime.FuncForPC(reflect.ValueOf(err).Pointer()).Name()

	// Clean up the type name
	parts := strings.Split(errType, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return errType
}

// getStackTrace captures a stack trace starting from the specified skip level
func getStackTrace(skip int) string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var builder strings.Builder
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.Function, "runtime.") {
			builder.WriteString(frame.Function)
			builder.WriteString("\n\t")
			builder.WriteString(frame.File)
			builder.WriteString(":")
			builder.WriteString(strconv.Itoa(frame.Line))
			builder.WriteString("\n")
		}
		if !more {
			break
		}
	}

	return builder.String()
}

// filterSensitiveHeaders removes sensitive information from headers before logging
func filterSensitiveHeaders(headers http.Header) map[string]string {
	filtered := make(map[string]string)

	for name, values := range headers {
		// Skip sensitive headers entirely
		if strings.EqualFold(name, "Authorization") ||
			strings.EqualFold(name, "Cookie") ||
			strings.Contains(strings.ToLower(name), "token") ||
			strings.Contains(strings.ToLower(name), "key") ||
			strings.Contains(strings.ToLower(name), "secret") {
			filtered[name] = "[REDACTED]"
			continue
		}

		// Include other headers
		if len(values) > 0 {
			filtered[name] = values[0]
		}
	}

	return filtered
}
