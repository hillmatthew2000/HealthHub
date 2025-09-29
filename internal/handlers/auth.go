package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"gorm.io/gorm"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	db           *gorm.DB
	validator    *validator.Validate
	tokenManager *auth.TokenManager
	rbacService  *auth.RBACService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(db *gorm.DB, jwtSecret string) *AuthHandler {
	tokenManager := auth.NewTokenManager(jwtSecret, "HealthHub API")
	rbacService := auth.NewRBACService(db)

	return &AuthHandler{
		db:           db,
		validator:    validator.New(),
		tokenManager: tokenManager,
		rbacService:  rbacService,
	}
}

// Login authenticates a user and returns a JWT token
// @Summary User login
// @Description Authenticate user and get access token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body models.AuthRequest true "Login credentials"
// @Success 200 {object} models.AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.AuthRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
			Code:    "INVALID_REQUEST_BODY",
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
			Code:    "VALIDATION_FAILED",
		})
		return
	}

	// Find user by email
	var user models.User
	if err := h.db.Preload("Roles").Where("email = ? AND active = ?", req.Email, true).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error: "Invalid credentials",
				Code:  "INVALID_CREDENTIALS",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to authenticate user",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check password
	if err := user.CheckPassword(req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Invalid credentials",
			Code:  "INVALID_CREDENTIALS",
		})
		return
	}

	// Update last login time
	now := time.Now()
	user.LastLogin = &now
	h.db.Model(&user).Update("last_login", now)

	// Generate JWT token
	roleNames := user.GetRoleNames()
	token, expiresAt, err := h.tokenManager.GenerateToken(user.ID, user.Email, roleNames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to generate token",
			Message: err.Error(),
			Code:    "TOKEN_GENERATION_FAILED",
		})
		return
	}

	// Prepare response
	response := models.AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: models.UserInfo{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Roles:     roleNames,
			Active:    user.Active,
		},
	}

	c.JSON(http.StatusOK, response)
}

// Register creates a new user account
// @Summary User registration
// @Description Create a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param user body models.RegisterRequest true "User registration data"
// @Success 201 {object} models.AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
			Code:    "INVALID_REQUEST_BODY",
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
			Code:    "VALIDATION_FAILED",
		})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "User with this email already exists",
			Code:  "USER_ALREADY_EXISTS",
		})
		return
	}

	// Create new user
	user := models.User{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Active:    true,
	}

	// Hash password
	if err := user.HashPassword(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to process password",
			Message: err.Error(),
			Code:    "PASSWORD_HASH_FAILED",
		})
		return
	}

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create user
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create user",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Assign default roles
	defaultRoles := req.Roles
	if len(defaultRoles) == 0 {
		defaultRoles = []string{"nurse"} // Default role for new users
	}

	for _, roleName := range defaultRoles {
		var role models.Role
		if err := tx.Where("name = ?", roleName).First(&role).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "Invalid role: " + roleName,
				Code:  "INVALID_ROLE",
			})
			return
		}

		if err := h.rbacService.AssignRoleToUser(user.ID, role.ID, "system"); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to assign role",
				Message: err.Error(),
				Code:    "ROLE_ASSIGNMENT_FAILED",
			})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to complete registration",
			Message: err.Error(),
			Code:    "TRANSACTION_FAILED",
		})
		return
	}

	// Load user with roles for response
	if err := h.db.Preload("Roles").Where("id = ?", user.ID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to load user data",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Generate JWT token
	roleNames := user.GetRoleNames()
	token, expiresAt, err := h.tokenManager.GenerateToken(user.ID, user.Email, roleNames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to generate token",
			Message: err.Error(),
			Code:    "TOKEN_GENERATION_FAILED",
		})
		return
	}

	// Prepare response
	response := models.AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: models.UserInfo{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Roles:     roleNames,
			Active:    user.Active,
		},
	}

	c.JSON(http.StatusCreated, response)
}

// RefreshToken refreshes an existing JWT token
// @Summary Refresh access token
// @Description Refresh an existing access token
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} models.AuthResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Invalid token",
			Code:  "INVALID_TOKEN",
		})
		return
	}

	// Verify user is still active
	var user models.User
	if err := h.db.Preload("Roles").Where("id = ? AND active = ?", claims.UserID, true).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not found or inactive",
			Code:  "USER_INACTIVE",
		})
		return
	}

	// Generate new token
	roleNames := user.GetRoleNames()
	token, expiresAt, err := h.tokenManager.GenerateToken(user.ID, user.Email, roleNames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to generate token",
			Message: err.Error(),
			Code:    "TOKEN_GENERATION_FAILED",
		})
		return
	}

	response := models.AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: models.UserInfo{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Roles:     roleNames,
			Active:    user.Active,
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetProfile returns the current user's profile
// @Summary Get user profile
// @Description Get the authenticated user's profile information
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} models.UserInfo
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
			Code:  "NOT_AUTHENTICATED",
		})
		return
	}

	var user models.User
	if err := h.db.Preload("Roles").Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch user profile",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	userInfo := models.UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Roles:     user.GetRoleNames(),
		Active:    user.Active,
	}

	c.JSON(http.StatusOK, userInfo)
}

// ChangePassword changes the current user's password
// @Summary Change password
// @Description Change the authenticated user's password
// @Tags auth
// @Accept json
// @Produce json
// @Param password body models.ChangePasswordRequest true "Password change data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
			Code:  "NOT_AUTHENTICATED",
		})
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
			Code:    "INVALID_REQUEST_BODY",
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
			Code:    "VALIDATION_FAILED",
		})
		return
	}

	// Get current user
	var user models.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch user",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Verify current password
	if err := user.CheckPassword(req.CurrentPassword); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Current password is incorrect",
			Code:  "INVALID_CURRENT_PASSWORD",
		})
		return
	}

	// Update password
	user.Password = req.NewPassword
	if err := user.HashPassword(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to process new password",
			Message: err.Error(),
			Code:    "PASSWORD_HASH_FAILED",
		})
		return
	}

	if err := h.db.Model(&user).Update("password", user.Password).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update password",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, NewSuccessResponse("Password changed successfully", nil))
}
