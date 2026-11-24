package store

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hillmatthew2000/HealthHub/internal/models"
)

// AuditFilter narrows an audit trail listing. Zero fields match everything.
type AuditFilter struct {
	ActorID      *uuid.UUID
	ResourceID   *uuid.UUID
	ResourceType string
	From, To     *time.Time
}

// AuditLog is the append-only audit trail.
type AuditLog struct {
	db  DB
	now func() time.Time
}

// NewAuditLog returns an audit trail repository.
func NewAuditLog(db DB) *AuditLog {
	return &AuditLog{db: db, now: time.Now}
}

// Append writes e. OccurredAt defaults to the current time.
func (r *AuditLog) Append(ctx context.Context, e models.AuditEvent) error {
	actor, err := optionalUUID(e.ActorID)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(e.ResourceIDs))
	for _, s := range e.ResourceIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return fmt.Errorf("store: invalid resource ID %q in audit event", s)
		}
		ids = append(ids, id)
	}
	var ip *netip.Addr
	if a, err := netip.ParseAddr(e.ClientIP); err == nil {
		ip = &a
	}
	occurred := e.OccurredAt
	if occurred.IsZero() {
		occurred = r.now()
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_log (occurred_at, actor_id, action, resource_type, resource_ids, request_id, client_ip)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)`,
		occurred.UTC(), actor, e.Action, e.ResourceType, ids, e.RequestID, ip)
	return mapError(err)
}

// List returns matching events, newest first, and the number of matches.
func (r *AuditLog) List(ctx context.Context, f AuditFilter, page Page) ([]models.AuditEvent, int, error) {
	const where = `
		WHERE ($1::uuid IS NULL OR actor_id = $1)
		  AND ($2::uuid IS NULL OR resource_ids @> ARRAY[$2::uuid])
		  AND ($3 = '' OR resource_type = $3)
		  AND ($4::timestamptz IS NULL OR occurred_at >= $4)
		  AND ($5::timestamptz IS NULL OR occurred_at <= $5)`
	args := []any{f.ActorID, f.ResourceID, f.ResourceType, f.From, f.To}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM audit_log`+where, args...).Scan(&total); err != nil {
		return nil, 0, mapError(err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, occurred_at, actor_id, action, resource_type, resource_ids, request_id, client_ip
		FROM audit_log`+where+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT $6 OFFSET $7`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, mapError(err)
	}
	events, err := pgx.CollectRows(rows, scanAuditEvent)
	if err != nil {
		return nil, 0, mapError(err)
	}
	return events, total, nil
}

func scanAuditEvent(row pgx.CollectableRow) (models.AuditEvent, error) {
	var (
		e         models.AuditEvent
		actor     *uuid.UUID
		ids       []uuid.UUID
		requestID *string
		ip        *netip.Addr
	)
	if err := row.Scan(&e.ID, &e.OccurredAt, &actor, &e.Action, &e.ResourceType, &ids, &requestID, &ip); err != nil {
		return e, err
	}
	if actor != nil {
		e.ActorID = actor.String()
	}
	e.ResourceIDs = make([]string, len(ids))
	for i, id := range ids {
		e.ResourceIDs[i] = id.String()
	}
	if requestID != nil {
		e.RequestID = *requestID
	}
	if ip != nil {
		e.ClientIP = ip.String()
	}
	return e, nil
}
