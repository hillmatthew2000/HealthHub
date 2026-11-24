package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

func login(e *env, email, password string) *models.AuthResponse {
	e.t.Helper()
	w := e.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": email, "password": password})
	wantStatus(e.t, w, http.StatusOK)
	resp := decode[models.AuthResponse](e.t, w)
	return &resp
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	u := e.addUser("nurse@example.org", auth.RoleNurse)

	// Email addresses are matched case-insensitively.
	resp := login(e, "Nurse@Example.ORG", testPassword)
	if resp.User.ID != u.ID || resp.User.Email != u.Email {
		t.Errorf("user = %+v, want %s", resp.User, u.ID)
	}
	p, err := e.tokens.Verify(resp.Token)
	if err != nil {
		t.Fatalf("issued token does not verify: %v", err)
	}
	if p.UserID != u.ID || len(p.Roles) != 1 || p.Roles[0] != auth.RoleNurse {
		t.Errorf("principal = %+v", p)
	}
	if strings.Contains(e.do(http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"email": u.Email, "password": testPassword}).Body.String(), "passwordHash") {
		t.Error("login response exposes the password hash")
	}
	stored, _ := e.users.GetByEmail(t.Context(), u.Email)
	if stored.LastLoginAt == nil {
		t.Error("last login time was not recorded")
	}
}

func TestLoginFailures(t *testing.T) {
	e := newEnv(t)
	u := e.addUser("someone@example.org", auth.RolePractitioner)
	inactive := e.addUser("gone@example.org", auth.RolePractitioner)
	active := false
	if _, err := e.users.Update(t.Context(), mustUUID(t, inactive.ID), storeUpdate(nil, &active)); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		body     any
		status   int
		code     string
		fields   []string
		sameBody bool
	}{
		{name: "wrong password", body: map[string]string{"email": u.Email, "password": "not the password"},
			status: http.StatusUnauthorized, code: problem.CodeInvalidCredentials, sameBody: true},
		{name: "unknown account", body: map[string]string{"email": "nobody@example.org", "password": testPassword},
			status: http.StatusUnauthorized, code: problem.CodeInvalidCredentials, sameBody: true},
		{name: "deactivated account", body: map[string]string{"email": inactive.Email, "password": testPassword},
			status: http.StatusUnauthorized, code: problem.CodeInvalidCredentials, sameBody: true},
		{name: "malformed JSON", body: `{"email":`,
			status: http.StatusBadRequest, code: problem.CodeInvalidRequest},
		{name: "empty body", body: "",
			status: http.StatusBadRequest, code: problem.CodeInvalidRequest},
		{name: "wrong JSON types", body: `{"email": 42, "password": true}`,
			status: http.StatusBadRequest, code: problem.CodeInvalidRequest},
		{name: "missing fields", body: map[string]string{},
			status: http.StatusUnprocessableEntity, code: problem.CodeValidationFailed, fields: []string{"email", "password"}},
		{name: "invalid email", body: map[string]string{"email": "not-an-email", "password": testPassword},
			status: http.StatusUnprocessableEntity, code: problem.CodeValidationFailed, fields: []string{"email"}},
	}
	var detail string
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(http.MethodPost, "/api/v1/auth/login", "", tt.body)
			p := wantProblem(t, w, tt.status, tt.code)
			if tt.fields != nil {
				wantFields(t, p, tt.fields...)
			}
			// Every credential failure must look the same, so a caller
			// cannot tell which accounts exist or are deactivated.
			if tt.sameBody {
				if detail == "" {
					detail = p.Detail
				} else if p.Detail != detail {
					t.Errorf("detail %q differs from %q", p.Detail, detail)
				}
			}
		})
	}
}

func TestRegister(t *testing.T) {
	body := map[string]string{
		"email": "New.User@Example.org", "password": testPassword, "firstName": " New ", "lastName": "User",
	}

	t.Run("disabled by default", func(t *testing.T) {
		e := newEnv(t)
		w := e.do(http.MethodPost, "/api/v1/auth/register", "", body)
		wantProblem(t, w, http.StatusForbidden, problem.CodeRegistrationDisabled)
		if n, _, _ := e.users.List(t.Context(), storePage(10)); len(n) != 0 {
			t.Error("an account was created while registration is disabled")
		}
	})

	t.Run("creates an account without roles", func(t *testing.T) {
		e := newEnv(t, withRegistration())
		w := e.do(http.MethodPost, "/api/v1/auth/register", "", body)
		wantStatus(t, w, http.StatusCreated)
		resp := decode[models.AuthResponse](t, w)
		if resp.User.Email != "new.user@example.org" || resp.User.FirstName != "New" {
			t.Errorf("user = %+v", resp.User)
		}
		if len(resp.User.Roles) != 0 {
			t.Errorf("self-registered user received roles %v", resp.User.Roles)
		}
		// A role-less account can authenticate but not read patients.
		w = e.do(http.MethodGet, "/api/v1/patients", resp.Token, nil)
		wantProblem(t, w, http.StatusForbidden, problem.CodeForbidden)

		w = e.do(http.MethodPost, "/api/v1/auth/register", "", body)
		wantProblem(t, w, http.StatusConflict, problem.CodeConflict)
	})

	t.Run("validation", func(t *testing.T) {
		e := newEnv(t, withRegistration())
		w := e.do(http.MethodPost, "/api/v1/auth/register", "", map[string]string{
			"email": "x@example.org", "password": "short", "firstName": "", "lastName": strings.Repeat("x", 101),
		})
		p := wantProblem(t, w, http.StatusUnprocessableEntity, problem.CodeValidationFailed)
		wantFields(t, p, "password", "firstName", "lastName")
		for _, fe := range p.Errors {
			if strings.Contains(fe.Message, "short") {
				t.Errorf("field error echoes the rejected value: %+v", fe)
			}
		}
	})
}

