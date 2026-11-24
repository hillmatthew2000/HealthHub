package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/server"
)

func withRateLimit(perMinute, authPerMinute int) envOption {
	return func(o *server.Options, _ *server.Deps) {
		o.RateLimit = server.RateLimit{PerMinute: perMinute, AuthPerMinute: authPerMinute}
	}
}

// from sends a request from the given client address.
func (e *env) from(remote, method, path, token string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remote
	return e.serve(req, token, body)
}

func TestLoginIsRateLimited(t *testing.T) {
	e := newEnv(t, withRateLimit(100, 3))
	u := e.addUser("nurse@example.org", auth.RoleNurse)
	wrong := map[string]string{"email": u.Email, "password": "guess guess guess"}

	for range 3 {
		wantProblem(t, e.from("192.0.2.1:1000", http.MethodPost, "/api/v1/auth/login", "", wrong),
			http.StatusUnauthorized, problem.CodeInvalidCredentials)
	}
	// The limit applies to correct passwords too, or it would be no limit.
	w := e.from("192.0.2.1:1000", http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"email": u.Email, "password": testPassword})
	wantProblem(t, w, http.StatusTooManyRequests, problem.CodeRateLimited)
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	// Registration shares the budget.
	wantProblem(t, e.from("192.0.2.1:1000", http.MethodPost, "/api/v1/auth/register", "", map[string]string{}),
		http.StatusTooManyRequests, problem.CodeRateLimited)

	// Other clients and other endpoints are unaffected.
	wantStatus(t, e.from("192.0.2.2:1000", http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"email": u.Email, "password": testPassword}), http.StatusOK)
	wantStatus(t, e.from("192.0.2.1:1000", http.MethodGet, "/api/v1/auth/profile", e.token(u), nil), http.StatusOK)
}

func TestAPIIsRateLimited(t *testing.T) {
	e := newEnv(t, withRateLimit(5, 5))
	tok := e.tokenFor(auth.RoleNurse)
	for range 5 {
		wantStatus(t, e.from("198.51.100.1:1", http.MethodGet, "/api/v1/patients", tok, nil), http.StatusOK)
	}
	wantProblem(t, e.from("198.51.100.1:1", http.MethodGet, "/api/v1/patients", tok, nil),
		http.StatusTooManyRequests, problem.CodeRateLimited)
	// Health probes are never limited.
	wantStatus(t, e.from("198.51.100.1:1", http.MethodGet, "/health/live", "", nil), http.StatusOK)
}
