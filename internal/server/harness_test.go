package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/server"
	"github.com/hillmatthew2000/HealthHub/internal/store"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// testPassword satisfies the password policy.
const testPassword = "correct horse battery"

// env is a router wired to in-memory fakes.
type env struct {
	t            *testing.T
	router       http.Handler
	users        *fakeUsers
	patients     *fakePatients
	observations *fakeObservations
	audit        *fakeAuditor
	tokens       *auth.TokenManager
	passwords    *auth.PasswordHasher
}

type envOption func(*server.Options, *server.Deps)

func withRegistration() envOption {
	return func(o *server.Options, _ *server.Deps) { o.AllowRegistration = true }
}

func withMaxBody(n int64) envOption {
	return func(o *server.Options, _ *server.Deps) { o.MaxBodyBytes = n }
}

func withDBDown() envOption {
	return func(_ *server.Options, d *server.Deps) { d.DB = fakePinger{err: errBoom} }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	tokens, err := auth.NewTokenManager(auth.TokenConfig{
		Secret:   []byte(strings.Repeat("s", 32)),
		Issuer:   "healthhub",
		Audience: "healthhub-api",
		TTL:      time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The minimum bcrypt cost keeps the tests fast.
	passwords, err := auth.NewPasswordHasher(4)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{
		t:         t,
		users:     newFakeUsers(),
		patients:  newFakePatients(),
		audit:     &fakeAuditor{},
		tokens:    tokens,
		passwords: passwords,
	}
	e.observations = newFakeObservations(e.patients)

	o := server.Options{Version: "test", MaxBodyBytes: 1 << 20, RequestTimeout: 5 * time.Second}
	d := server.Deps{
		DB:           fakePinger{},
		Users:        e.users,
		Patients:     e.patients,
		Observations: e.observations,
		Tokens:       tokens,
		Passwords:    passwords,
		Audit:        e.audit,
		AuditLog:     e.audit,
	}
	for _, opt := range opts {
		opt(&o, &d)
	}
	r, err := server.New(o, d)
	if err != nil {
		t.Fatal(err)
	}
	e.router = r
	return e
}

// addUser stores an active account with the given roles and returns it.
func (e *env) addUser(email string, roles ...auth.Role) *models.User {
	e.t.Helper()
	hash, err := e.passwords.Hash(testPassword)
	if err != nil {
		e.t.Fatal(err)
	}
	u := &models.User{Email: email, PasswordHash: hash, FirstName: "Test", LastName: "User", Active: true}
	for _, r := range roles {
		u.Roles = append(u.Roles, string(r))
	}
	if err := e.users.Create(e.t.Context(), u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

// token returns a bearer token for u.
func (e *env) token(u *models.User) string {
	e.t.Helper()
	p := auth.Principal{UserID: u.ID, Email: u.Email}
	for _, r := range u.Roles {
		p.Roles = append(p.Roles, auth.Role(r))
	}
	tok, _, err := e.tokens.Issue(p)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

// tokenFor returns a token for a new user holding roles.
func (e *env) tokenFor(roles ...auth.Role) string {
	e.t.Helper()
	return e.token(e.addUser(uuid.NewString()+"@example.org", roles...))
}

// do sends a request. body may be nil, a string (sent verbatim) or a value
// encoded as JSON.
func (e *env) do(method, path, token string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		r = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

// decode unmarshals the response body into a value of type T.
func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return v
}

// wantStatus fails the test if the response status is not want.
func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}

// wantProblem checks that the response is a problem+json document with the
// given status and code, and returns it.
func wantProblem(t *testing.T, w *httptest.ResponseRecorder, status int, code string) problem.Problem {
	t.Helper()
	wantStatus(t, w, status)
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, problem.ContentType) {
		t.Errorf("Content-Type = %q, want %s", ct, problem.ContentType)
	}
	p := decode[problem.Problem](t, w)
	if p.Status != status || p.Code != code {
		t.Errorf("problem status/code = %d/%q, want %d/%q", p.Status, p.Code, status, code)
	}
	if p.RequestID == "" || p.RequestID != w.Header().Get("X-Request-ID") {
		t.Errorf("problem requestId %q does not match X-Request-ID %q", p.RequestID, w.Header().Get("X-Request-ID"))
	}
	return p
}

// wantFields checks that p lists errors for exactly the given fields.
func wantFields(t *testing.T, p problem.Problem, fields ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, fe := range p.Errors {
		got[fe.Field] = true
	}
	for _, f := range fields {
		if !got[f] {
			t.Errorf("no error for field %q in %+v", f, p.Errors)
		}
	}
	if len(got) != len(fields) {
		t.Errorf("errors = %+v, want fields %v", p.Errors, fields)
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func storeUpdate(roles *[]string, active *bool) store.UserUpdate {
	return store.UserUpdate{Roles: roles, Active: active}
}

func storePage(limit int) store.Page { return store.Page{Limit: limit} }

// Request bodies.

func patientBody() map[string]any {
	return map[string]any{
		"name":      []map[string]any{{"family": "Rivera", "given": []string{"Ana"}}},
		"gender":    "female",
		"birthDate": "1984-03-02",
		"telecom":   []map[string]any{{"system": "phone", "value": "+1 555 0100"}},
	}
}

func observationBody(patientID string) map[string]any {
	return map[string]any{
		"status":            "final",
		"code":              map[string]any{"coding": []map[string]any{{"system": "http://loinc.org", "code": "8867-4"}}},
		"subject":           map[string]any{"reference": "Patient/" + patientID},
		"effectiveDateTime": "2026-01-15T09:30:00Z",
		"valueQuantity":     map[string]any{"value": 72, "unit": "beats/minute"},
	}
}
