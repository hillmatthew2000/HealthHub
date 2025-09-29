package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware creates a middleware function for JWT authentication
func AuthMiddleware(tokenManager *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
				"code":  "MISSING_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>" format
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Bearer token required",
				"code":  "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := tokenManager.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_roles", claims.Roles)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireRole creates a middleware that requires specific roles
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "User authentication required",
				"code":  "NOT_AUTHENTICATED",
			})
			c.Abort()
			return
		}

		userClaims, ok := claims.(*Claims)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid user claims",
				"code":  "INVALID_CLAIMS",
			})
			c.Abort()
			return
		}

		// Check if user has any of the allowed roles
		if !userClaims.HasAnyRole(allowedRoles...) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":          "Insufficient permissions",
				"code":           "INSUFFICIENT_PERMISSIONS",
				"required_roles": allowedRoles,
				"user_roles":     userClaims.Roles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePermission creates a middleware that requires specific permissions
func RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "User authentication required",
				"code":  "NOT_AUTHENTICATED",
			})
			c.Abort()
			return
		}

		// This would typically check against a permission service
		// For now, we'll use role-based checks
		// In a real implementation, you'd query the database for user permissions

		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "User claims not found",
				"code":  "MISSING_CLAIMS",
			})
			c.Abort()
			return
		}

		userClaims := claims.(*Claims)

		// Basic permission mapping based on roles
		hasPermission := checkPermission(userClaims.Roles, resource, action)

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient permissions",
				"code":     "INSUFFICIENT_PERMISSIONS",
				"resource": resource,
				"action":   action,
				"user_id":  userID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkPermission is a helper function to check permissions based on roles
func checkPermission(userRoles []string, resource, action string) bool {
	// Define role-based permissions
	permissions := map[string]map[string][]string{
		"admin": {
			"patients":     {"create", "read", "update", "delete"},
			"observations": {"create", "read", "update", "delete"},
			"users":        {"create", "read", "update", "delete"},
		},
		"practitioner": {
			"patients":     {"create", "read", "update"},
			"observations": {"create", "read", "update"},
		},
		"nurse": {
			"patients":     {"read"},
			"observations": {"read"},
		},
		"lab-tech": {
			"patients":     {"read"},
			"observations": {"create", "read", "update"},
		},
	}

	for _, role := range userRoles {
		if resourcePerms, exists := permissions[role]; exists {
			if actions, exists := resourcePerms[resource]; exists {
				for _, allowedAction := range actions {
					if allowedAction == action {
						return true
					}
				}
			}
		}
	}

	return false
}

// OptionalAuth creates a middleware that extracts user info if present but doesn't require it
func OptionalAuth(tokenManager *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.Next()
			return
		}

		claims, err := tokenManager.ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_roles", claims.Roles)
		c.Set("claims", claims)
		c.Set("authenticated", true)

		c.Next()
	}
}

// GetUserID extracts the user ID from the context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}

	if id, ok := userID.(string); ok {
		return id, true
	}

	return "", false
}

// GetUserRoles extracts the user roles from the context
func GetUserRoles(c *gin.Context) ([]string, bool) {
	userRoles, exists := c.Get("user_roles")
	if !exists {
		return nil, false
	}

	if roles, ok := userRoles.([]string); ok {
		return roles, true
	}

	return nil, false
}

// GetClaims extracts the JWT claims from the context
func GetClaims(c *gin.Context) (*Claims, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}

	if userClaims, ok := claims.(*Claims); ok {
		return userClaims, true
	}

	return nil, false
}

// IsAuthenticated checks if the request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}
