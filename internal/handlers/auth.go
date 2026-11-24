package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/store"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// UserStore is the persistence the account handlers need.
type UserStore interface {
	Create(ctx context.Context, u *models.User) error
	Get(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context, page store.Page) ([]models.User, int, error)
	Update(ctx context.Context, id uuid.UUID, upd store.UserUpdate) (*models.User, error)
	SetPassword(ctx context.Context, id uuid.UUID, hash string) error
	RecordLogin(ctx context.Context, id uuid.UUID) error
}

// TokenIssuer issues access tokens. *auth.TokenManager implements it.
type TokenIssuer interface {
	Issue(p auth.Principal) (string, time.Time, error)
}

// AuthHandler handles authentication requests.
type AuthHandler struct {
	users             UserStore
	tokens            TokenIssuer
	passwords         *auth.PasswordHasher
	validator         *validator.Validate
	allowRegistration bool
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(users UserStore, tokens TokenIssuer, passwords *auth.PasswordHasher, allowRegistration bool) *AuthHandler {
	return &AuthHandler{
		users:             users,
		tokens:            tokens,
		passwords:         passwords,
		validator:         newValidator(),
		allowRegistration: allowRegistration,
	}
}

// Login authenticates a user and returns an access token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.AuthRequest
	if !bindJSON(c, h.validator, &req) {
		return
	}
	ctx := c.Request.Context()
	user, err := h.users.GetByEmail(ctx, NormalizeEmail(req.Email))
	switch {
	case errors.Is(err, store.ErrNotFound):
		user = &models.User{}
	case err != nil:
		internalError(c, "load user for login", err)
		return
	}

	// Verify runs a bcrypt comparison even for unknown accounts so the
	// response time does not reveal which email addresses are registered.
	if !h.passwords.Verify(user.PasswordHash, req.Password) || !user.Active {
		logger.LogSecurityEvent("login_failed", user.ID, nil)
		problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeInvalidCredentials,
			"The email address or password is incorrect."))
		return
	}

	if err := h.users.RecordLogin(ctx, uuid.MustParse(user.ID)); err != nil {
		logger.Warn("failed to record last login", zap.String("user_id", user.ID), zap.Error(err))
	}
	logger.LogSecurityEvent("login_succeeded", user.ID, nil)
	h.respondWithToken(c, http.StatusOK, user)
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
	if !bindJSON(c, h.validator, &req) {
		return
	}
	hash, err := h.passwords.Hash(req.Password)
	if err != nil {
		problem.Write(c, problem.Validation(problem.FieldError{Field: "password", Message: err.Error()}))
		return
	}
	user := &models.User{
		Email:        NormalizeEmail(req.Email),
		PasswordHash: hash,
		FirstName:    strings.TrimSpace(req.FirstName),
		LastName:     strings.TrimSpace(req.LastName),
		Active:       true,
	}
	if err := h.users.Create(c.Request.Context(), user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			problem.Write(c, problem.Conflict("An account with this email address already exists."))
			return
		}
		internalError(c, "create user", err)
		return
	}

	logger.LogSecurityEvent("user_registered", user.ID, nil)
	h.respondWithToken(c, http.StatusCreated, user)
}

// currentUser loads the authenticated caller's account. It writes a
// problem and returns nil if the account is gone or deactivated.
func (h *AuthHandler) currentUser(c *gin.Context) *models.User {
	p, ok := auth.PrincipalFrom(c)
	if !ok {
		unauthenticated(c)
		return nil
	}
	id, err := uuid.Parse(p.UserID)
	if err != nil {
		unauthenticated(c)
		return nil
	}
	user, err := h.users.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !user.Active) {
		problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeInvalidToken,
			"The account no longer exists or has been deactivated."))
		return nil
	}
	if err != nil {
		internalError(c, "load current user", err)
		return nil
	}
	return user
}

// RefreshToken issues a new token for a caller holding a valid one. The
// account is re-read so deactivated users and role changes take effect.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	if user := h.currentUser(c); user != nil {
		h.respondWithToken(c, http.StatusOK, user)
	}
}

// GetProfile returns the current user's profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
	if user := h.currentUser(c); user != nil {
		c.JSON(http.StatusOK, user.Info())
	}
}

// ChangePassword changes the current user's password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req models.ChangePasswordRequest
	if !bindJSON(c, h.validator, &req) {
		return
	}
	user := h.currentUser(c)
	if user == nil {
		return
	}
	if !h.passwords.Verify(user.PasswordHash, req.CurrentPassword) {
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
	if err := h.users.SetPassword(c.Request.Context(), uuid.MustParse(user.ID), hash); err != nil {
		storeError(c, "update password", "User", err)
		return
	}
	logger.LogSecurityEvent("password_changed", user.ID, nil)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) respondWithToken(c *gin.Context, status int, user *models.User) {
	roles := make([]auth.Role, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, auth.Role(r))
	}
	token, expiresAt, err := h.tokens.Issue(auth.Principal{UserID: user.ID, Email: user.Email, Roles: roles})
	if err != nil {
		internalError(c, "issue token", err)
		return
	}
	c.JSON(status, models.AuthResponse{Token: token, ExpiresAt: expiresAt, User: user.Info()})
}

// NormalizeEmail lower-cases and trims an email address. Accounts are
// stored and looked up in this form.
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
