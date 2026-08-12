package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationLockKey int64 = 724918263

// Migrate applies embedded SQL migrations in lexicographic version order. Each
// migration runs inside its own transaction and its version is recorded in
// schema_migrations so already-applied versions are skipped on later starts.
// The session advisory lock serializes this process across service instances.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	locked := false
	defer func() {
		if locked {
			releaseMigrationLock(conn)
			return
		}
		conn.Release()
	}()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	locked = true

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text primary key,
			applied_at timestamptz not null default now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	names, err := fs.Glob(migrationsFS, "migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		version := name[len("migrations/") : len(name)-len(".up.sql")]
		applied, err := isApplied(ctx, conn, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, conn, version, string(body)); err != nil {
			return err
		}
	}
	return nil
}

func releaseMigrationLock(conn *pgxpool.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var unlocked bool
	err := conn.QueryRow(ctx,
		"SELECT pg_advisory_unlock($1)", migrationLockKey,
	).Scan(&unlocked)
	if err != nil || !unlocked {
		_ = conn.Conn().Close(ctx)
	}
	conn.Release()
}

func isApplied(ctx context.Context, conn *pgxpool.Conn, version string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)",
		version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, version, body string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, body); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version) VALUES ($1)",
		version,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
