package handlers

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// AuditAction is what was done to protected health information.
type AuditAction string

// Audited actions.
const (
	ActionCreate AuditAction = "create"
	ActionRead   AuditAction = "read"
	ActionList   AuditAction = "list"
	ActionUpdate AuditAction = "update"
	ActionDelete AuditAction = "delete"
)

// Auditor records access to protected health information: who touched
// which records, when, and how. Implementations must not record PHI itself.
type Auditor interface {
	Record(c *gin.Context, action AuditAction, resourceType string, ids ...string)
}

// LogAuditor writes audit events to the structured "audit" log.
type LogAuditor struct{}

// Record implements Auditor.
func (LogAuditor) Record(c *gin.Context, action AuditAction, resourceType string, ids ...string) {
	actor := ""
	if p, ok := auth.PrincipalFrom(c); ok {
		actor = p.UserID
	}
	logger.AuditLogger().Info("phi access",
		zap.String("action", string(action)),
		zap.String("resource_type", resourceType),
		zap.Strings("resource_ids", ids),
		zap.String("actor_id", actor),
		zap.String("request_id", c.GetString(problem.RequestIDKey)),
		zap.String("client_ip", c.ClientIP()),
	)
}
