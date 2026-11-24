package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hillmatthew2000/HealthHub/internal/models"
)

// Users stores staff accounts.
type Users struct {
	db  DB
	now func() time.Time
}

// NewUsers returns a user repository.
func NewUsers(db DB) *Users {
	return &Users{db: db, now: time.Now}
}

const userColumns = `id, email, password_hash, first_name, last_name, roles, active,
	last_login_at, created_at, updated_at, created_by`

func scanUser(row pgx.Row) (*models.User, error) {
	var (
		u         models.User
		id        uuid.UUID
		createdBy *uuid.UUID
	)
	err := row.Scan(&id, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Roles, &u.Active,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &createdBy)
	if err != nil {
		return nil, mapError(err)
	}
	u.ID = id.String()
	if createdBy != nil {
		u.CreatedBy = createdBy.String()
	}
	return &u, nil
}

// Create inserts u, assigning its ID and timestamps. It returns ErrConflict
// if the email address is taken.
func (r *Users) Create(ctx context.Context, u *models.User) error {
	id := uuid.New()
	now := r.now().UTC()
	createdBy, err := optionalUUID(u.CreatedBy)
	if err != nil {
		return err
	}
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, roles, active,
		                   created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)`,
		id, u.Email, u.PasswordHash, u.FirstName, u.LastName, roles, u.Active, now, createdBy)
	if err != nil {
		return mapError(err)
	}
	u.ID, u.Roles, u.CreatedAt, u.UpdatedAt = id.String(), roles, now, now
	return nil
}

// Get returns the user with the given ID.
func (r *Users) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// GetByEmail returns the user with the given (normalised) email address.
func (r *Users) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

// List returns users ordered by email, and the total number of users.
func (r *Users) List(ctx context.Context, page Page) ([]models.User, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, mapError(err)
	}
	rows, err := r.db.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY email LIMIT $1 OFFSET $2`,
		page.Limit, page.Offset)
	if err != nil {
		return nil, 0, mapError(err)
	}
	defer rows.Close()
	users := []models.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *u)
	}
	return users, total, mapError(rows.Err())
}

// UserUpdate lists the fields an administrator may change. Nil fields are
// left as they are.
type UserUpdate struct {
	Roles  *[]string
	Active *bool
}

// Update applies upd to the user and returns the updated row.
func (r *Users) Update(ctx context.Context, id uuid.UUID, upd UserUpdate) (*models.User, error) {
	var roles []string
	if upd.Roles != nil {
		roles = *upd.Roles
		if roles == nil {
			roles = []string{}
		}
	}
	return scanUser(r.db.QueryRow(ctx, `
		UPDATE users SET
			roles      = CASE WHEN $2::boolean THEN $3::text[] ELSE roles END,
			active     = COALESCE($4, active),
			updated_at = $5
		WHERE id = $1
		RETURNING `+userColumns,
		id, upd.Roles != nil, roles, upd.Active, r.now().UTC()))
}

// SetPassword replaces the user's password hash.
func (r *Users) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`,
		id, hash, r.now().UTC())
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordLogin stores the time of a successful login.
func (r *Users) RecordLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, id, r.now().UTC())
	return mapError(err)
}

// CountAdmins returns the number of active administrators.
func (r *Users) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE active AND 'admin' = ANY (roles)`).Scan(&n)
	return n, mapError(err)
}

// optionalUUID parses s, treating "" as NULL.
func optionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("store: invalid user ID %q", s)
	}
	return &id, nil
}
