package auth

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// Password length limits. bcrypt only uses the first 72 bytes of its input,
// so longer passwords are rejected rather than silently truncated.
const (
	MinPasswordLength = 12
	MaxPasswordBytes  = 72
)

// ErrPasswordPolicy is returned for passwords that violate the length policy.
var ErrPasswordPolicy = fmt.Errorf("password must be %d to %d bytes long", MinPasswordLength, MaxPasswordBytes)

// PasswordHasher hashes and verifies passwords with bcrypt.
type PasswordHasher struct {
	cost  int
	dummy []byte
}

// NewPasswordHasher returns a hasher using the given bcrypt cost.
func NewPasswordHasher(cost int) (*PasswordHasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("auth: bcrypt cost %d out of range", cost)
	}
	// A hash of a random-looking value with the same cost, compared against
	// when a login names an unknown account so response time does not reveal
	// whether the account exists.
	dummy, err := bcrypt.GenerateFromPassword([]byte("healthhub-timing-equaliser"), cost)
	if err != nil {
		return nil, err
	}
	return &PasswordHasher{cost: cost, dummy: dummy}, nil
}

// CheckPolicy validates password against the length policy.
func CheckPolicy(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength || len(password) > MaxPasswordBytes {
		return ErrPasswordPolicy
	}
	return nil
}

// Hash returns the bcrypt hash of password after checking the policy.
func (h *PasswordHasher) Hash(password string) (string, error) {
	if err := CheckPolicy(password); err != nil {
		return "", err
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify reports whether password matches hash. An empty hash (unknown
// account) still costs one bcrypt comparison.
func (h *PasswordHasher) Verify(hash, password string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(h.dummy, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
