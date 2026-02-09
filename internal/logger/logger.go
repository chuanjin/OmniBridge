package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Init initializes the global logger.
// If debug is true, it uses a development config (console encoder, debug level).
// Otherwise, it uses a production config (JSON encoder, info level).
func Init(debug bool) error {
	var err error
	once.Do(func() {
		var config zap.Config
		if debug {
			config = zap.NewDevelopmentConfig()
			config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		} else {
			config = zap.NewProductionConfig()
			config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		}

		// Customize output to stdout/stderr or file if needed
		// For now, we stick to stdout/stderr which is container-friendly
		globalLogger, err = config.Build(zap.AddCallerSkip(1)) // Skip 1 caller level for wrapper functions if we had them
	})
	return err
}

// Get returns the global logger.
// It initializes a default production logger if Init hasn't been called.
func Get() *zap.Logger {
	if globalLogger == nil {
		// Fallback to a basic production logger if not initialized
		l, _ := zap.NewProduction()
		return l
	}
	return globalLogger
}

// Sync flushes any buffered log entries.
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// Named returns a logger with a specific name.
func Named(name string) *zap.Logger {
	return Get().Named(name)
}

// Info logs a message at InfoLevel.
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Error logs a message at ErrorLevel.
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Debug logs a message at DebugLevel.
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Warn logs a message at WarnLevel.
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Fatal logs a message at FatalLevel.
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}
