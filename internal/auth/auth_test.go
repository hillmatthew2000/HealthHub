package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

var testSecret = []byte(strings.Repeat("k", 32))

func newTestManager(t *testing.T) *TokenManager {
	t.Helper()
	m, err := NewTokenManager(TokenConfig{Secret: testSecret, Issuer: "healthhub", Audience: "healthhub-api", TTL: time.Hour})
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	return m
}

func TestTokenRoundTrip(t *testing.T) {
	m := newTestManager(t)
	in := Principal{UserID: "u-1", Email: "a@example.com", Roles: []Role{RoleNurse}}
	tok, exp, err := m.Issue(in)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if d := time.Until(exp); d < 59*time.Minute || d > time.Hour+time.Second {
		t.Errorf("unexpected expiry in %s", d)
	}
	got, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UserID != in.UserID || got.Email != in.Email || len(got.Roles) != 1 || got.Roles[0] != RoleNurse {
		t.Fatalf("principal = %+v, want %+v", got, in)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	m := newTestManager(t)
	issued := time.Now().Add(-2 * time.Hour)
	m.now = func() time.Time { return issued }
	tok, _, err := m.Issue(Principal{UserID: "u-1"})
	if err != nil {
		t.Fatal(err)
	}
	m.now = time.Now
	if _, err := m.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, c jwt.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyRejectsForgedTokens(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	valid := func() *claims {
		return &claims{RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "u-1",
			Issuer:    "healthhub",
			Audience:  jwt.ClaimStrings{"healthhub-api"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}}
	}

	wrongIssuer := valid()
	wrongIssuer.Issuer = "someone-else"
	wrongAudience := valid()
	wrongAudience.Audience = jwt.ClaimStrings{"other-api"}
	noExpiry := valid()
	noExpiry.ExpiresAt = nil
	noSubject := valid()
	noSubject.Subject = ""

	tests := map[string]string{
		"alg none":       sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid()),
		"HS512":          sign(t, jwt.SigningMethodHS512, testSecret, valid()),
		"wrong secret":   sign(t, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32)), valid()),
		"wrong issuer":   sign(t, jwt.SigningMethodHS256, testSecret, wrongIssuer),
		"wrong audience": sign(t, jwt.SigningMethodHS256, testSecret, wrongAudience),
		"no expiry":      sign(t, jwt.SigningMethodHS256, testSecret, noExpiry),
		"no subject":     sign(t, jwt.SigningMethodHS256, testSecret, noSubject),
		"garbage":        "not.a.token",
		"empty":          "",
	}
	for name, tok := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := m.Verify(tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyDropsUnknownRoles(t *testing.T) {
	m := newTestManager(t)
	tok, _, _ := m.Issue(Principal{UserID: "u-1", Roles: []Role{"superuser", RoleNurse}})
	p, err := m.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Roles) != 1 || p.Roles[0] != RoleNurse {
		t.Fatalf("roles = %v", p.Roles)
	}
}

