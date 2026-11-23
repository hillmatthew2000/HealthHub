package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

// Context keys set by Authenticate.
const (
	principalKey = "auth.principal"
	// UserIDKey holds the authenticated user ID as a string; the request
	// logger reads it.
	UserIDKey = "user_id"
)

// TokenVerifier verifies bearer tokens. *TokenManager implements it.
type TokenVerifier interface {
	Verify(token string) (Principal, error)
}

// Authenticate requires a valid bearer token and stores the principal in
// the request context.
func Authenticate(v TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.Header("WWW-Authenticate", `Bearer realm="healthhub"`)
			problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeUnauthorized,
				"A bearer token is required."))
			return
		}
		p, err := v.Verify(token)
		if err != nil {
			c.Header("WWW-Authenticate", `Bearer realm="healthhub", error="invalid_token"`)
			problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeInvalidToken,
				"The access token is invalid or has expired."))
			return
		}
		SetPrincipal(c, p)
		c.Next()
	}
}

// Require allows the request only if the principal may perform action on
// resource. It must run after Authenticate.
func Require(resource Resource, action Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := PrincipalFrom(c)
		if !ok {
			problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeUnauthorized,
				"Authentication is required."))
			return
		}
		if !p.Can(resource, action) {
			// Do not echo the caller's roles or the required roles.
			problem.Write(c, problem.Forbidden())
			return
		}
		c.Next()
	}
}

// SetPrincipal stores p in the request context.
func SetPrincipal(c *gin.Context, p Principal) {
	c.Set(principalKey, p)
	c.Set(UserIDKey, p.UserID)
}

// PrincipalFrom returns the authenticated principal, if any.
func PrincipalFrom(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header. The scheme is case-insensitive per RFC 7235.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
