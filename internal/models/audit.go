package models

import "time"

// AuditEvent is one entry in the audit trail of access to protected health
// information and to accounts. It names records by ID and never holds PHI.
type AuditEvent struct {
	ID           int64     `json:"id"`
	OccurredAt   time.Time `json:"occurredAt"`
	ActorID      string    `json:"actorId,omitempty"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resourceType"`
	ResourceIDs  []string  `json:"resourceIds"`
	RequestID    string    `json:"requestId,omitempty"`
	ClientIP     string    `json:"clientIp,omitempty"`
}
