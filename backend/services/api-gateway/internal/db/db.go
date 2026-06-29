// Package db owns all PostgreSQL access for the gateway: connection pooling,
// embedded schema migrations, and the repository methods services build on.
// Every query is parameterized; no SQL is ever assembled from user input.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Common repository errors. Callers branch on these with errors.Is.
var (
	ErrNotFound      = errors.New("db: not found")
	ErrUsernameTaken = errors.New("db: username already taken")
	ErrEmailTaken    = errors.New("db: email already registered")
	ErrSlugTaken     = errors.New("db: contest slug already taken")
)

// Store provides repository access backed by a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps an existing pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Ping verifies database connectivity (readiness probes).
func (s *Store) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}
	return nil
}

// Connect opens a pgx pool and verifies connectivity. Queries emit OTel
// spans (a no-op when tracing is disabled).
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse pool config: %w", err)
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping (is PostgreSQL running? task infra:up): %w", err)
	}
	return pool, nil
}

// Migrate applies all pending embedded migrations. A Postgres session lock
// makes concurrent replicas racing at startup safe: one migrates, the rest
// wait.
func Migrate(ctx context.Context, dsn string, log *slog.Logger) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("db: open migration connection: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Warn("close migration connection", "error", err)
		}
	}()

	fsys, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("db: sub migrations fs: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("db: create session locker: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("db: create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("db: apply migrations: %w", err)
	}
	for _, r := range results {
		log.Info("migration applied", "version", r.Source.Version, "path", r.Source.Path, "duration", r.Duration.Round(time.Millisecond).String())
	}
	return nil
}
