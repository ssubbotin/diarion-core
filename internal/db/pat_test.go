package db_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPAT_LifeCycle(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "pat-g-1", Email: "pat1@e.com", DisplayName: "P1",
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}

	hash := sha256.Sum256([]byte("plaintext-token-abc"))
	hashHex := hex.EncodeToString(hash[:])
	pat, err := q.InsertPAT(ctx, dbgen.InsertPATParams{
		UserID:    u.ID,
		TokenHash: hashHex,
		Name:      "claude-desktop",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("InsertPAT: %v", err)
	}

	got, err := q.GetPATByHash(ctx, hashHex)
	if err != nil {
		t.Fatalf("GetPATByHash: %v", err)
	}
	if got.ID != pat.ID {
		t.Errorf("PAT roundtrip mismatch")
	}

	// Touch
	if err := q.TouchPAT(ctx, pat.ID); err != nil {
		t.Fatalf("TouchPAT: %v", err)
	}
	touched, _ := q.GetPATByHash(ctx, hashHex)
	if !touched.LastUsedAt.Valid {
		t.Errorf("LastUsedAt should be set after Touch")
	}

	// Revoke
	if err := q.RevokePAT(ctx, dbgen.RevokePATParams{ID: pat.ID, UserID: u.ID}); err != nil {
		t.Fatalf("RevokePAT: %v", err)
	}
	if _, err := q.GetPATByHash(ctx, hashHex); err == nil {
		t.Errorf("revoked PAT should not be returned")
	}
}

func TestPAT_NoExpiryToken(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "pat-g-2", Email: "pat2@e.com", DisplayName: "P2",
	})

	hash := sha256.Sum256([]byte("forever-token"))
	hashHex := hex.EncodeToString(hash[:])
	_, err := q.InsertPAT(ctx, dbgen.InsertPATParams{
		UserID:    u.ID,
		TokenHash: hashHex,
		Name:      "forever",
		ExpiresAt: pgtype.Timestamptz{Valid: false}, // NULL = never expires
	})
	if err != nil {
		t.Fatalf("InsertPAT (no expiry): %v", err)
	}

	got, err := q.GetPATByHash(ctx, hashHex)
	if err != nil {
		t.Fatalf("GetPATByHash: %v", err)
	}
	if got.ExpiresAt.Valid {
		t.Errorf("ExpiresAt should be NULL for no-expiry token")
	}
}

func TestPAT_ExpiredTokenNotReturned(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "pat-g-3", Email: "pat3@e.com", DisplayName: "P3",
	})

	hash := sha256.Sum256([]byte("expired-token"))
	hashHex := hex.EncodeToString(hash[:])
	if _, err := q.InsertPAT(ctx, dbgen.InsertPATParams{
		UserID:    u.ID,
		TokenHash: hashHex,
		Name:      "expired",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("InsertPAT: %v", err)
	}

	if _, err := q.GetPATByHash(ctx, hashHex); err == nil {
		t.Errorf("expired PAT should not be returned")
	}
}

func TestPAT_ListForUser(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "pat-g-4", Email: "pat4@e.com", DisplayName: "P4",
	})

	for i := 0; i < 3; i++ {
		hash := sha256.Sum256([]byte("tok-" + string(rune('a'+i))))
		_, err := q.InsertPAT(ctx, dbgen.InsertPATParams{
			UserID:    u.ID,
			TokenHash: hex.EncodeToString(hash[:]),
			Name:      "tok-" + string(rune('a'+i)),
			ExpiresAt: pgtype.Timestamptz{Valid: false},
		})
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	list, err := q.ListPATsForUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListPATsForUser: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 active PATs, got %d", len(list))
	}

	// Revoke one; should drop to 2
	if err := q.RevokePAT(ctx, dbgen.RevokePATParams{ID: list[0].ID, UserID: u.ID}); err != nil {
		t.Fatalf("RevokePAT: %v", err)
	}

	list2, _ := q.ListPATsForUser(ctx, u.ID)
	if len(list2) != 2 {
		t.Errorf("expected 2 PATs after revoke, got %d", len(list2))
	}
}
