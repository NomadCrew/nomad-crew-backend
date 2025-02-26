package logger

import (
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.SugaredLogger
	once   sync.Once
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
		logger = zapLogger.Sugar()
	})
}

func GetLogger() *zap.SugaredLogger {
	if logger == nil {
		InitLogger() // Ensure initialization if not already done
	}
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
