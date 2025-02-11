package logger

import (
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
)

var (
    logger *zap.SugaredLogger
    once   sync.Once
)

var IsTest bool

func InitLogger() {
    once.Do(func() {
        var zapLogger *zap.Logger
        var err error

        if IsTest {
            // Test configuration - no stderr sync needed
            config := zap.NewDevelopmentConfig()
            config.OutputPaths = []string{"stdout"}
            zapLogger, err = config.Build()
        } else if os.Getenv("ENVIRONMENT") == "production" {
            cfg := zap.NewProductionConfig()
            cfg.OutputPaths = []string{"stdout", "cloudwatch:///nomadcrew-logs"}
            cfg.ErrorOutputPaths = []string{"stderr", "cloudwatch:///nomadcrew-errors"}
            zapLogger, err = cfg.Build()
        } else {
            zapLogger, err = zap.NewProduction()
        }

        if err != nil {
            panic(fmt.Sprintf("failed to initialize logger: %v", err))
        }
        logger = zapLogger.Sugar()
    })
}

func Close() error {
    if logger != nil && !IsTest {  // Skip sync in test environment
        err := logger.Sync()
        if err != nil {
            logger.Error("Error syncing logger:", err)
        }
        return err
    }
    return nil
}