package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application
type Config struct {
	// Server configuration
	Port        string
	Environment string
	LogLevel    string

	// Database configuration
	DatabaseURL string

	// Security configuration
	JWTSecret     string
	EncryptionKey string

	// Redis configuration
	RedisURL string

	// CORS configuration
	AllowedOrigins []string

	// Rate limiting
	RateLimitEnabled bool
	RateLimitRPM     int

	// TLS configuration
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	// Health check configuration
	HealthCheckPath string

	// Pagination defaults
	DefaultPageSize int
	MaxPageSize     int
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		// Server configuration
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// Database configuration
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://localhost:5432/healthcare_api?sslmode=disable"),

		// Security configuration
		JWTSecret:     getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production"),
		EncryptionKey: getEnv("ENCRYPTION_KEY", "your-32-byte-encryption-key-change-this"),

		// Redis configuration
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379"),

		// CORS configuration
		AllowedOrigins: getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),

		// Rate limiting
		RateLimitEnabled: getEnvAsBool("RATE_LIMIT_ENABLED", true),
		RateLimitRPM:     getEnvAsInt("RATE_LIMIT_RPM", 100),

		// TLS configuration
		TLSEnabled:  getEnvAsBool("TLS_ENABLED", false),
		TLSCertFile: getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", ""),

		// Health check configuration
		HealthCheckPath: getEnv("HEALTH_CHECK_PATH", "/health"),

		// Pagination defaults
		DefaultPageSize: getEnvAsInt("DEFAULT_PAGE_SIZE", 10),
		MaxPageSize:     getEnvAsInt("MAX_PAGE_SIZE", 100),
	}
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvAsInt gets an environment variable as an integer with a fallback value
func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

// getEnvAsBool gets an environment variable as a boolean with a fallback value
func getEnvAsBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return fallback
}

// getEnvAsSlice gets an environment variable as a slice with a fallback value
func getEnvAsSlice(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return fallback
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsTest returns true if running in test mode
func (c *Config) IsTest() bool {
	return c.Environment == "test"
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check required fields
	if c.DatabaseURL == "" {
		return NewConfigError("DATABASE_URL is required")
	}

	if c.JWTSecret == "" {
		return NewConfigError("JWT_SECRET is required")
	}

	if len(c.JWTSecret) < 32 {
		return NewConfigError("JWT_SECRET must be at least 32 characters long")
	}

	if c.EncryptionKey == "" {
		return NewConfigError("ENCRYPTION_KEY is required")
	}

	if len(c.EncryptionKey) != 32 {
		return NewConfigError("ENCRYPTION_KEY must be exactly 32 characters long")
	}

	if c.TLSEnabled && (c.TLSCertFile == "" || c.TLSKeyFile == "") {
		return NewConfigError("TLS_CERT_FILE and TLS_KEY_FILE are required when TLS is enabled")
	}

	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

// NewConfigError creates a new configuration error
func NewConfigError(message string) *ConfigError {
	return &ConfigError{Message: message}
}

// Error returns the error message
func (e *ConfigError) Error() string {
	return "Configuration error: " + e.Message
}
