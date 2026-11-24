package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// AuthHandler handles authentication requests.
type AuthHandler struct {
	db                *gorm.DB
	validator         *validator.Validate
	tokens            *auth.TokenManager
	passwords         *auth.PasswordHasher
	allowRegistration bool
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(db *gorm.DB, tokens *auth.TokenManager, passwords *auth.PasswordHasher, allowRegistration bool) *AuthHandler {
	return &AuthHandler{
		db:                db,
		validator:         validator.New(),
		tokens:            tokens,
		passwords:         passwords,
		allowRegistration: allowRegistration,
	}
}

// Login authenticates a user and returns an access token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.AuthRequest
	if !h.bind(c, &req) {
		return
	}
	email := normalizeEmail(req.Email)

	var user models.User
	err := h.db.WithContext(c.Request.Context()).Preload("Roles").
		Where("email = ? AND active = ?", email, true).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		internalError(c, "load user for login", err)
		return
	}

	// Verify runs a bcrypt comparison even for unknown accounts so the
	// response time does not reveal which email addresses are registered.
	if !h.passwords.Verify(user.Password, req.Password) {
		logger.LogSecurityEvent("login_failed", user.ID, nil)
		problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeInvalidCredentials,
			"The email address or password is incorrect."))
		return
	}

	now := time.Now().UTC()
	if err := h.db.WithContext(c.Request.Context()).Model(&user).Update("last_login", now).Error; err != nil {
		logger.Warn("failed to record last login", zap.String("user_id", user.ID), zap.Error(err))
	}
	logger.LogSecurityEvent("login_succeeded", user.ID, nil)
	h.respondWithToken(c, http.StatusOK, &user)
}

// Register creates a new user account. Self-registration is disabled unless
// ALLOW_REGISTRATION is set, and self-registered accounts receive no roles:
// an administrator must grant access explicitly.
func (h *AuthHandler) Register(c *gin.Context) {
	if !h.allowRegistration {
		problem.Write(c, problem.New(http.StatusForbidden, problem.CodeRegistrationDisabled,
			"Self-registration is disabled. Ask an administrator to create your account."))
		return
	}

	var req models.RegisterRequest
	if !h.bind(c, &req) {
		return
	}
	hash, err := h.passwords.Hash(req.Password)
	if err != nil {
		problem.Write(c, problem.Validation(problem.FieldError{Field: "password", Message: err.Error()}))
		return
	}

	user := models.User{
		Email:     normalizeEmail(req.Email),
		Password:  hash,
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
		Active:    true,
	}

	db := h.db.WithContext(c.Request.Context())
	var count int64
	if err := db.Model(&models.User{}).Where("email = ?", user.Email).Count(&count).Error; err != nil {
		internalError(c, "check existing user", err)
		return
	}
	if count > 0 {
		problem.Write(c, problem.Conflict("An account with this email address already exists."))
		return
	}
	if err := db.Create(&user).Error; err != nil {
		internalError(c, "create user", err)
		return
	}

	logger.LogAuditEvent("create", "user", user.ID, map[string]any{"source": "self_registration"})
	h.respondWithToken(c, http.StatusCreated, &user)
}

// RefreshToken issues a new token for a caller holding a valid one. The
// account is re-read so deactivated users and role changes take effect.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	p, ok := auth.PrincipalFrom(c)
	if !ok {
		unauthenticated(c)
		return
	}
	var user models.User
	err := h.db.WithContext(c.Request.Context()).Preload("Roles").
		Where("id = ? AND active = ?", p.UserID, true).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeInvalidToken,
			"The account no longer exists or has been deactivated."))
		return
	}
	if err != nil {
		internalError(c, "load user for refresh", err)
		return
	}
	h.respondWithToken(c, http.StatusOK, &user)
}

// GetProfile returns the current user's profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
	p, ok := auth.PrincipalFrom(c)
	if !ok {
		unauthenticated(c)
		return
	}
	var user models.User
	err := h.db.WithContext(c.Request.Context()).Preload("Roles").Where("id = ?", p.UserID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		problem.Write(c, problem.NotFound("The account no longer exists."))
		return
	}
	if err != nil {
		internalError(c, "load profile", err)
		return
	}
	c.JSON(http.StatusOK, userInfo(&user))
}

// ChangePassword changes the current user's password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	p, ok := auth.PrincipalFrom(c)
	if !ok {
		unauthenticated(c)
		return
	}
	var req models.ChangePasswordRequest
	if !h.bind(c, &req) {
		return
	}

	db := h.db.WithContext(c.Request.Context())
	var user models.User
	if err := db.Where("id = ?", p.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			problem.Write(c, problem.NotFound("The account no longer exists."))
			return
		}
		internalError(c, "load user for password change", err)
		return
	}
	if !h.passwords.Verify(user.Password, req.CurrentPassword) {
		logger.LogSecurityEvent("password_change_failed", user.ID, nil)
		problem.Write(c, problem.Validation(problem.FieldError{
			Field: "currentPassword", Message: "is incorrect",
		}))
		return
	}
	hash, err := h.passwords.Hash(req.NewPassword)
	if err != nil {
		problem.Write(c, problem.Validation(problem.FieldError{Field: "newPassword", Message: err.Error()}))
		return
	}
	if err := db.Model(&user).Update("password", hash).Error; err != nil {
		internalError(c, "update password", err)
		return
	}
	logger.LogSecurityEvent("password_changed", user.ID, nil)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) respondWithToken(c *gin.Context, status int, user *models.User) {
	info := userInfo(user)
	roles := make([]auth.Role, len(info.Roles))
	for i, r := range info.Roles {
		roles[i] = auth.Role(r)
	}
	token, expiresAt, err := h.tokens.Issue(auth.Principal{UserID: user.ID, Email: user.Email, Roles: roles})
	if err != nil {
		internalError(c, "issue token", err)
		return
	}
	c.JSON(status, models.AuthResponse{Token: token, ExpiresAt: expiresAt, User: info})
}

// bind decodes and validates a JSON body, writing a problem on failure.
func (h *AuthHandler) bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		problem.Write(c, problem.BadRequest("The request body must be valid JSON."))
		return false
	}
	if err := h.validator.Struct(dst); err != nil {
		problem.Write(c, problem.Validation(fieldErrors(err)...))
		return false
	}
	return true
}

func userInfo(u *models.User) models.UserInfo {
	return models.UserInfo{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Roles:     u.GetRoleNames(),
		Active:    u.Active,
	}
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
