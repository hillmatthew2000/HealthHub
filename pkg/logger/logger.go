package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// Init initializes the logger with the specified level
func Init(level string) {
	config := zap.NewProductionConfig()

	// Set log level
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	// Configure encoding
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Add file output for production
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build(zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		panic(err)
	}

	Logger = logger
}

// InitDevelopment initializes the logger for development with human-readable output
func InitDevelopment() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := config.Build(zap.AddCaller())
	if err != nil {
		panic(err)
	}

	Logger = logger
}

// Info logs an info message with optional fields
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Error logs an error message with optional fields
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Debug logs a debug message with optional fields
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Warn logs a warning message with optional fields
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Fatal logs a fatal message and exits the program
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// Panic logs a panic message and panics
func Panic(msg string, fields ...zap.Field) {
	Logger.Panic(msg, fields...)
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// WithFields creates a logger with predefined fields
func WithFields(fields ...zap.Field) *zap.Logger {
	return Logger.With(fields...)
}

// GetLogger returns the global logger instance
func GetLogger() *zap.Logger {
	return Logger
}

// SetLogger sets a custom logger instance
func SetLogger(logger *zap.Logger) {
	Logger = logger
}

// HTTPLogger returns a logger specifically for HTTP requests
func HTTPLogger() *zap.Logger {
	return Logger.Named("http")
}

// DatabaseLogger returns a logger specifically for database operations
func DatabaseLogger() *zap.Logger {
	return Logger.Named("database")
}

// AuthLogger returns a logger specifically for authentication
func AuthLogger() *zap.Logger {
	return Logger.Named("auth")
}

// SecurityLogger returns a logger specifically for security events
func SecurityLogger() *zap.Logger {
	return Logger.Named("security")
}

// AuditLogger returns a logger specifically for audit events
func AuditLogger() *zap.Logger {
	return Logger.Named("audit")
}

// LogSecurityEvent logs a security-related event
func LogSecurityEvent(event string, userID string, details map[string]interface{}) {
	fields := []zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID),
	}

	for key, value := range details {
		fields = append(fields, zap.Any(key, value))
	}

	SecurityLogger().Info("Security event", fields...)
}

// LogAuditEvent logs an audit event
func LogAuditEvent(action string, resource string, userID string, details map[string]interface{}) {
	fields := []zap.Field{
		zap.String("action", action),
		zap.String("resource", resource),
		zap.String("user_id", userID),
	}

	for key, value := range details {
		fields = append(fields, zap.Any(key, value))
	}

	AuditLogger().Info("Audit event", fields...)
}

// LogHTTPRequest logs an HTTP request
func LogHTTPRequest(method string, path string, statusCode int, duration int64, userID string) {
	HTTPLogger().Info("HTTP request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", statusCode),
		zap.Int64("duration_ms", duration),
		zap.String("user_id", userID),
	)
}

// LogDatabaseOperation logs a database operation
func LogDatabaseOperation(operation string, table string, userID string, duration int64, err error) {
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.String("table", table),
		zap.String("user_id", userID),
		zap.Int64("duration_ms", duration),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		DatabaseLogger().Error("Database operation failed", fields...)
	} else {
		DatabaseLogger().Info("Database operation", fields...)
	}
}
