package logger

import (
	"go.uber.org/zap"
	"sync"
)

var (
    logger *zap.SugaredLogger
    once   sync.Once
)

func InitLogger() {
    once.Do(func() {
        zapLogger, _ := zap.NewProduction()
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
	if logger != nil {
		err := logger.Sync()
		if err != nil {
			logger.Error("Error syncing logger:", err)
		}
		return err
	}
	return nil
}
