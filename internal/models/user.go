package models

import "time"

// User is a staff account. Roles hold auth.Role values; the access policy
// for each role lives in the auth package.
type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FirstName    string     `json:"firstName"`
	LastName     string     `json:"lastName"`
	Roles        []string   `json:"roles"`
	Active       bool       `json:"active"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	CreatedBy    string     `json:"createdBy,omitempty"`
}

// FullName returns the user's display name.
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// AuthRequest represents a login request.
type AuthRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=1024"`
}

// AuthResponse represents a login response.
type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      UserInfo  `json:"user"`
}

// UserInfo is the public view of a user.
type UserInfo struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	FirstName   string     `json:"firstName"`
	LastName    string     `json:"lastName"`
	Roles       []string   `json:"roles"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Info returns the public view of u.
func (u *User) Info() UserInfo {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return UserInfo{
		ID:          u.ID,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Roles:       roles,
		Active:      u.Active,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}

// RegisterRequest represents a self-registration request.
type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email,max=254"`
	Password  string `json:"password" validate:"required,min=12,max=72"`
	FirstName string `json:"firstName" validate:"required,max=100"`
	LastName  string `json:"lastName" validate:"required,max=100"`
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required,max=1024"`
	NewPassword     string `json:"newPassword" validate:"required,min=12,max=72"`
}

// CreateUserRequest is an administrator's request to create an account.
type CreateUserRequest struct {
	Email     string   `json:"email" validate:"required,email,max=254"`
	Password  string   `json:"password" validate:"required,min=12,max=72"`
	FirstName string   `json:"firstName" validate:"required,max=100"`
	LastName  string   `json:"lastName" validate:"required,max=100"`
	Roles     []string `json:"roles" validate:"max=4,dive,oneof=admin practitioner nurse lab-tech"`
}

// UpdateUserRequest is an administrator's partial update of an account.
// Omitted fields are left unchanged.
type UpdateUserRequest struct {
	Roles  *[]string `json:"roles,omitempty" validate:"omitempty,max=4,dive,oneof=admin practitioner nurse lab-tech"`
	Active *bool     `json:"active,omitempty"`
}
