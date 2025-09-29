package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/hillmatthew2000/HealthHub/internal/models"
	"gorm.io/gorm"
)

// RBACService handles role-based access control operations
type RBACService struct {
	db *gorm.DB
}

// NewRBACService creates a new RBAC service
func NewRBACService(db *gorm.DB) *RBACService {
	return &RBACService{db: db}
}

// CreateRole creates a new role
func (s *RBACService) CreateRole(name, description string, permissionIDs []string) (*models.Role, error) {
	// Check if role already exists
	var existingRole models.Role
	if err := s.db.Where("name = ?", name).First(&existingRole).Error; err == nil {
		return nil, fmt.Errorf("role with name '%s' already exists", name)
	}

	// Create the role
	role := &models.Role{
		Name:        name,
		Description: description,
	}

	if err := s.db.Create(role).Error; err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	// Associate permissions if provided
	if len(permissionIDs) > 0 {
		var permissions []models.Permission
		if err := s.db.Where("id IN ?", permissionIDs).Find(&permissions).Error; err != nil {
			return nil, fmt.Errorf("failed to find permissions: %w", err)
		}

		if err := s.db.Model(role).Association("Permissions").Append(&permissions); err != nil {
			return nil, fmt.Errorf("failed to associate permissions: %w", err)
		}
	}

	// Load the role with permissions
	s.db.Preload("Permissions").First(role, "id = ?", role.ID)

	return role, nil
}

// CreatePermission creates a new permission
func (s *RBACService) CreatePermission(name, description, resource, action string) (*models.Permission, error) {
	// Check if permission already exists
	var existingPermission models.Permission
	if err := s.db.Where("name = ?", name).First(&existingPermission).Error; err == nil {
		return nil, fmt.Errorf("permission with name '%s' already exists", name)
	}

	permission := &models.Permission{
		Name:        name,
		Description: description,
		Resource:    resource,
		Action:      action,
	}

	if err := s.db.Create(permission).Error; err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	return permission, nil
}

// AssignRoleToUser assigns a role to a user
func (s *RBACService) AssignRoleToUser(userID, roleID, grantedBy string) error {
	// Check if user exists
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Check if role exists
	var role models.Role
	if err := s.db.First(&role, "id = ?", roleID).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	// Check if assignment already exists
	var existingAssignment models.UserRole
	if err := s.db.Where("user_id = ? AND role_id = ?", userID, roleID).First(&existingAssignment).Error; err == nil {
		return fmt.Errorf("user already has this role")
	}

	// Create the assignment
	assignment := &models.UserRole{
		UserID:    userID,
		RoleID:    roleID,
		GrantedBy: grantedBy,
		GrantedAt: time.Now(),
	}

	if err := s.db.Create(assignment).Error; err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	return nil
}

// RemoveRoleFromUser removes a role from a user
func (s *RBACService) RemoveRoleFromUser(userID, roleID string) error {
	result := s.db.Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&models.UserRole{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove role: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("role assignment not found")
	}

	return nil
}

// GetUserRoles retrieves all roles for a user
func (s *RBACService) GetUserRoles(userID string) ([]models.Role, error) {
	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user.Roles, nil
}

// GetUserPermissions retrieves all permissions for a user
func (s *RBACService) GetUserPermissions(userID string) ([]models.Permission, error) {
	roles, err := s.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	permissionMap := make(map[string]models.Permission)
	for _, role := range roles {
		for _, permission := range role.Permissions {
			permissionMap[permission.ID] = permission
		}
	}

	permissions := make([]models.Permission, 0, len(permissionMap))
	for _, permission := range permissionMap {
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// HasPermission checks if a user has a specific permission
func (s *RBACService) HasPermission(userID, resource, action string) (bool, error) {
	permissions, err := s.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}

	for _, permission := range permissions {
		if permission.Resource == resource && permission.Action == action {
			return true, nil
		}
	}

	return false, nil
}

// HasRole checks if a user has a specific role
func (s *RBACService) HasRole(userID, roleName string) (bool, error) {
	roles, err := s.GetUserRoles(userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role.Name == roleName {
			return true, nil
		}
	}

	return false, nil
}

// ListRoles retrieves all roles with pagination
func (s *RBACService) ListRoles(page, limit int) ([]models.Role, int64, error) {
	var roles []models.Role
	var total int64

	// Count total roles
	s.db.Model(&models.Role{}).Count(&total)

	// Get roles with pagination
	offset := (page - 1) * limit
	if err := s.db.Preload("Permissions").Offset(offset).Limit(limit).Find(&roles).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list roles: %w", err)
	}

	return roles, total, nil
}

