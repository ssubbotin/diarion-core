// Package db owns the Postgres pool and the schema migration runner.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Migrate runs all pending up migrations against the database at databaseURL.
// It uses a Postgres advisory lock (held internally by golang-migrate) so concurrent
// callers (e.g. multiple replicas booting at once) serialise correctly.
func Migrate(ctx context.Context, databaseURL string) error {
	sub, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("db.Migrate: fs.Sub: %w", err)
	}

	src, err := iofs.New(sub, ".")
	if err != nil {
		return fmt.Errorf("db.Migrate: iofs source: %w", err)
	}

	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("db.Migrate: parse url: %w", err)
	}
	sqlDB := stdlib.OpenDB(*cfg)
	defer func() { _ = sqlDB.Close() }()

	driver, err := migratepg.WithInstance(sqlDB, &migratepg.Config{})
	if err != nil {
		return fmt.Errorf("db.Migrate: pg driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("db.Migrate: new instance: %w", err)
	}

	slog.InfoContext(ctx, "running migrations")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db.Migrate: up: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("db.Migrate: read version: %w", err)
	}
	slog.InfoContext(ctx, "migrations applied", "version", version, "dirty", dirty)

	return nil
}
