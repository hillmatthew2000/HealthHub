// Package store is the PostgreSQL data layer. It owns the schema (embedded,
// versioned migrations) and exposes one repository per aggregate. Protected
// health information is encrypted before it is written; see phi.go.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the repositories. Handlers map them to HTTP statuses.
var (
	// ErrNotFound means no row matched.
	ErrNotFound = errors.New("store: not found")
	// ErrConflict means a uniqueness constraint was violated.
	ErrConflict = errors.New("store: conflict")
	// ErrInvalidReference means a foreign key pointed at a missing row.
	ErrInvalidReference = errors.New("store: invalid reference")
	// ErrLastAdmin means a change would leave no active administrator.
	ErrLastAdmin = errors.New("store: at least one active administrator is required")
)

// PoolConfig configures the connection pool.
type PoolConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// Open connects to PostgreSQL and verifies the connection.
func Open(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		// The parse error can echo the URL, password included.
		return nil, errors.New("store: DATABASE_URL is not a valid PostgreSQL connection string")
	}
	if cfg.MaxConns > 0 {
		pc.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pc.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pc.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	pc.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pc.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		scanTimesInUTC(conn)
		return nil
	}
	if pc.ConnConfig.RuntimeParams["application_name"] == "" {
		pc.ConnConfig.RuntimeParams["application_name"] = "healthhub"
	}

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("store: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	return pool, nil
}

// scanTimesInUTC makes timestamptz values scan as UTC times. pgx otherwise
// returns them in the server process's local time zone, so API responses
// would carry whatever offset the host happens to use.
func scanTimesInUTC(conn *pgx.Conn) {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})
}

// DB is the subset of pgxpool.Pool and pgx.Tx used by the repositories.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Page selects a window of an ordered result.
type Page struct {
	Limit  int
	Offset int
}

// mapError translates driver errors into the package's sentinel errors.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: %s", ErrInvalidReference, pgErr.ConstraintName)
		}
	}
	return err
}
