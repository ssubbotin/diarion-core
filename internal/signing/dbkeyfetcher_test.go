//go:build integration

package signing_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/signing"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupQueries(t *testing.T) (*dbgen.Queries, func()) {
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

func mintAgentWithKey(t *testing.T, q *dbgen.Queries, ctx context.Context, handle string) (int64, int64, ed25519.PublicKey, string) {
	t.Helper()
	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "kf-" + handle, Email: handle + "@e.com", DisplayName: handle,
	})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: u.ID, Handle: handle, DisplayName: handle, KeyCustody: "managed",
	})
	pub, _, _ := ed25519.GenerateKey(nil)
	sum := sha256.Sum256(pub)
	fp := hex.EncodeToString(sum[:])
	k, _ := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID: a.ID, PublicKey: []byte(pub), Fingerprint: fp,
	})
	return a.ID, k.ID, pub, fp
}

func TestDBKeyFetcher_Active(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()
	ctx := context.Background()
	agentID, keyID, pub, fp := mintAgentWithKey(t, q, ctx, "kf1")

	rec, err := signing.NewDBKeyFetcher(q).Fetch(ctx, fp)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if rec.AgentID != agentID || rec.KeyID != keyID {
		t.Errorf("rec mismatch: %+v", rec)
	}
	if !rec.PublicKey.Equal(pub) {
		t.Errorf("PublicKey mismatch")
	}
}

func TestDBKeyFetcher_Revoked(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()
	ctx := context.Background()
	_, keyID, _, fp := mintAgentWithKey(t, q, ctx, "kf2")
	if err := q.RevokeAgentKey(ctx, keyID); err != nil {
		t.Fatalf("RevokeAgentKey: %v", err)
	}

	_, err := signing.NewDBKeyFetcher(q).Fetch(ctx, fp)
	if !errors.Is(err, signing.ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked; got %v", err)
	}
}

func TestDBKeyFetcher_NotFound(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()
	_, err := signing.NewDBKeyFetcher(q).Fetch(context.Background(), "deadbeef")
	if !errors.Is(err, signing.ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound; got %v", err)
	}
}
