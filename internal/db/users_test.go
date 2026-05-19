//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupTestDB spins up a fresh Postgres, runs migrations, and returns a Queries handle.
// The cleanup function shuts down the container and the pool.
func setupTestDB(t *testing.T) (*dbgen.Queries, *pgxpool.Pool, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	pgC, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("diarion_test"),
		postgres.WithUsername("diarion"),
		postgres.WithPassword("diarion"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		cancel()
		t.Fatalf("start postgres: %v", err)
	}

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("dsn: %v", err)
	}

	if err := db.Migrate(ctx, dsn); err != nil {
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("migrate: %v", err)
	}

	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("pool: %v", err)
	}

	cleanup := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}

	return dbgen.New(pool), pool, cleanup
}

func TestUsers_InsertAndFetch(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	avatar := "https://example.com/avatar.png"
	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:     "google-sub-1",
		Email:         "alice@example.com",
		EmailVerified: true,
		DisplayName:   "Alice",
		AvatarURL:     &avatar,
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}
	if u.ID == 0 {
		t.Errorf("ID not assigned")
	}

	byID, err := q.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID.Email != "alice@example.com" {
		t.Errorf("email mismatch: %q", byID.Email)
	}

	bySub, err := q.GetUserByGoogleSub(ctx, "google-sub-1")
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if bySub.ID != u.ID {
		t.Errorf("id mismatch")
	}

	byEmail, err := q.GetUserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if byEmail.ID != u.ID {
		t.Errorf("email lookup id mismatch")
	}
}

func TestUsers_MarkDeletedAndRestore(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "g-2",
		Email:       "bob@example.com",
		DisplayName: "Bob",
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}

	if err := q.MarkUserDeleted(ctx, u.ID); err != nil {
		t.Fatalf("MarkUserDeleted: %v", err)
	}

	got, err := q.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID after delete: %v", err)
	}
	if !got.DeletedAt.Valid {
		t.Errorf("DeletedAt should be set")
	}
	if !got.HardDeleteAt.Valid {
		t.Errorf("HardDeleteAt should be set")
	}

	if err := q.RestoreUser(ctx, u.ID); err != nil {
		t.Fatalf("RestoreUser: %v", err)
	}

	restored, err := q.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID after restore: %v", err)
	}
	if restored.DeletedAt.Valid {
		t.Errorf("DeletedAt should be cleared after restore")
	}
	if restored.HardDeleteAt.Valid {
		t.Errorf("HardDeleteAt should be cleared after restore")
	}
}
