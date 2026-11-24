package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/store"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// AuditAction is what was done to a record.
type AuditAction string

// Audited actions.
const (
	ActionCreate AuditAction = "create"
	ActionRead   AuditAction = "read"
	ActionList   AuditAction = "list"
	ActionUpdate AuditAction = "update"
	ActionDelete AuditAction = "delete"
)

// Auditor records access to protected health information and changes to
// accounts: who touched which records, when, and how. Implementations must
// not record PHI itself.
type Auditor interface {
	Record(c *gin.Context, action AuditAction, resourceType string, ids ...string) error
}

// event describes the access in c.
func event(c *gin.Context, action AuditAction, resourceType string, ids []string) models.AuditEvent {
	e := models.AuditEvent{
		OccurredAt:   time.Now().UTC(),
		Action:       string(action),
		ResourceType: resourceType,
		ResourceIDs:  ids,
		RequestID:    c.GetString(problem.RequestIDKey),
		ClientIP:     c.ClientIP(),
	}
	if p, ok := auth.PrincipalFrom(c); ok {
		e.ActorID = p.UserID
	}
	return e
}

func logEvent(e models.AuditEvent) {
	logger.AuditLogger().Info("audit event",
		zap.String("action", e.Action),
		zap.String("resource_type", e.ResourceType),
		zap.Strings("resource_ids", e.ResourceIDs),
		zap.String("actor_id", e.ActorID),
		zap.String("request_id", e.RequestID),
		zap.String("client_ip", e.ClientIP),
	)
}

// LogAuditor writes audit events to the structured "audit" log only.
type LogAuditor struct{}

// Record implements Auditor.
func (LogAuditor) Record(c *gin.Context, action AuditAction, resourceType string, ids ...string) error {
	logEvent(event(c, action, resourceType, ids))
	return nil
}

// AuditAppender stores audit events. *store.AuditLog implements it.
type AuditAppender interface {
	Append(ctx context.Context, e models.AuditEvent) error
}

// auditWriteTimeout bounds an audit write. The write is detached from the
// request deadline so a slow handler does not lose its audit record.
const auditWriteTimeout = 5 * time.Second

// StoreAuditor writes audit events to the database and to the "audit" log.
type StoreAuditor struct {
	Events AuditAppender
}

// Record implements Auditor.
func (a StoreAuditor) Record(c *gin.Context, action AuditAction, resourceType string, ids ...string) error {
	e := event(c, action, resourceType, ids)
	logEvent(e)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), auditWriteTimeout)
	defer cancel()
	return a.Events.Append(ctx, e)
}

// audit records an access and reports whether the handler may go on to
// respond. Reads fail closed: data is not disclosed unless its disclosure
// was recorded. A write has already happened by the time it is audited, so
// failing the request would misreport it; the failure is logged at error
// level instead, with the event, so the trail can be reconciled.
func audit(c *gin.Context, a Auditor, action AuditAction, resourceType string, ids ...string) bool {
	err := a.Record(c, action, resourceType, ids...)
	if err == nil {
		return true
	}
	if action == ActionRead || action == ActionList {
		internalError(c, "record audit event", err)
		return false
	}
	logger.AuditLogger().Error("audit event not stored",
		zap.String("action", string(action)),
		zap.String("resource_type", resourceType),
		zap.Strings("resource_ids", ids),
		zap.String("request_id", c.GetString(problem.RequestIDKey)),
		zap.Error(err),
	)
	return true
}

// AuditEventStore lists audit events. *store.AuditLog implements it.
type AuditEventStore interface {
	List(ctx context.Context, f store.AuditFilter, page store.Page) ([]models.AuditEvent, int, error)
}

// AuditHandler serves the audit trail to administrators.
type AuditHandler struct {
	events AuditEventStore
	paging Paging
}

// NewAuditHandler returns an audit trail handler.
func NewAuditHandler(events AuditEventStore, paging Paging) *AuditHandler {
	return &AuditHandler{events: events, paging: paging}
}

// GetAuditEvents handles GET /audit-events, newest first.
func (h *AuditHandler) GetAuditEvents(c *gin.Context) {
	var errs queryErrors
	page, pageNo, limit := h.paging.page(c, &errs)
	f := store.AuditFilter{
		ActorID:      queryUUID(c, "actor", &errs),
		ResourceID:   queryUUID(c, "resource", &errs),
		ResourceType: queryEnum(c, "resourceType", &errs, "patient", "observation", "user"),
		From:         queryTime(c, "from", false, &errs),
		To:           queryTime(c, "to", true, &errs),
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		errs.add("from", "must not be after to")
	}
	if errs.write(c) {
		return
	}
	events, total, err := h.events.List(c.Request.Context(), f, page)
	if err != nil {
		internalError(c, "list audit events", err)
		return
	}
	c.JSON(http.StatusOK, newPage(events, pageNo, limit, total))
}
