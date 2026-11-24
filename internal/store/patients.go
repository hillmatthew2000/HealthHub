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

const patientsTable = "patients"

// Patients stores patient resources, encrypted at rest.
type Patients struct {
	db  DB
	phi phi
	now func() time.Time
}

// NewPatients returns a patient repository.
func NewPatients(db DB, keys *encryption.Keys) *Patients {
	return &Patients{db: db, phi: phi{keys: keys}, now: time.Now}
}

// PatientFilter narrows a patient listing. All string filters are exact
// matches (after normalisation), because the underlying values are only
// stored encrypted.
type PatientFilter struct {
	Family    string
	BirthDate *models.Date
	Telecom   string
	Gender    string
	Active    *bool
}

const patientColumns = `id, active, gender, resource, created_at, updated_at, created_by`

// patientIndexes holds the blind indexes derived from a patient.
type patientIndexes struct {
	family    []string
	birthDate *string
	telecom   []string
}

func (r *Patients) indexes(p *models.Patient) patientIndexes {
	families := make([]string, 0, len(p.Name))
	for _, n := range p.Name {
		families = append(families, n.Family)
	}
	telecom := make([]string, 0, len(p.Telecom))
	for _, c := range p.Telecom {
		if idx := r.phi.index(indexTelecom, NormaliseTelecom(c.System, c.Value)); idx != "" {
			telecom = append(telecom, idx)
		}
	}
	ix := patientIndexes{
		family:  r.phi.indexes(indexFamily, NormaliseName, families...),
		telecom: telecom,
	}
	if p.BirthDate != nil {
		b := r.phi.index(indexBirthDate, p.BirthDate.String())
		ix.birthDate = &b
	}
	return ix
}

// body returns the part of p that is encrypted: everything except the
// identifiers and bookkeeping fields, which live in their own columns.
func patientBody(p *models.Patient) models.Patient {
	b := *p
	b.ID, b.CreatedBy = "", ""
	b.CreatedAt, b.UpdatedAt = time.Time{}, time.Time{}
	return b
}

// Create inserts p, assigning its ID and timestamps.
func (r *Patients) Create(ctx context.Context, p *models.Patient) error {
	id := uuid.New()
	createdBy, err := optionalUUID(p.CreatedBy)
	if err != nil {
		return err
	}
	body := patientBody(p)
	sealed, err := r.phi.seal(patientsTable, id.String(), body)
	if err != nil {
		return err
	}
	ix := r.indexes(p)
	now := r.now().UTC()
	_, err = r.db.Exec(ctx, `
		INSERT INTO patients (id, active, gender, resource, family_bidx, birthdate_bidx, telecom_bidx,
		                      created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)`,
		id, p.Active, p.Gender, sealed, ix.family, ix.birthDate, ix.telecom, now, createdBy)
	if err != nil {
		return mapError(err)
	}
	p.ID, p.CreatedAt, p.UpdatedAt = id.String(), now, now
	return nil
}

func (r *Patients) scan(row pgx.Row) (*models.Patient, error) {
	var (
		id        uuid.UUID
		active    bool
		gender    string
		sealed    []byte
		createdAt time.Time
		updatedAt time.Time
		createdBy *uuid.UUID
	)
	if err := row.Scan(&id, &active, &gender, &sealed, &createdAt, &updatedAt, &createdBy); err != nil {
		return nil, mapError(err)
	}
	var p models.Patient
	if err := r.phi.open(patientsTable, id.String(), sealed, &p); err != nil {
		return nil, err
	}
	p.ID, p.Active, p.Gender = id.String(), active, gender
	p.CreatedAt, p.UpdatedAt = createdAt, updatedAt
	if createdBy != nil {
		p.CreatedBy = createdBy.String()
	}
	return &p, nil
}

// Get returns the patient with the given ID.
func (r *Patients) Get(ctx context.Context, id uuid.UUID) (*models.Patient, error) {
	return r.scan(r.db.QueryRow(ctx, `SELECT `+patientColumns+` FROM patients WHERE id = $1`, id))
}

// Exists reports whether a patient with the given ID exists.
func (r *Patients) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM patients WHERE id = $1)`, id).Scan(&ok)
	return ok, mapError(err)
}

// List returns the patients matching f, newest first, and the total number
// of matches.
func (r *Patients) List(ctx context.Context, f PatientFilter, page Page) ([]models.Patient, int, error) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, arg any) {
		args = append(args, arg)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Family != "" {
		add("family_bidx @> ARRAY[$%d::text]", r.phi.index(indexFamily, NormaliseName(f.Family)))
	}
	if f.BirthDate != nil {
		add("birthdate_bidx = $%d", r.phi.index(indexBirthDate, f.BirthDate.String()))
	}
	if f.Telecom != "" {
		// The contact system is unknown at search time, so match either
		// normalisation.
		add("telecom_bidx && $%d::text[]", r.phi.indexes(indexTelecom, func(s string) string { return s },
			NormaliseTelecom("phone", f.Telecom), NormaliseTelecom("email", f.Telecom)))
	}
	if f.Gender != "" {
		add("gender = $%d", f.Gender)
	}
	if f.Active != nil {
		add("active = $%d", *f.Active)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM patients`+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapError(err)
	}
	args = append(args, page.Limit, page.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM patients%s ORDER BY created_at DESC, id LIMIT $%d OFFSET $%d`,
		patientColumns, cond, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, mapError(err)
	}
	defer rows.Close()
	patients := []models.Patient{}
	for rows.Next() {
		p, err := r.scan(rows)
		if err != nil {
			return nil, 0, err
		}
		patients = append(patients, *p)
	}
	return patients, total, mapError(rows.Err())
}

// Update replaces the patient's resource. ID and creation metadata are
// preserved; p is updated with the stored values.
func (r *Patients) Update(ctx context.Context, p *models.Patient) error {
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return ErrNotFound
	}
	sealed, err := r.phi.seal(patientsTable, id.String(), patientBody(p))
	if err != nil {
		return err
	}
	ix := r.indexes(p)
	var (
		createdAt time.Time
		createdBy *uuid.UUID
	)
	now := r.now().UTC()
	err = r.db.QueryRow(ctx, `
		UPDATE patients SET active = $2, gender = $3, resource = $4, family_bidx = $5,
		       birthdate_bidx = $6, telecom_bidx = $7, updated_at = $8
		WHERE id = $1
		RETURNING created_at, created_by`,
		id, p.Active, p.Gender, sealed, ix.family, ix.birthDate, ix.telecom, now).Scan(&createdAt, &createdBy)
	if err != nil {
		return mapError(err)
	}
	p.CreatedAt, p.UpdatedAt, p.CreatedBy = createdAt, now, ""
	if createdBy != nil {
		p.CreatedBy = createdBy.String()
	}
	return nil
}

// Delete removes the patient and, through the foreign key, all of the
// patient's observations.
func (r *Patients) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM patients WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
