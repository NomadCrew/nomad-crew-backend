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
	mu     sync.RWMutex // Added mutex for logger access
)

// IsTest is used to detect test environment
var IsTest bool

func InitLogger() {
	once.Do(func() {
		var zapLogger *zap.Logger
		var err error

		// Determine log level from the environment (default to info)
		levelStr := os.Getenv("LOG_LEVEL")
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(levelStr)); err != nil {
			// If parsing fails or LOG_LEVEL is empty, default to info.
			level = zapcore.InfoLevel
		}

		if IsTest {
			config := zap.NewDevelopmentConfig()
			config.Level = zap.NewAtomicLevelAt(level)
			config.OutputPaths = []string{"stdout"}
			zapLogger, err = config.Build()
		} else if os.Getenv("ENVIRONMENT") == "production" {
			cfg := zap.NewProductionConfig()
			cfg.Level = zap.NewAtomicLevelAt(level)
			cfg.OutputPaths = []string{"stdout", "cloudwatch:///nomadcrew-logs"}
			cfg.ErrorOutputPaths = []string{"stderr", "cloudwatch:///nomadcrew-errors"}
			zapLogger, err = cfg.Build()
		} else {
			// For other environments, you can choose your desired config
			// Here we use a development-style configuration.
			devCfg := zap.NewDevelopmentConfig()
			devCfg.Level = zap.NewAtomicLevelAt(level)
			zapLogger, err = devCfg.Build()
		}

		if err != nil {
			panic(fmt.Sprintf("failed to initialize logger: %v", err))
		}

		mu.Lock()
		logger = zapLogger.Sugar()
		mu.Unlock()
	})
}

func GetLogger() *zap.SugaredLogger {
	// Ensure initialization is done
	once.Do(func() {
		InitLogger()
	})

	// Get logger with read lock
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

func Close() error {
	if logger != nil && !IsTest {
		err := logger.Sync()
		if err != nil {
			logger.Error("Error syncing logger:", err)
		}
		return err
	}
	return nil
}

// MaskSensitiveString masks a sensitive string for logging
// It shows only the first and last few characters
func MaskSensitiveString(s string, prefixLen, suffixLen int) string {
	if s == "" {
		return ""
	}

	// For very short strings, just return asterisks
	if len(s) < (prefixLen + suffixLen + 3) {
		return strings.Repeat("*", len(s))
	}

	// Otherwise mask the middle part
	prefix := s[:prefixLen]
	suffix := s[len(s)-suffixLen:]
	return prefix + "..." + suffix
}

// MaskEmail masks an email address for logging
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return MaskSensitiveString(email, 2, 2)
	}

	username := parts[0]
	domain := parts[1]

	// Mask username but keep domain visible
	maskedUsername := MaskSensitiveString(username, 2, 1)
	return maskedUsername + "@" + domain
}

// MaskJWT masks a JWT token for logging
func MaskJWT(token string) string {
	if token == "" {
		return ""
	}

	// For very short tokens, just return asterisks
	if len(token) < 10 {
		return strings.Repeat("*", len(token))
	}

	// For JWT tokens, show only first 3 and last 3 characters
	return token[:3] + "..." + token[len(token)-3:]
}

// MaskConnectionString masks a database connection string for logging
func MaskConnectionString(connStr string) string {
	if connStr == "" {
		return ""
	}

	// Replace password in connection string
	// This handles formats like:
	// - "postgres://user:password@host:port/dbname"
	// - "host=host port=port user=user password=password dbname=dbname"

	// Handle URL format
	connStr = strings.Replace(connStr, "://", "://***:", 1)
	connStr = strings.Replace(connStr, ":", "***:", 1)

	// Handle key-value format
	connStr = strings.Replace(connStr, "password=", "password=***", 1)

	return connStr
}