func TestAuthenticationRequired(t *testing.T) {
	e := newEnv(t)
	u := e.addUser("p@example.org", auth.RolePractitioner)
	other, err := auth.NewTokenManager(auth.TokenConfig{
		Secret: []byte(strings.Repeat("x", 32)), Issuer: "healthhub", Audience: "healthhub-api", TTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	forged, _, err := other.Issue(auth.Principal{UserID: u.ID, Roles: []auth.Role{auth.RoleAdmin}})
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{"/api/v1/auth/profile", "/api/v1/patients", "/api/v1/observations"}
	for _, path := range paths {
		w := e.do(http.MethodGet, path, "", nil)
		wantProblem(t, w, http.StatusUnauthorized, problem.CodeUnauthorized)
		if !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("%s: missing WWW-Authenticate challenge", path)
		}

		for _, tok := range []string{"garbage", forged} {
			w = e.do(http.MethodGet, path, tok, nil)
			wantProblem(t, w, http.StatusUnauthorized, problem.CodeInvalidToken)
		}
	}

	// Only the bearer scheme is accepted.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
	req.Header.Set("Authorization", "Basic "+e.token(u))
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	wantProblem(t, w, http.StatusUnauthorized, problem.CodeUnauthorized)
}

func TestProfileAndRefresh(t *testing.T) {
	e := newEnv(t)
	u := e.addUser("lab@example.org", auth.RoleLabTech)
	tok := e.token(u)

	w := e.do(http.MethodGet, "/api/v1/auth/profile", tok, nil)
	wantStatus(t, w, http.StatusOK)
	if info := decode[models.UserInfo](t, w); info.ID != u.ID || info.Email != u.Email {
		t.Errorf("profile = %+v", info)
	}

	// Refreshing re-reads the account, so role changes take effect.
	roles := []string{string(auth.RolePractitioner)}
	if _, err := e.users.Update(t.Context(), mustUUID(t, u.ID), storeUpdate(&roles, nil)); err != nil {
		t.Fatal(err)
	}
	w = e.do(http.MethodPost, "/api/v1/auth/refresh", tok, nil)
	wantStatus(t, w, http.StatusOK)
	p, err := e.tokens.Verify(decode[models.AuthResponse](t, w).Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Roles) != 1 || p.Roles[0] != auth.RolePractitioner {
		t.Errorf("refreshed roles = %v", p.Roles)
	}

	// A token for a deactivated account no longer refreshes.
	active := false
	if _, err := e.users.Update(t.Context(), mustUUID(t, u.ID), storeUpdate(nil, &active)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/auth/refresh", "/api/v1/auth/profile"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "refresh") {
			method = http.MethodPost
		}
		wantProblem(t, e.do(method, path, tok, nil), http.StatusUnauthorized, problem.CodeInvalidToken)
	}
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t)
	u := e.addUser("doc@example.org", auth.RolePractitioner)
	tok := e.token(u)
	const newPassword = "a much longer passphrase"

	w := e.do(http.MethodPost, "/api/v1/auth/change-password", tok,
		map[string]string{"currentPassword": "wrong password!", "newPassword": newPassword})
	p := wantProblem(t, w, http.StatusUnprocessableEntity, problem.CodeValidationFailed)
	wantFields(t, p, "currentPassword")

	w = e.do(http.MethodPost, "/api/v1/auth/change-password", tok,
		map[string]string{"currentPassword": testPassword, "newPassword": "short"})
	p = wantProblem(t, w, http.StatusUnprocessableEntity, problem.CodeValidationFailed)
	wantFields(t, p, "newPassword")

	w = e.do(http.MethodPost, "/api/v1/auth/change-password", tok,
		map[string]string{"currentPassword": testPassword, "newPassword": newPassword})
	wantStatus(t, w, http.StatusNoContent)

	login(e, u.Email, newPassword)
	w = e.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": u.Email, "password": testPassword})
	wantProblem(t, w, http.StatusUnauthorized, problem.CodeInvalidCredentials)
}

func TestStoreFailureIsOpaque(t *testing.T) {
	e := newEnv(t)
	e.users.err = errBoom
	w := e.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "a@example.org", "password": testPassword})
	p := wantProblem(t, w, http.StatusInternalServerError, problem.CodeInternal)
	if strings.Contains(w.Body.String(), "SELECT") || strings.Contains(p.Detail, "connection") {
		t.Errorf("internal error leaked: %s", w.Body.String())
	}
}
