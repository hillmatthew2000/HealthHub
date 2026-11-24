// Package auth implements authentication (JWT access tokens, password
// hashing) and role based authorisation.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken is returned for any token that fails verification. The
// underlying reason is deliberately not exposed to clients.
var ErrInvalidToken = errors.New("auth: invalid or expired token")

// clockSkew is the leeway allowed when checking exp, nbf and iat.
const clockSkew = 30 * time.Second

// Principal is the authenticated caller.
type Principal struct {
	UserID string
	Email  string
	Roles  []Role
}

// Can reports whether the principal may perform action on resource.
func (p Principal) Can(resource Resource, action Action) bool {
	return Can(p.Roles, resource, action)
}

// claims is the JWT payload.
type claims struct {
	Email string `json:"email"`
	Roles []Role `json:"roles"`
	jwt.RegisteredClaims
}

// TokenConfig configures a TokenManager.
type TokenConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
	TTL      time.Duration
}

// TokenManager issues and verifies HS256 signed access tokens.
type TokenManager struct {
	cfg    TokenConfig
	now    func() time.Time
	parser *jwt.Parser
}

// NewTokenManager validates cfg and returns a TokenManager.
func NewTokenManager(cfg TokenConfig) (*TokenManager, error) {
	if len(cfg.Secret) < 32 {
		return nil, errors.New("auth: token secret must be at least 32 bytes")
	}
	if cfg.Issuer == "" || cfg.Audience == "" {
		return nil, errors.New("auth: token issuer and audience are required")
	}
	if cfg.TTL <= 0 {
		return nil, errors.New("auth: token TTL must be positive")
	}
	m := &TokenManager{cfg: cfg, now: time.Now}
	m.parser = jwt.NewParser(
		// Pin the algorithm: rejects "none", RS/ES confusion and anything else.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithAudience(cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
		jwt.WithTimeFunc(func() time.Time { return m.now() }),
	)
	return m, nil
}

// TTL returns the lifetime of issued tokens.
func (m *TokenManager) TTL() time.Duration { return m.cfg.TTL }

// Issue signs a new access token for p.
func (m *TokenManager) Issue(p Principal) (token string, expiresAt time.Time, err error) {
	if p.UserID == "" {
		return "", time.Time{}, errors.New("auth: principal has no user ID")
	}
	now := m.now().UTC().Truncate(time.Second)
	expiresAt = now.Add(m.cfg.TTL)
	c := claims{
		Email: p.Email,
		Roles: p.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   p.UserID,
			Issuer:    m.cfg.Issuer,
			Audience:  jwt.ClaimStrings{m.cfg.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.cfg.Secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return token, expiresAt, nil
}

// Verify checks the signature and registered claims of token and returns
// the principal it was issued for.
func (m *TokenManager) Verify(token string) (Principal, error) {
	var c claims
	_, err := m.parser.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) {
		return m.cfg.Secret, nil
	})
	if err != nil || c.Subject == "" {
		return Principal{}, ErrInvalidToken
	}
	roles := make([]Role, 0, len(c.Roles))
	for _, r := range c.Roles {
		if r.Valid() {
			roles = append(roles, r)
		}
	}
	return Principal{UserID: c.Subject, Email: c.Email, Roles: roles}, nil
}
