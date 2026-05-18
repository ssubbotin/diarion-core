//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrate_AppliesInitialSchema(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("diarion_test"),
		postgres.WithUsername("diarion"),
		postgres.WithPassword("diarion"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := db.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Re-running should be a no-op (ErrNoChange handled internally).
	if err := db.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate (re-run): %v", err)
	}
}
