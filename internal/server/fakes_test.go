package server_test

import (
	"context"
	"errors"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/store"
)

// The fakes below implement the repository interfaces in memory. They
// return the store package's sentinel errors so the handlers' error
// mapping is exercised exactly as it is against PostgreSQL. Setting err
// makes every call fail, to test the 500 path.

type fakeUsers struct {
	mu   sync.Mutex
	byID map[string]*models.User
	err  error
}

func newFakeUsers() *fakeUsers { return &fakeUsers{byID: map[string]*models.User{}} }

func (f *fakeUsers) Create(_ context.Context, u *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	for _, other := range f.byID {
		if other.Email == u.Email {
			return store.ErrConflict
		}
	}
	u.ID = uuid.NewString()
	if u.Roles == nil {
		u.Roles = []string{}
	}
	u.CreatedAt, u.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	cp := *u
	f.byID[u.ID] = &cp
	return nil
}

func (f *fakeUsers) Get(_ context.Context, id uuid.UUID) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.byID[id.String()]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	for _, u := range f.byID {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeUsers) List(_ context.Context, page store.Page) ([]models.User, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, 0, f.err
	}
	all := make([]models.User, 0, len(f.byID))
	for _, u := range f.byID {
		all = append(all, *u)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Email < all[j].Email })
	return window(all, page), len(all), nil
}

func (f *fakeUsers) Update(_ context.Context, id uuid.UUID, upd store.UserUpdate) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.byID[id.String()]
	if !ok {
		return nil, store.ErrNotFound
	}
	if upd.Roles != nil {
		u.Roles = append([]string{}, *upd.Roles...)
	}
	if upd.Active != nil {
		u.Active = *upd.Active
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUsers) SetPassword(_ context.Context, id uuid.UUID, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	u, ok := f.byID[id.String()]
	if !ok {
		return store.ErrNotFound
	}
	u.PasswordHash = hash
	return nil
}

func (f *fakeUsers) RecordLogin(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id.String()]; ok {
		now := time.Now().UTC()
		u.LastLoginAt = &now
	}
	return f.err
}

func (f *fakeUsers) CountAdmins(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	n := 0
	for _, u := range f.byID {
		if u.Active && slices.Contains(u.Roles, string(auth.RoleAdmin)) {
			n++
		}
	}
	return n, nil
}

type fakePatients struct {
	mu      sync.Mutex
	byID    map[string]*models.Patient
	err     error
	filters []store.PatientFilter
}

func newFakePatients() *fakePatients { return &fakePatients{byID: map[string]*models.Patient{}} }

func (f *fakePatients) Create(_ context.Context, p *models.Patient) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	p.ID = uuid.NewString()
	p.CreatedAt, p.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	cp := *p
	f.byID[p.ID] = &cp
	return nil
}

func (f *fakePatients) Get(_ context.Context, id uuid.UUID) (*models.Patient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	p, ok := f.byID[id.String()]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakePatients) Exists(_ context.Context, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.byID[id.String()]
	return ok, nil
}

// List records the filter it was given and ignores it, except for gender;
// the store's filtering is covered by its integration tests.
func (f *fakePatients) List(_ context.Context, filter store.PatientFilter, page store.Page) ([]models.Patient, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, 0, f.err
	}
	f.filters = append(f.filters, filter)
	var all []models.Patient
	for _, p := range f.byID {
		if filter.Gender == "" || p.Gender == filter.Gender {
			all = append(all, *p)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return window(all, page), len(all), nil
}

func (f *fakePatients) Update(_ context.Context, p *models.Patient) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	old, ok := f.byID[p.ID]
	if !ok {
		return store.ErrNotFound
	}
	p.CreatedAt, p.CreatedBy, p.UpdatedAt = old.CreatedAt, old.CreatedBy, time.Now().UTC()
	cp := *p
	f.byID[p.ID] = &cp
	return nil
}

func (f *fakePatients) Delete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byID[id.String()]; !ok {
		return store.ErrNotFound
	}
	delete(f.byID, id.String())
	return nil
}

type fakeObservations struct {
	mu       sync.Mutex
	byID     map[string]*models.Observation
	patients *fakePatients
	err      error
	filters  []store.ObservationFilter
}

func newFakeObservations(patients *fakePatients) *fakeObservations {
	return &fakeObservations{byID: map[string]*models.Observation{}, patients: patients}
}

// checkSubject mirrors the foreign key on observations.patient_id.
func (f *fakeObservations) checkSubject(o *models.Observation) error {
	id, _ := o.Subject.PatientID()
	uid, err := uuid.Parse(id)
	if err != nil {
		return store.ErrInvalidReference
	}
	ok, err := f.patients.Exists(context.Background(), uid)
	if err != nil {
		return err
	}
	if !ok {
		return store.ErrInvalidReference
	}
	return nil
}

func (f *fakeObservations) Create(_ context.Context, o *models.Observation) error {
	if err := f.checkSubject(o); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	o.ID = uuid.NewString()
	o.CreatedAt, o.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	cp := *o
	f.byID[o.ID] = &cp
	return nil
}

func (f *fakeObservations) Get(_ context.Context, id uuid.UUID) (*models.Observation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	o, ok := f.byID[id.String()]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *o
	return &cp, nil
}

func (f *fakeObservations) List(_ context.Context, filter store.ObservationFilter, page store.Page) ([]models.Observation, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, 0, f.err
	}
	f.filters = append(f.filters, filter)
	var all []models.Observation
	for _, o := range f.byID {
		if filter.PatientID != nil {
			if id, _ := o.Subject.PatientID(); id != filter.PatientID.String() {
				continue
			}
		}
		all = append(all, *o)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return window(all, page), len(all), nil
}

func (f *fakeObservations) Update(_ context.Context, o *models.Observation) error {
	if err := f.checkSubject(o); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	old, ok := f.byID[o.ID]
	if !ok {
		return store.ErrNotFound
	}
	o.CreatedAt, o.CreatedBy, o.UpdatedAt = old.CreatedAt, old.CreatedBy, time.Now().UTC()
	cp := *o
	f.byID[o.ID] = &cp
	return nil
}

func (f *fakeObservations) Delete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byID[id.String()]; !ok {
		return store.ErrNotFound
	}
	delete(f.byID, id.String())
	return nil
}

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

// auditEvent is one recorded call to the auditor.
type auditEvent struct {
	Actor        string
	Action       handlers.AuditAction
	ResourceType string
	IDs          []string
}

type fakeAuditor struct {
	mu     sync.Mutex
	events []auditEvent
}

func (a *fakeAuditor) Record(c *gin.Context, action handlers.AuditAction, resourceType string, ids ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, auditEvent{
		Actor:        c.GetString(auth.UserIDKey),
		Action:       action,
		ResourceType: resourceType,
		IDs:          append([]string{}, ids...),
	})
}

func (a *fakeAuditor) last() (auditEvent, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.events) == 0 {
		return auditEvent{}, false
	}
	return a.events[len(a.events)-1], true
}

func (a *fakeAuditor) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.events)
}

var errBoom = errors.New("connection reset by peer: SELECT secret FROM users")

func window[T any](all []T, page store.Page) []T {
	if page.Offset >= len(all) {
		return []T{}
	}
	end := min(page.Offset+page.Limit, len(all))
	return all[page.Offset:end]
}
