// Package config loads and validates the service configuration from
// environment variables.
package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hillmatthew2000/HealthHub/pkg/encryption"
)

// Environment names recognised by the service.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Config holds all runtime configuration.
type Config struct {
	Environment string
	LogLevel    string

	// HTTP server
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
	MaxBodyBytes    int64
	TrustedProxies  []string
	AllowedOrigins  []string
	DocsEnabled     bool
	MetricsEnabled  bool

	// TLS (optional; usually terminated by the ingress)
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	// Database
	DatabaseURL       string
	DBMaxConns        int32
	DBMinConns        int32
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
	// AutoMigrate applies pending migrations when the server starts.
	AutoMigrate bool

	// Authentication
	JWTSecret         string
	JWTIssuer         string
	JWTAudience       string
	JWTTTL            time.Duration
	BcryptCost        int
	AllowRegistration bool

	// EncryptionKey is the master key for field-level PHI encryption,
	// base64 encoded (32 bytes decoded) or a raw string of at least 32 bytes.
	EncryptionKey string

	// Rate limiting (per client IP)
	RateLimitEnabled bool
	RateLimitRPM     int
	AuthRateLimitRPM int

	// Pagination
	DefaultPageSize int
	MaxPageSize     int
}

// Load reads the configuration from the process environment and validates it.
func Load() (*Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom reads configuration using the given lookup function, which makes
// it possible to test without touching the process environment.
func LoadFrom(lookup func(string) (string, bool)) (*Config, error) {
	r := reader{lookup: lookup}

	cfg := &Config{
		Environment: strings.ToLower(r.str("ENVIRONMENT", EnvDevelopment)),
		LogLevel:    strings.ToLower(r.str("LOG_LEVEL", "info")),

		Port:            r.str("PORT", "8080"),
		ReadTimeout:     r.duration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    r.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:     r.duration("HTTP_IDLE_TIMEOUT", 120*time.Second),
		ShutdownTimeout: r.duration("SHUTDOWN_TIMEOUT", 20*time.Second),
		RequestTimeout:  r.duration("REQUEST_TIMEOUT", 10*time.Second),
		MaxBodyBytes:    int64(r.integer("MAX_BODY_BYTES", 1<<20)),
		TrustedProxies:  r.list("TRUSTED_PROXIES", nil),
		AllowedOrigins:  r.list("ALLOWED_ORIGINS", nil),
		DocsEnabled:     r.boolean("DOCS_ENABLED", true),
		MetricsEnabled:  r.boolean("METRICS_ENABLED", true),

		TLSEnabled:  r.boolean("TLS_ENABLED", false),
		TLSCertFile: r.str("TLS_CERT_FILE", ""),
		TLSKeyFile:  r.str("TLS_KEY_FILE", ""),

		DatabaseURL:       r.str("DATABASE_URL", ""),
		DBMaxConns:        r.int32("DB_MAX_CONNS", 20),
		DBMinConns:        r.int32("DB_MIN_CONNS", 2),
		DBConnMaxLifetime: r.duration("DB_CONN_MAX_LIFETIME", time.Hour),
		DBConnMaxIdleTime: r.duration("DB_CONN_MAX_IDLE_TIME", 15*time.Minute),
		AutoMigrate:       r.boolean("DB_AUTO_MIGRATE", true),

		JWTSecret:         r.str("JWT_SECRET", ""),
		JWTIssuer:         r.str("JWT_ISSUER", "healthhub"),
		JWTAudience:       r.str("JWT_AUDIENCE", "healthhub-api"),
		JWTTTL:            r.duration("JWT_TTL", time.Hour),
		BcryptCost:        r.integer("BCRYPT_COST", 12),
		AllowRegistration: r.boolean("ALLOW_REGISTRATION", false),

		EncryptionKey: r.str("ENCRYPTION_KEY", ""),

		RateLimitEnabled: r.boolean("RATE_LIMIT_ENABLED", true),
		RateLimitRPM:     r.integer("RATE_LIMIT_RPM", 300),
		AuthRateLimitRPM: r.integer("AUTH_RATE_LIMIT_RPM", 10),

		DefaultPageSize: r.integer("DEFAULT_PAGE_SIZE", 20),
		MaxPageSize:     r.integer("MAX_PAGE_SIZE", 100),
	}

	if err := errors.Join(append(r.errs, cfg.Validate())...); err != nil {
		return nil, err
	}
	return cfg, nil
}

// IsDevelopment reports whether the service runs in development mode.
func (c *Config) IsDevelopment() bool { return c.Environment == EnvDevelopment }

// IsProduction reports whether the service runs in production mode.
func (c *Config) IsProduction() bool { return c.Environment == EnvProduction }

// Addr returns the listen address for the HTTP server.
func (c *Config) Addr() string { return ":" + c.Port }

// knownExampleSecrets are values that appear in documentation and sample
// manifests. They must never be accepted outside development.
var knownExampleSecrets = []string{
	"your-super-secret-jwt-key-change-in-production",
	"your-super-secret-jwt-key-change-in-production-make-it-at-least-32-chars",
	"your-32-byte-encryption-key-change-this",
	"your-32-byte-encryption-key-change-this-in-production",
	"your-32-byte-encryption-key-change-this-in-production-1234",
	"dev-only-jwt-secret-do-not-use-in-production-0123456789",
}

// Validate checks that the configuration is complete and safe to run with.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	switch c.Environment {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
	default:
		add("ENVIRONMENT must be one of development, test, staging, production (got %q)", c.Environment)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		add("LOG_LEVEL must be one of debug, info, warn, error (got %q)", c.LogLevel)
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		add("PORT must be a valid TCP port (got %q)", c.Port)
	}

	if c.DatabaseURL == "" {
		add("DATABASE_URL is required")
	}
	if c.DBMaxConns < 1 {
		add("DB_MAX_CONNS must be at least 1")
	}
	if c.DBMinConns < 0 || c.DBMinConns > c.DBMaxConns {
		add("DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
	}

	if len(c.JWTSecret) < 32 {
		add("JWT_SECRET is required and must be at least 32 bytes")
	}
	if c.JWTTTL < time.Minute || c.JWTTTL > 24*time.Hour {
		add("JWT_TTL must be between 1m and 24h (got %s)", c.JWTTTL)
	}
	if c.BcryptCost < 10 || c.BcryptCost > 16 {
		add("BCRYPT_COST must be between 10 and 16 (got %d)", c.BcryptCost)
	}

	if c.EncryptionKey == "" {
		add("ENCRYPTION_KEY is required")
	} else if _, err := encryption.ParseKey(c.EncryptionKey); err != nil {
		add("ENCRYPTION_KEY is invalid: %w", err)
	}

	if !c.IsDevelopment() && c.Environment != EnvTest {
		for _, s := range knownExampleSecrets {
			if c.JWTSecret == s {
				add("JWT_SECRET is a documented example value; generate a real secret")
			}
			if c.EncryptionKey == s {
				add("ENCRYPTION_KEY is a documented example value; generate a real key")
			}
		}
	}

	if c.TLSEnabled && (c.TLSCertFile == "" || c.TLSKeyFile == "") {
		add("TLS_CERT_FILE and TLS_KEY_FILE are required when TLS_ENABLED is true")
	}
	if c.MaxBodyBytes < 1024 {
		add("MAX_BODY_BYTES must be at least 1024")
	}
	if c.RequestTimeout <= 0 {
		add("REQUEST_TIMEOUT must be positive")
	}
	if c.RateLimitEnabled && (c.RateLimitRPM < 1 || c.AuthRateLimitRPM < 1) {
		add("RATE_LIMIT_RPM and AUTH_RATE_LIMIT_RPM must be positive when rate limiting is enabled")
	}
	if c.MaxPageSize < 1 || c.DefaultPageSize < 1 || c.DefaultPageSize > c.MaxPageSize {
		add("page sizes must satisfy 1 <= DEFAULT_PAGE_SIZE <= MAX_PAGE_SIZE")
	}
	for _, o := range c.AllowedOrigins {
		if o == "*" && len(c.AllowedOrigins) > 1 {
			add("ALLOWED_ORIGINS cannot mix \"*\" with explicit origins")
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
}

// reader collects parse errors instead of silently falling back to defaults,
// so a typo such as RATE_LIMIT_RPM=10O is reported at startup.
type reader struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (r *reader) raw(key string) (string, bool) {
	v, ok := r.lookup(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (r *reader) str(key, def string) string {
	if v, ok := r.raw(key); ok {
		return v
	}
	return def
}

func (r *reader) integer(key string, def int) int {
	v, ok := r.raw(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s must be an integer (got %q)", key, v))
		return def
	}
	return n
}

func (r *reader) int32(key string, def int32) int32 {
	n := r.integer(key, int(def))
	if n < math.MinInt32 || n > math.MaxInt32 {
		r.errs = append(r.errs, fmt.Errorf("%s is out of range (got %d)", key, n))
		return def
	}
	return int32(n)
}

func (r *reader) boolean(key string, def bool) bool {
	v, ok := r.raw(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s must be a boolean (got %q)", key, v))
		return def
	}
	return b
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v, ok := r.raw(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s must be a duration such as 30s or 5m (got %q)", key, v))
		return def
	}
	return d
}

func (r *reader) list(key string, def []string) []string {
	v, ok := r.raw(key)
	if !ok {
		return def
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
