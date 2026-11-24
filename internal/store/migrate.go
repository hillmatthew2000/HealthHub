package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockID is the pg_advisory_lock key that serialises migrations
// when several replicas start at once ("HEALTHHU" in ASCII).
const migrationLockID int64 = 0x4845414c54484855

// Migration is one versioned schema change.
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

// Migrations returns the embedded migrations in version order. File names
// have the form NNNN_description.sql.
func Migrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	out := make([]Migration, 0, len(entries))
	seen := map[int64]string{}
	for _, e := range entries {
		name := e.Name()
		prefix, rest, ok := strings.Cut(strings.TrimSuffix(name, ".sql"), "_")
		if !ok {
			return nil, fmt.Errorf("store: migration %q is not named NNNN_description.sql", name)
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("store: migration %q has an invalid version", name)
		}
		if other, dup := seen[v]; dup {
			return nil, fmt.Errorf("store: migrations %q and %q share version %d", other, name, v)
		}
		seen[v] = name
		body, err := migrationFiles.ReadFile(path.Join("migrations", name))
		if err != nil {
			return nil, err
		}
		out = append(out, Migration{Version: v, Name: rest, SQL: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Migrate applies every pending migration, each in its own transaction, and
// returns the versions it applied. It holds an advisory lock for the
// duration so concurrent callers wait rather than race.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]int64, error) {
	migrations, err := Migrations()
	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return nil, fmt.Errorf("store: take migration lock: %w", err)
	}
	defer func() {
		// Use a fresh context: ctx may already be cancelled.
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockID)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    bigint      PRIMARY KEY,
			name       text        NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("store: create schema_migrations: %w", err)
	}

	applied := map[int64]bool{}
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("store: read schema_migrations: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("store: read schema_migrations: %w", err)
	}
	for _, v := range versions {
		applied[v] = true
	}

	var done []int64
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.SQL); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name)
			return err
		})
		if err != nil {
			return done, fmt.Errorf("store: apply migration %d_%s: %w", m.Version, m.Name, err)
		}
		done = append(done, m.Version)
	}
	return done, nil
}
