// Package middleware holds HTTP middleware shared by all routes.
package middleware

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

// CORS allows cross-origin requests from the configured origins. A single
// "*" allows any origin. Credentials are never allowed: the API uses bearer
// tokens, not cookies, so there is no reason to let browsers attach ambient
// credentials to cross-origin requests.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowAny := slices.Contains(allowedOrigins, "*")
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Writer.Header().Add("Vary", "Origin")

		allowed := origin != "" && (allowAny || slices.Contains(allowedOrigins, origin))
		if allowed {
			if allowAny {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header("Access-Control-Expose-Headers", "Location, X-Request-ID")
		}

		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			if allowed {
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				c.Header("Access-Control-Max-Age", "600")
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders sets defensive response headers. API responses are JSON,
// so the content security policy forbids everything.
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Responses may contain PHI; never let shared caches store them.
		h.Set("Cache-Control", "no-store")
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		c.Next()
	}
}

// RequestIDHeader carries the request ID in both directions.
const RequestIDHeader = "X-Request-ID"

// RequestID assigns every request an ID, reusing a well-formed one supplied
// by the client or a proxy, and echoes it in the response so logs and
// problem responses can be correlated.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID(id) {
			id = uuid.NewString()
		}
		c.Set(problem.RequestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// validRequestID accepts short IDs made of URL-safe characters, so a client
// cannot inject newlines or arbitrary text into logs.
func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// Timeout bounds the time a handler may spend, mostly waiting on the
// database, by putting a deadline on the request context.
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// MaxBodyBytes limits the size of request bodies.
func MaxBodyBytes(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
