package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

// Init initializes the Zap logger
func Init() {
	var config zap.Config
	
	if os.Getenv("APP_ENV") == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		
		// "Premium" Developer Experience Look
		config.EncoderConfig.EncodeLevel = zapcore.LowercaseColorLevelEncoder // Softer colors
		config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")
		config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		config.EncoderConfig.ConsoleSeparator = "|" 
		config.EncoderConfig.EncodeName = zapcore.FullNameEncoder
	}

	var err error
	Log, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
}

// Sync flushes any buffered log entries
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// Info logs a message at Info level
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Error logs a message at Error level
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a message at Fatal level
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// Warn logs a message at Warn level
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Debug logs a message at Debug level
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}
