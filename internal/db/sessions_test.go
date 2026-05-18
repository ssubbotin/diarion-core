//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSessions_LifeCycle(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "sess-g-1",
		Email:       "sess1@example.com",
		DisplayName: "Sess1",
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	s, err := q.InsertSession(ctx, dbgen.InsertSessionParams{
		SessionToken: "tok-1",
		UserID:       u.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	got, err := q.GetSessionByToken(ctx, "tok-1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("session id mismatch")
	}

	if err := q.TouchSession(ctx, s.ID); err != nil {
		t.Fatalf("touch session: %v", err)
	}

	if err := q.DeleteSession(ctx, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := q.GetSessionByToken(ctx, "tok-1"); err == nil {
		t.Errorf("expected ErrNoRows after delete")
	}
}

func TestSessions_DeleteExpired(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "sess-g-2",
		Email:       "sess2@example.com",
		DisplayName: "Sess2",
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// One expired, one live
	if _, err := q.InsertSession(ctx, dbgen.InsertSessionParams{
		SessionToken: "expired",
		UserID:       u.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("insert expired: %v", err)
	}
	if _, err := q.InsertSession(ctx, dbgen.InsertSessionParams{
		SessionToken: "live",
		UserID:       u.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("insert live: %v", err)
	}

	deleted, err := q.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify the live session survives
	if _, err := q.GetSessionByToken(ctx, "live"); err != nil {
		t.Errorf("live session should still be fetchable: %v", err)
	}
}

func TestSessions_DeleteAllForUser(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "sess-g-3",
		Email:       "sess3@example.com",
		DisplayName: "Sess3",
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := q.InsertSession(ctx, dbgen.InsertSessionParams{
			SessionToken: "tok-" + string(rune('a'+i)),
			UserID:       u.ID,
			ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
		}); err != nil {
			t.Fatalf("insert session %d: %v", i, err)
		}
	}

	if err := q.DeleteAllSessionsForUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteAllSessionsForUser: %v", err)
	}

	if _, err := q.GetSessionByToken(ctx, "tok-a"); err == nil {
		t.Errorf("session should be gone after DeleteAllSessionsForUser")
	}
}
