package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/store"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// UserHandler serves the account administration endpoints.
type UserHandler struct {
	users     UserStore
	passwords *auth.PasswordHasher
	audit     Auditor
	paging    Paging
	validator *validator.Validate
}

// NewUserHandler returns an account administration handler.
func NewUserHandler(users UserStore, passwords *auth.PasswordHasher, audit Auditor, paging Paging) *UserHandler {
	return &UserHandler{users: users, passwords: passwords, audit: audit, paging: paging, validator: newValidator()}
}

// normalizeRoles removes duplicates and sorts roles so they are stored in
// one canonical form.
func normalizeRoles(roles []string) []string {
	out := slices.Clone(roles)
	if out == nil {
		out = []string{}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// CreateUser handles POST /users.
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
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
		Roles:        normalizeRoles(req.Roles),
		Active:       true,
	}
	if p, ok := auth.PrincipalFrom(c); ok {
		user.CreatedBy = p.UserID
	}
	if err := h.users.Create(c.Request.Context(), user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			problem.Write(c, problem.Conflict("An account with this email address already exists."))
			return
		}
		internalError(c, "create user", err)
		return
	}
	audit(c, h.audit, ActionCreate, "user", user.ID)
	logger.LogSecurityEvent("user_created", user.ID, map[string]any{"roles": user.Roles, "by": user.CreatedBy})
	c.Header("Location", c.FullPath()+"/"+user.ID)
	c.JSON(http.StatusCreated, user.Info())
}

// GetUsers handles GET /users.
func (h *UserHandler) GetUsers(c *gin.Context) {
	var errs queryErrors
	page, pageNo, limit := h.paging.page(c, &errs)
	if errs.write(c) {
		return
	}
	users, total, err := h.users.List(c.Request.Context(), page)
	if err != nil {
		internalError(c, "list users", err)
		return
	}
	infos := make([]models.UserInfo, len(users))
	for i := range users {
		infos[i] = users[i].Info()
	}
	c.JSON(http.StatusOK, newPage(infos, pageNo, limit, total))
}

// GetUser handles GET /users/:id.
func (h *UserHandler) GetUser(c *gin.Context) {
	id, ok := pathID(c, "id", "User")
	if !ok {
		return
	}
	user, err := h.users.Get(c.Request.Context(), id)
	if err != nil {
		storeError(c, "get user", "User", err)
		return
	}
	c.JSON(http.StatusOK, user.Info())
}

// UpdateUser handles PATCH /users/:id: changing an account's roles or
// deactivating it. Changes apply to new tokens; existing tokens keep their
// roles until they expire or are refreshed.
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, ok := pathID(c, "id", "User")
	if !ok {
		return
	}
	var req models.UpdateUserRequest
	if !bindJSON(c, h.validator, &req) {
		return
	}
	if req.Roles == nil && req.Active == nil {
		problem.Write(c, problem.Validation(problem.FieldError{Field: "body", Message: "must set roles or active"}))
		return
	}
	upd := store.UserUpdate{Active: req.Active}
	if req.Roles != nil {
		roles := normalizeRoles(*req.Roles)
		upd.Roles = &roles
	}
	user, err := h.users.Update(c.Request.Context(), id, upd)
	if errors.Is(err, store.ErrLastAdmin) {
		problem.Write(c, problem.Conflict("This change would leave no active administrator."))
		return
	}
	if err != nil {
		storeError(c, "update user", "User", err)
		return
	}
	audit(c, h.audit, ActionUpdate, "user", user.ID)
	details := map[string]any{"roles": user.Roles, "active": user.Active}
	if p, ok := auth.PrincipalFrom(c); ok {
		details["by"] = p.UserID
	}
	logger.LogSecurityEvent("user_updated", user.ID, details)
	c.JSON(http.StatusOK, user.Info())
}