func TestNewTokenManagerValidation(t *testing.T) {
	bad := []TokenConfig{
		{Secret: []byte("short"), Issuer: "i", Audience: "a", TTL: time.Hour},
		{Secret: testSecret, Audience: "a", TTL: time.Hour},
		{Secret: testSecret, Issuer: "i", TTL: time.Hour},
		{Secret: testSecret, Issuer: "i", Audience: "a"},
	}
	for i, cfg := range bad {
		if _, err := NewTokenManager(cfg); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestCan(t *testing.T) {
	tests := []struct {
		roles  []Role
		res    Resource
		act    Action
		expect bool
	}{
		{[]Role{RoleAdmin}, ResourcePatients, ActionDelete, true},
		{[]Role{RoleAdmin}, ResourceUsers, ActionCreate, true},
		{[]Role{RolePractitioner}, ResourcePatients, ActionCreate, true},
		{[]Role{RolePractitioner}, ResourcePatients, ActionDelete, false},
		{[]Role{RolePractitioner}, ResourceUsers, ActionRead, false},
		{[]Role{RoleNurse}, ResourcePatients, ActionRead, true},
		{[]Role{RoleNurse}, ResourceObservations, ActionCreate, false},
		{[]Role{RoleLabTech}, ResourceObservations, ActionCreate, true},
		{[]Role{RoleLabTech}, ResourcePatients, ActionUpdate, false},
		{[]Role{RoleNurse, RoleLabTech}, ResourceObservations, ActionUpdate, true},
		{nil, ResourcePatients, ActionRead, false},
		{[]Role{"unknown"}, ResourcePatients, ActionRead, false},
	}
	for _, tt := range tests {
		if got := Can(tt.roles, tt.res, tt.act); got != tt.expect {
			t.Errorf("Can(%v, %s, %s) = %v, want %v", tt.roles, tt.res, tt.act, got, tt.expect)
		}
	}
}

func TestRoleValid(t *testing.T) {
	for _, r := range AllRoles() {
		if !r.Valid() {
			t.Errorf("%s should be valid", r)
		}
	}
	if Role("root").Valid() {
		t.Error("root should not be valid")
	}
}

func TestPasswordHasher(t *testing.T) {
	h, err := NewPasswordHasher(10)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !h.Verify(hash, "correct horse battery") {
		t.Error("correct password rejected")
	}
	if h.Verify(hash, "wrong password!!") {
		t.Error("wrong password accepted")
	}
	if h.Verify("", "anything at all") {
		t.Error("empty hash (unknown account) accepted")
	}
	if _, err := h.Hash("short"); !errors.Is(err, ErrPasswordPolicy) {
		t.Errorf("short password: err = %v", err)
	}
	if _, err := h.Hash(strings.Repeat("a", MaxPasswordBytes+1)); !errors.Is(err, ErrPasswordPolicy) {
		t.Errorf("long password: err = %v", err)
	}
	if _, err := NewPasswordHasher(99); err == nil {
		t.Error("expected error for out of range cost")
	}
}

func TestBearerToken(t *testing.T) {
	tests := map[string]struct {
		tok string
		ok  bool
	}{
		"Bearer abc":   {"abc", true},
		"bearer abc":   {"abc", true},
		"  Bearer abc": {"abc", true},
		"Bearer ":      {"", false},
		"Basic abc":    {"", false},
		"abc":          {"", false},
		"":             {"", false},
	}
	for in, want := range tests {
		tok, ok := bearerToken(in)
		if tok != want.tok || ok != want.ok {
			t.Errorf("bearerToken(%q) = %q, %v; want %q, %v", in, tok, ok, want.tok, want.ok)
		}
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestManager(t)
	nurseToken, _, _ := m.Issue(Principal{UserID: "nurse-1", Roles: []Role{RoleNurse}})

	r := gin.New()
	g := r.Group("", Authenticate(m))
	g.GET("/patients", Require(ResourcePatients, ActionRead), func(c *gin.Context) {
		p, _ := PrincipalFrom(c)
		c.String(http.StatusOK, p.UserID)
	})
	g.DELETE("/patients", Require(ResourcePatients, ActionDelete), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tests := []struct {
		name, method, auth string
		status             int
		code               string
	}{
		{"no header", http.MethodGet, "", http.StatusUnauthorized, problem.CodeUnauthorized},
		{"bad token", http.MethodGet, "Bearer nope", http.StatusUnauthorized, problem.CodeInvalidToken},
		{"allowed", http.MethodGet, "Bearer " + nurseToken, http.StatusOK, ""},
		{"forbidden", http.MethodDelete, "Bearer " + nurseToken, http.StatusForbidden, problem.CodeForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/patients", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tt.status, w.Body)
			}
			if tt.code == "" {
				if w.Body.String() != "nurse-1" {
					t.Fatalf("body = %q", w.Body)
				}
				return
			}
			if ct := w.Header().Get("Content-Type"); ct != problem.ContentType {
				t.Errorf("content type = %q", ct)
			}
			var p problem.Problem
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Code != tt.code || p.Status != tt.status {
				t.Errorf("problem = %+v", p)
			}
			if strings.Contains(w.Body.String(), "nurse") {
				t.Error("problem response leaks the caller's roles")
			}
			if tt.status == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
				t.Error("missing WWW-Authenticate header")
			}
		})
	}
}
