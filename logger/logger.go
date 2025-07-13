// Package logger provides a configured Zap sugared logger instance for the application.
// It handles initialization based on environment variables (LOG_LEVEL, ENVIRONMENT)
// and provides utility functions for masking sensitive data in logs.
package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.SugaredLogger
	once   sync.Once
)

// IsTest should be set to true when running in a test environment to adjust
// logger configuration (e.g., outputting to stdout without cloudwatch).
var IsTest bool

// initLoggerInternal sets up the global zap.SugaredLogger based on environment.
// It configures levels and outputs differently for test, production, and development.
func initLoggerInternal() {
	var zapLogger *zap.Logger
	var err error

	// Determine log level from the environment (default to info)
	levelStr := os.Getenv("LOG_LEVEL")
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		// Default to info level if parsing fails or LOG_LEVEL is not set.
		level = zapcore.InfoLevel
	}

	if IsTest {
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		config.OutputPaths = []string{"stdout"} // Test output to stdout
		zapLogger, err = config.Build()
	} else if os.Getenv("ENVIRONMENT") == "production" {
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(level)
		// TODO: Configure actual cloudwatch paths if needed
		cfg.OutputPaths = []string{"stdout"}      // Example: "stdout", "cloudwatch:///nomadcrew-logs"
		cfg.ErrorOutputPaths = []string{"stderr"} // Example: "stderr", "cloudwatch:///nomadcrew-errors"
		zapLogger, err = cfg.Build()
	} else {
		// Use development config for non-production, non-test environments.
		devCfg := zap.NewDevelopmentConfig()
		devCfg.Level = zap.NewAtomicLevelAt(level)
		zapLogger, err = devCfg.Build()
	}

	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	logger = zapLogger.Sugar()
}

// InitLogger initializes the global logger instance using sync.Once to ensure
// it's done only once, making it safe for concurrent calls.
func InitLogger() {
	once.Do(initLoggerInternal)
}

// GetLogger returns the shared global zap.SugaredLogger instance.
// It ensures the logger is initialized before returning it.
func GetLogger() *zap.SugaredLogger {
	// Ensure the logger is initialized using sync.Once.
	once.Do(initLoggerInternal)
	return logger
}

// Close syncs the global logger to flush any buffered log entries.
// It should be called before the application exits.
// Returns an error if syncing fails.
func Close() error {
	if logger != nil && !IsTest {
		err := logger.Sync()
		if err != nil {
			// Use fmt.Println or os.Stderr here to avoid potential loops if logger.Sync fails
			fmt.Fprintf(os.Stderr, "Error syncing logger: %v\n", err)
		}
		return err
	}
	return nil
}

// MaskSensitiveString masks the middle part of a string, showing only the
// first prefixLen and last suffixLen characters. Used for logging sensitive data.
func MaskSensitiveString(s string, prefixLen, suffixLen int) string {
	if s == "" {
		return ""
	}

	// For short strings, return all asterisks to avoid revealing length.
	if len(s) < (prefixLen + suffixLen + 3) {
		return strings.Repeat("*", len(s))
	}

	prefix := s[:prefixLen]
	suffix := s[len(s)-suffixLen:]
	return prefix + "..." + suffix
}

// MaskEmail masks an email address for logging purposes.
// It masks the username part but keeps the domain visible.
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		// Fallback for invalid email format
		return MaskSensitiveString(email, 2, 2)
	}

	username := parts[0]
	domain := parts[1]

	maskedUsername := MaskSensitiveString(username, 2, 1)
	return maskedUsername + "@" + domain
}

// MaskJWT masks a JWT token for logging, showing only the first and last few characters.
func MaskJWT(token string) string {
	if token == "" {
		return ""
	}

	// For short tokens, return all asterisks.
	if len(token) < 10 {
		return strings.Repeat("*", len(token))
	}

	// Show first 3 and last 3 characters for standard JWT length.
	return token[:3] + "..." + token[len(token)-3:]
}

// MaskConnectionString attempts to mask passwords within database connection strings
// for safer logging. Supports common URL and key-value formats.
// Note: This is a best-effort masking and might not cover all connection string variations.
func MaskConnectionString(connStr string) string {
	if connStr == "" {
		return ""
	}

	masked := connStr // Start with original

	// Attempt to mask password in URL format: scheme://user:password@host...
	if idx := strings.Index(masked, "://"); idx != -1 {
		if credIdx := strings.Index(masked[idx+3:], "@"); credIdx != -1 {
			userInfo := masked[idx+3 : idx+3+credIdx]
			if passIdx := strings.Index(userInfo, ":"); passIdx != -1 {
				user := userInfo[:passIdx]
				masked = strings.Replace(masked, userInfo, user+":***", 1)
			}
		}
	}

	// Attempt to mask password in key-value format: ... password=somepass ...
	if kvIdx := strings.Index(masked, "password="); kvIdx != -1 {
		// Find the end of the password value (space or end of string)
		endIdx := strings.Index(masked[kvIdx+len("password="):], " ")
		if endIdx == -1 {
			// Password is the last part
			masked = masked[:kvIdx+len("password=")] + "***"
		} else {
			// Password is in the middle
			masked = masked[:kvIdx+len("password=")] + "***" + masked[kvIdx+len("password=")+endIdx:]
		}
	}

	return masked
}
