package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a system user with role-based access
type User struct {
	ID        string     `json:"id" gorm:"primaryKey"`
	Email     string     `json:"email" gorm:"uniqueIndex" validate:"required,email"`
	Password  string     `json:"-" validate:"required,min=8"` // Never expose password in JSON
	FirstName string     `json:"firstName" validate:"required"`
	LastName  string     `json:"lastName" validate:"required"`
	Roles     []Role     `json:"roles" gorm:"many2many:user_roles;"`
	Active    bool       `json:"active" gorm:"default:true"`
	LastLogin *time.Time `json:"lastLogin,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	CreatedBy string     `json:"createdBy,omitempty"`
}

// Role represents a system role for RBAC
type Role struct {
	ID          string       `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"uniqueIndex" validate:"required"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// Permission represents a specific permission in the system
type Permission struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex" validate:"required"`
	Description string    `json:"description"`
	Resource    string    `json:"resource" validate:"required"` // e.g., "patients", "observations"
	Action      string    `json:"action" validate:"required"`   // e.g., "read", "write", "delete"
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// UserRole represents the junction table for users and roles
type UserRole struct {
	UserID    string    `json:"userId" gorm:"primaryKey"`
	RoleID    string    `json:"roleId" gorm:"primaryKey"`
	GrantedBy string    `json:"grantedBy"`
	GrantedAt time.Time `json:"grantedAt"`
}

// RolePermission represents the junction table for roles and permissions
type RolePermission struct {
	RoleID       string    `json:"roleId" gorm:"primaryKey"`
	PermissionID string    `json:"permissionId" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"createdAt"`
}

// BeforeCreate is a GORM hook that runs before creating a user
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// BeforeCreate is a GORM hook that runs before creating a role
func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// BeforeCreate is a GORM hook that runs before creating a permission
func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// TableName returns the table name for the Role model
func (Role) TableName() string {
	return "roles"
}

// TableName returns the table name for the Permission model
func (Permission) TableName() string {
	return "permissions"
}

// TableName returns the table name for the UserRole model
func (UserRole) TableName() string {
	return "user_roles"
}

// TableName returns the table name for the RolePermission model
func (RolePermission) TableName() string {
	return "role_permissions"
}

// HashPassword hashes the user's password using bcrypt
func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword verifies the provided password against the stored hash
func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

// GetFullName returns the user's full name
func (u *User) GetFullName() string {
	return u.FirstName + " " + u.LastName
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(roleName string) bool {
	for _, role := range u.Roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// HasPermission checks if the user has a specific permission
func (u *User) HasPermission(resource, action string) bool {
	for _, role := range u.Roles {
		for _, permission := range role.Permissions {
			if permission.Resource == resource && permission.Action == action {
				return true
			}
		}
	}
	return false
}

// GetRoleNames returns a slice of role names for the user
func (u *User) GetRoleNames() []string {
	roleNames := make([]string, len(u.Roles))
	for i, role := range u.Roles {
		roleNames[i] = role.Name
	}
	return roleNames
}

// AuthRequest represents a login request
type AuthRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse represents a login response
type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      UserInfo  `json:"user"`
}

// UserInfo represents user information for responses
type UserInfo struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Roles     []string `json:"roles"`
	Active    bool     `json:"active"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email     string   `json:"email" validate:"required,email"`
	Password  string   `json:"password" validate:"required,min=8"`
	FirstName string   `json:"firstName" validate:"required"`
	LastName  string   `json:"lastName" validate:"required"`
	Roles     []string `json:"roles,omitempty"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	FirstName string   `json:"firstName,omitempty"`
	LastName  string   `json:"lastName,omitempty"`
	Active    *bool    `json:"active,omitempty"`
	Roles     []string `json:"roles,omitempty"`
}
