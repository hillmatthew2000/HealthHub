// Package middleware holds HTTP middleware shared by all routes.
package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
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
