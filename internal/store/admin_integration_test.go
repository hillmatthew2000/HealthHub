package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/models"
)

func TestUsersKeepAnAdministrator(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	users := NewUsers(pool)

	// Without any administrator (a fresh install) other accounts can still
	// be changed.
	nurse := createUser(t, users, "nurse@example.org", "nurse")
	if _, err := users.Update(ctx, uuid.MustParse(nurse.ID), UserUpdate{Active: ptr(false)}); err != nil {
		t.Fatalf("update without administrators: %v", err)
	}

	a := createUser(t, users, "a@example.org", "admin")
	b := createUser(t, users, "b@example.org", "admin", "practitioner")
	if _, err := users.Update(ctx, uuid.MustParse(b.ID), UserUpdate{Roles: &[]string{"practitioner"}}); err != nil {
		t.Fatal(err)
	}
	for _, upd := range []UserUpdate{{Roles: &[]string{"nurse"}}, {Active: ptr(false)}} {
		if _, err := users.Update(ctx, uuid.MustParse(a.ID), upd); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("demoting the last administrator: err = %v, want ErrLastAdmin", err)
		}
	}
	if got, _ := users.Get(ctx, uuid.MustParse(a.ID)); !got.Active || len(got.Roles) != 1 || got.Roles[0] != "admin" {
		t.Errorf("rejected update was applied: %+v", got)
	}
	if _, err := users.Update(ctx, uuid.New(), UserUpdate{Active: ptr(true)}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user: err = %v", err)
	}
}

// TestUsersConcurrentDemotions demotes two administrators at once. The
// row locks must serialise the updates so exactly one succeeds.
func TestUsersConcurrentDemotions(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	users := NewUsers(pool)
	admins := []*models.User{
		createUser(t, users, "a@example.org", "admin"),
		createUser(t, users, "b@example.org", "admin"),
	}

	for range 10 {
		var wg sync.WaitGroup
		errs := make([]error, len(admins))
		for i, u := range admins {
			wg.Go(func() {
				_, errs[i] = users.Update(ctx, uuid.MustParse(u.ID), UserUpdate{Roles: &[]string{"nurse"}})
			})
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case !errors.Is(err, ErrLastAdmin):
				t.Fatal(err)
			}
		}
		if n, _ := users.CountAdmins(ctx); ok != 1 || n != 1 {
			t.Fatalf("%d demotions succeeded, %d administrators left", ok, n)
		}
		// Restore both administrators for the next round.
		for _, u := range admins {
			if _, err := users.Update(ctx, uuid.MustParse(u.ID), UserUpdate{Roles: &[]string{"admin"}}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestAuditLog(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	log := NewAuditLog(pool)
	log.now = ticker()

	actor := uuid.NewString()
	p1, p2 := uuid.NewString(), uuid.NewString()
	events := []models.AuditEvent{
		{ActorID: actor, Action: "create", ResourceType: "patient", ResourceIDs: []string{p1}, RequestID: "req-1", ClientIP: "203.0.113.9"},
		{ActorID: actor, Action: "list", ResourceType: "patient", ResourceIDs: []string{p1, p2}, ClientIP: "2001:db8::1"},
		{Action: "read", ResourceType: "observation", ResourceIDs: []string{uuid.NewString()}, ClientIP: "not an address"},
	}
	for _, e := range events {
		if err := log.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Append(ctx, models.AuditEvent{Action: "read", ResourceType: "patient", ResourceIDs: []string{"x"}}); err == nil {
		t.Error("a malformed resource ID was accepted")
	}
	if err := log.Append(ctx, models.AuditEvent{Action: "export", ResourceType: "patient"}); err == nil {
		t.Error("an unknown action was accepted")
	}

	all, total, err := log.List(ctx, AuditFilter{}, Page{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || all[0].Action != "read" || all[2].Action != "create" {
		t.Fatalf("List = %d %+v", total, all)
	}
	first := all[2]
	if first.ActorID != actor || first.RequestID != "req-1" || first.ClientIP != "203.0.113.9" || first.ResourceIDs[0] != p1 {
		t.Errorf("event = %+v", first)
	}
	if all[1].ClientIP != "2001:db8::1" || all[0].ClientIP != "" || all[0].ActorID != "" {
		t.Errorf("events = %+v", all[:2])
	}

	byResource, total, err := log.List(ctx, AuditFilter{ResourceID: ptr(uuid.MustParse(p2))}, Page{Limit: 10})
	if err != nil || total != 1 || byResource[0].Action != "list" {
		t.Errorf("by resource = %d %+v, %v", total, byResource, err)
	}
	from := first.OccurredAt.Add(time.Second)
	_, total, err = log.List(ctx, AuditFilter{ActorID: ptr(uuid.MustParse(actor)), From: &from, ResourceType: "patient"}, Page{Limit: 10})
	if err != nil || total != 1 {
		t.Errorf("by actor and time = %d, %v", total, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE audit_log SET action = 'delete'`); err == nil {
		t.Error("audit rows can be modified")
	}
}