// ListPermissions retrieves all permissions with pagination
func (s *RBACService) ListPermissions(page, limit int) ([]models.Permission, int64, error) {
	var permissions []models.Permission
	var total int64

	// Count total permissions
	s.db.Model(&models.Permission{}).Count(&total)

	// Get permissions with pagination
	offset := (page - 1) * limit
	if err := s.db.Offset(offset).Limit(limit).Find(&permissions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, total, nil
}

// DeleteRole deletes a role and its associations
func (s *RBACService) DeleteRole(roleID string) error {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Remove role from all users
	if err := tx.Where("role_id = ?", roleID).Delete(&models.UserRole{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove role from users: %w", err)
	}

	// Remove role permissions
	if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove role permissions: %w", err)
	}

	// Delete the role
	if err := tx.Delete(&models.Role{}, "id = ?", roleID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete role: %w", err)
	}

	return tx.Commit().Error
}

// DeletePermission deletes a permission and its associations
func (s *RBACService) DeletePermission(permissionID string) error {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Remove permission from all roles
	if err := tx.Where("permission_id = ?", permissionID).Delete(&models.RolePermission{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove permission from roles: %w", err)
	}

	// Delete the permission
	if err := tx.Delete(&models.Permission{}, "id = ?", permissionID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	return tx.Commit().Error
}

// InitializeDefaultRoles creates default roles and permissions
func (s *RBACService) InitializeDefaultRoles() error {
	// Define default permissions
	defaultPermissions := []models.Permission{
		{Name: "patients:create", Description: "Create patients", Resource: "patients", Action: "create"},
		{Name: "patients:read", Description: "Read patients", Resource: "patients", Action: "read"},
		{Name: "patients:update", Description: "Update patients", Resource: "patients", Action: "update"},
		{Name: "patients:delete", Description: "Delete patients", Resource: "patients", Action: "delete"},
		{Name: "observations:create", Description: "Create observations", Resource: "observations", Action: "create"},
		{Name: "observations:read", Description: "Read observations", Resource: "observations", Action: "read"},
		{Name: "observations:update", Description: "Update observations", Resource: "observations", Action: "update"},
		{Name: "observations:delete", Description: "Delete observations", Resource: "observations", Action: "delete"},
		{Name: "users:create", Description: "Create users", Resource: "users", Action: "create"},
		{Name: "users:read", Description: "Read users", Resource: "users", Action: "read"},
		{Name: "users:update", Description: "Update users", Resource: "users", Action: "update"},
		{Name: "users:delete", Description: "Delete users", Resource: "users", Action: "delete"},
	}

	// Create permissions if they don't exist
	for _, perm := range defaultPermissions {
		var existing models.Permission
		if err := s.db.Where("name = ?", perm.Name).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := s.db.Create(&perm).Error; err != nil {
					return fmt.Errorf("failed to create permission %s: %w", perm.Name, err)
				}
			} else {
				return fmt.Errorf("failed to check permission %s: %w", perm.Name, err)
			}
		}
	}

	// Define default roles with their permissions
	rolePermissions := map[string][]string{
		"admin": {
			"patients:create", "patients:read", "patients:update", "patients:delete",
			"observations:create", "observations:read", "observations:update", "observations:delete",
			"users:create", "users:read", "users:update", "users:delete",
		},
		"practitioner": {
			"patients:create", "patients:read", "patients:update",
			"observations:create", "observations:read", "observations:update",
		},
		"nurse": {
			"patients:read", "observations:read",
		},
		"lab-tech": {
			"patients:read", "observations:create", "observations:read", "observations:update",
		},
	}

	// Create roles if they don't exist
	for roleName, permNames := range rolePermissions {
		var role models.Role
		if err := s.db.Where("name = ?", roleName).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				role = models.Role{
					Name:        roleName,
					Description: fmt.Sprintf("Default %s role", roleName),
				}
				if err := s.db.Create(&role).Error; err != nil {
					return fmt.Errorf("failed to create role %s: %w", roleName, err)
				}

				// Assign permissions to role
				var permissions []models.Permission
				if err := s.db.Where("name IN ?", permNames).Find(&permissions).Error; err != nil {
					return fmt.Errorf("failed to find permissions for role %s: %w", roleName, err)
				}

				if err := s.db.Model(&role).Association("Permissions").Append(&permissions); err != nil {
					return fmt.Errorf("failed to assign permissions to role %s: %w", roleName, err)
				}
			} else {
				return fmt.Errorf("failed to check role %s: %w", roleName, err)
			}
		}
	}

	return nil
}
