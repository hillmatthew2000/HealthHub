package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenManager handles JWT token generation and validation
type TokenManager struct {
	secretKey []byte
	issuer    string
}

// NewTokenManager creates a new token manager
func NewTokenManager(secretKey, issuer string) *TokenManager {
	return &TokenManager{
		secretKey: []byte(secretKey),
		issuer:    issuer,
	}
}

// GenerateToken generates a JWT token for a user
func (tm *TokenManager) GenerateToken(userID, email string, roles []string) (string, time.Time, error) {
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &Claims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    tm.issuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(tm.secretKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// ValidateToken validates a JWT token and returns the claims
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return tm.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrTokenInvalid
}

// RefreshToken generates a new token from an existing valid token
func (tm *TokenManager) RefreshToken(tokenString string) (string, time.Time, error) {
	claims, err := tm.ValidateToken(tokenString)
	if err != nil {
		return "", time.Time{}, err
	}

	// Generate new token with the same claims but updated expiration
	return tm.GenerateToken(claims.UserID, claims.Email, claims.Roles)
}

// ExtractUserInfo extracts user information from claims
func (c *Claims) ExtractUserInfo() (userID, email string, roles []string) {
	return c.UserID, c.Email, c.Roles
}

// HasRole checks if the user has a specific role
func (c *Claims) HasRole(role string) bool {
	for _, userRole := range c.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}
