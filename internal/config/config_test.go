package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":   "postgres://u:p@localhost:5432/db",
		"JWT_SECRET":     strings.Repeat("s", 40),
		"ENCRYPTION_KEY": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := LoadFrom(env(validEnv()))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Port != "8080" || cfg.Environment != EnvDevelopment {
		t.Errorf("unexpected defaults: port=%s env=%s", cfg.Port, cfg.Environment)
	}
	if cfg.JWTTTL != time.Hour || cfg.BcryptCost != 12 {
		t.Errorf("unexpected auth defaults: ttl=%s cost=%d", cfg.JWTTTL, cfg.BcryptCost)
	}
	if cfg.AllowRegistration {
		t.Error("registration must be disabled by default")
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("no CORS origins should be allowed by default, got %v", cfg.AllowedOrigins)
	}
}

func TestLoadParsesValues(t *testing.T) {
	e := validEnv()
	e["PORT"] = "18090"
	e["JWT_TTL"] = "15m"
	e["ALLOWED_ORIGINS"] = " https://a.example , https://b.example ,"
	e["RATE_LIMIT_ENABLED"] = "false"
	cfg, err := LoadFrom(env(e))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Addr() != ":18090" || cfg.JWTTTL != 15*time.Minute || cfg.RateLimitEnabled {
		t.Errorf("values not applied: %+v", cfg)
	}
	if got := strings.Join(cfg.AllowedOrigins, "|"); got != "https://a.example|https://b.example" {
		t.Errorf("origins = %q", got)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]string)
		wantErr string
	}{
		{"missing database", func(e map[string]string) { delete(e, "DATABASE_URL") }, "DATABASE_URL is required"},
		{"missing jwt secret", func(e map[string]string) { delete(e, "JWT_SECRET") }, "JWT_SECRET"},
		{"short jwt secret", func(e map[string]string) { e["JWT_SECRET"] = "short" }, "JWT_SECRET"},
		{"missing encryption key", func(e map[string]string) { delete(e, "ENCRYPTION_KEY") }, "ENCRYPTION_KEY is required"},
		{"short encryption key", func(e map[string]string) { e["ENCRYPTION_KEY"] = "abc" }, "ENCRYPTION_KEY is invalid"},
		{"bad integer", func(e map[string]string) { e["RATE_LIMIT_RPM"] = "10O" }, "RATE_LIMIT_RPM must be an integer"},
		{"bad bool", func(e map[string]string) { e["TLS_ENABLED"] = "yes please" }, "TLS_ENABLED must be a boolean"},
		{"bad duration", func(e map[string]string) { e["JWT_TTL"] = "1 hour" }, "JWT_TTL must be a duration"},
		{"ttl too long", func(e map[string]string) { e["JWT_TTL"] = "48h" }, "JWT_TTL must be between"},
		{"bad environment", func(e map[string]string) { e["ENVIRONMENT"] = "prod" }, "ENVIRONMENT must be one of"},
		{"bad port", func(e map[string]string) { e["PORT"] = "99999" }, "PORT must be"},
		{"tls without files", func(e map[string]string) { e["TLS_ENABLED"] = "true" }, "TLS_CERT_FILE"},
		{"page sizes", func(e map[string]string) { e["DEFAULT_PAGE_SIZE"] = "500" }, "DEFAULT_PAGE_SIZE"},
		{"wildcard mixed", func(e map[string]string) { e["ALLOWED_ORIGINS"] = "*,https://a.example" }, "cannot mix"},
		{"example secret in production", func(e map[string]string) {
			e["ENVIRONMENT"] = "production"
			e["JWT_SECRET"] = "your-super-secret-jwt-key-change-in-production-make-it-at-least-32-chars"
		}, "documented example value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEnv()
			tt.mutate(e)
			_, err := LoadFrom(env(e))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadReportsAllProblems(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "ENCRYPTION_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}
