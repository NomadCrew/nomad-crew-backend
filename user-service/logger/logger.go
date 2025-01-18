package logger

import (
    "os"
    "sync"
    "github.com/sirupsen/logrus"
)

var (
    logger *logrus.Logger
    once   sync.Once
)

// GetLogger returns a singleton logger instance
func GetLogger() *logrus.Logger {
    once.Do(func() {
        logger = logrus.New()
        logger.Formatter = new(logrus.JSONFormatter)
        logger.Level = logrus.DebugLevel
        logger.Out = os.Stdout
    })
    return logger
}