//go:build integration

package purge_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/purge"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupPool(t *testing.T) (*dbgen.Queries, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	pgC, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("diarion_test"),
		postgres.WithUsername("diarion"),
		postgres.WithPassword("diarion"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		cancel()
		t.Fatalf("start postgres: %v", err)
	}
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
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
	cleanup := func() { pool.Close(); _ = pgC.Terminate(ctx); cancel() }
	return dbgen.New(pool), cleanup
}

func TestRunOnce_PurgesExpiredEntries(t *testing.T) {
	t.Parallel()
	q, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{GoogleSub: "p1", Email: "p1@e.com", DisplayName: "P1"})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{OwnerID: u.ID, Handle: "p1", DisplayName: "P1", KeyCustody: "managed"})
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = 1
	}
	k, _ := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{AgentID: a.ID, PublicKey: pub, Fingerprint: "p1-fp"})
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	e, _ := q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: k.ID, Slug: "e1", Title: "T",
		BodyMarkdown: "x", BodyHTML: "<p>x</p>",
		Tags: []string{}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})

	reason := "test"
	if err := q.SoftDeleteEntry(ctx, dbgen.SoftDeleteEntryParams{ID: e.ID, RemovedReason: &reason}); err != nil {
		t.Fatalf("SoftDeleteEntry: %v", err)
	}
	if err := q.ForceExpireBinnedEntry(ctx, e.ID); err != nil {
		t.Fatalf("ForceExpireBinnedEntry: %v", err)
	}

	if err := purge.RunOnce(ctx, q, logger); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if _, err := q.GetEntryByID(ctx, e.ID); err == nil {
		t.Errorf("entry should be hard-deleted")
	}
}

func TestRunOnce_PurgesExpiredAgentAndItsEntries(t *testing.T) {
	t.Parallel()
	q, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{GoogleSub: "p2", Email: "p2@e.com", DisplayName: "P2"})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{OwnerID: u.ID, Handle: "p2", DisplayName: "P2", KeyCustody: "managed"})
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = 2
	}
	k, _ := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{AgentID: a.ID, PublicKey: pub, Fingerprint: "p2-fp"})
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	e, _ := q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: k.ID, Slug: "e1", Title: "T",
		BodyMarkdown: "x", BodyHTML: "<p>x</p>",
		Tags: []string{}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})

	reason := "t"
	_ = q.SoftDeleteAgent(ctx, dbgen.SoftDeleteAgentParams{ID: a.ID, RemovedReason: &reason})
	_ = q.ForceExpireBinnedAgent(ctx, a.ID)

	if err := purge.RunOnce(ctx, q, logger); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if _, err := q.GetAgentByIDAny(ctx, a.ID); err == nil {
		t.Errorf("agent should be hard-deleted")
	}
	if _, err := q.GetEntryByID(ctx, e.ID); err == nil {
		t.Errorf("entry should be cascade-deleted")
	}
}

func TestRunOnce_PurgesExpiredUserAndCascadesEverything(t *testing.T) {
	t.Parallel()
	q, cleanup := setupPool(t)
	defer cleanup()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{GoogleSub: "p3", Email: "p3@e.com", DisplayName: "P3"})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{OwnerID: u.ID, Handle: "p3", DisplayName: "P3", KeyCustody: "managed"})
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = 3
	}
	k, _ := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{AgentID: a.ID, PublicKey: pub, Fingerprint: "p3-fp"})
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	_, _ = q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: k.ID, Slug: "e1", Title: "T",
		BodyMarkdown: "x", BodyHTML: "<p>x</p>",
		Tags: []string{}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})

	_ = q.MarkUserDeleted(ctx, u.ID)
	_ = q.ForceExpireBinnedUser(ctx, u.ID)

	if err := purge.RunOnce(ctx, q, logger); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if _, err := q.GetUserByID(ctx, u.ID); err == nil {
		t.Errorf("user should be hard-deleted")
	}
}
