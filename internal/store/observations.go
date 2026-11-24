package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/pkg/encryption"
)

const observationsTable = "observations"

// Observations stores observation resources, encrypted at rest.
type Observations struct {
	db  DB
	phi phi
	now func() time.Time
}

// NewObservations returns an observation repository.
func NewObservations(db DB, keys *encryption.Keys) *Observations {
	return &Observations{db: db, phi: phi{keys: keys}, now: time.Now}
}

// ObservationFilter narrows an observation listing.
type ObservationFilter struct {
	PatientID *uuid.UUID
	Status    string
	// Category matches a category code such as "laboratory".
	Category string
	// Code matches a coding exactly, either as "system|code" or as a bare
	// code.
	Code string
	From *time.Time
	To   *time.Time
}

const observationColumns = `id, patient_id, status, effective_at, resource, created_at, updated_at, created_by`

// categoryCodes returns the normalised category codes of o.
func categoryCodes(o *models.Observation) []string {
	out := []string{}
	for _, c := range o.Category {
		for _, coding := range c.Coding {
			if code := strings.ToLower(strings.TrimSpace(coding.Code)); code != "" {
				out = append(out, code)
			}
		}
	}
	return out
}

// codeIndexes returns blind indexes of every coding of the observation
// code, both qualified by system and bare.
func (r *Observations) codeIndexes(o *models.Observation) []string {
	var values []string
	for _, c := range o.Code.Coding {
		if c.Code == "" {
			continue
		}
		values = append(values, c.Code)
		if c.System != "" {
			values = append(values, c.System+"|"+c.Code)
		}
	}
	return r.phi.indexes(indexObsCode, NormaliseCode, values...)
}

func observationBody(o *models.Observation) models.Observation {
	b := *o
	b.ID, b.CreatedBy = "", ""
	b.CreatedAt, b.UpdatedAt = time.Time{}, time.Time{}
	return b
}

func patientRef(o *models.Observation) (uuid.UUID, error) {
	s, ok := o.Subject.PatientID()
	if !ok {
		return uuid.Nil, fmt.Errorf("%w: subject must reference a patient", ErrInvalidReference)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: subject must reference a patient", ErrInvalidReference)
	}
	return id, nil
}

// Create inserts o, assigning its ID and timestamps. It returns
// ErrInvalidReference if the subject patient does not exist.
func (r *Observations) Create(ctx context.Context, o *models.Observation) error {
	patientID, err := patientRef(o)
	if err != nil {
		return err
	}
	createdBy, err := optionalUUID(o.CreatedBy)
	if err != nil {
		return err
	}
	id := uuid.New()
	sealed, err := r.phi.seal(observationsTable, id.String(), observationBody(o))
	if err != nil {
		return err
	}
	now := r.now().UTC()
	_, err = r.db.Exec(ctx, `
		INSERT INTO observations (id, patient_id, status, category, code_bidx, effective_at, resource,
		                          created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)`,
		id, patientID, o.Status, categoryCodes(o), r.codeIndexes(o), o.EffectiveDateTime.UTC(), sealed, now, createdBy)
	if err != nil {
		return mapError(err)
	}
	o.ID, o.CreatedAt, o.UpdatedAt = id.String(), now, now
	return nil
}

func (r *Observations) scan(row pgx.Row) (*models.Observation, error) {
	var (
		id          uuid.UUID
		patientID   uuid.UUID
		status      string
		effectiveAt time.Time
		sealed      []byte
		createdAt   time.Time
		updatedAt   time.Time
		createdBy   *uuid.UUID
	)
	if err := row.Scan(&id, &patientID, &status, &effectiveAt, &sealed, &createdAt, &updatedAt, &createdBy); err != nil {
		return nil, mapError(err)
	}
	var o models.Observation
	if err := r.phi.open(observationsTable, id.String(), sealed, &o); err != nil {
		return nil, err
	}
	o.ID, o.Status = id.String(), status
	o.Subject.Reference = models.PatientReferencePrefix + patientID.String()
	o.CreatedAt, o.UpdatedAt = createdAt, updatedAt
	if createdBy != nil {
		o.CreatedBy = createdBy.String()
	}
	return &o, nil
}

// Get returns the observation with the given ID.
func (r *Observations) Get(ctx context.Context, id uuid.UUID) (*models.Observation, error) {
	return r.scan(r.db.QueryRow(ctx, `SELECT `+observationColumns+` FROM observations WHERE id = $1`, id))
}

// List returns the observations matching f, most recent first, and the
// total number of matches.
func (r *Observations) List(ctx context.Context, f ObservationFilter, page Page) ([]models.Observation, int, error) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, arg any) {
		args = append(args, arg)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.PatientID != nil {
		add("patient_id = $%d", *f.PatientID)
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Category != "" {
		add("category @> ARRAY[$%d::text]", strings.ToLower(strings.TrimSpace(f.Category)))
	}
	if f.Code != "" {
		add("code_bidx @> ARRAY[$%d::text]", r.phi.index(indexObsCode, NormaliseCode(f.Code)))
	}
	if f.From != nil {
		add("effective_at >= $%d", f.From.UTC())
	}
	if f.To != nil {
		add("effective_at <= $%d", f.To.UTC())
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM observations`+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapError(err)
	}
	args = append(args, page.Limit, page.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM observations%s ORDER BY effective_at DESC, id LIMIT $%d OFFSET $%d`,
		observationColumns, cond, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, mapError(err)
	}
	defer rows.Close()
	out := []models.Observation{}
	for rows.Next() {
		o, err := r.scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *o)
	}
	return out, total, mapError(rows.Err())
}

// Update replaces the observation's resource; o is updated with the stored
// metadata. It returns ErrInvalidReference if the subject patient does not
// exist.
func (r *Observations) Update(ctx context.Context, o *models.Observation) error {
	id, err := uuid.Parse(o.ID)
	if err != nil {
		return ErrNotFound
	}
	patientID, err := patientRef(o)
	if err != nil {
		return err
	}
	sealed, err := r.phi.seal(observationsTable, id.String(), observationBody(o))
	if err != nil {
		return err
	}
	var (
		createdAt time.Time
		createdBy *uuid.UUID
	)
	now := r.now().UTC()
	err = r.db.QueryRow(ctx, `
		UPDATE observations SET patient_id = $2, status = $3, category = $4, code_bidx = $5,
		       effective_at = $6, resource = $7, updated_at = $8
		WHERE id = $1
		RETURNING created_at, created_by`,
		id, patientID, o.Status, categoryCodes(o), r.codeIndexes(o), o.EffectiveDateTime.UTC(), sealed, now).
		Scan(&createdAt, &createdBy)
	if err != nil {
		return mapError(err)
	}
	o.CreatedAt, o.UpdatedAt, o.CreatedBy = createdAt, now, ""
	if createdBy != nil {
		o.CreatedBy = createdBy.String()
	}
	return nil
}

// Delete removes the observation.
func (r *Observations) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM observations WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
