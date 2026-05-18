package db_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/diarion/diarion-core/internal/db/dbgen"
)

func newKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	sum := sha256.Sum256(pub)
	return pub, priv, hex.EncodeToString(sum[:])
}

func TestAgentKeys_InsertAndLookup(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "k-g-1", Email: "k1@e.com", DisplayName: "K1",
	})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: u.ID, Handle: "k-a-1", DisplayName: "KA1", KeyCustody: "managed",
	})

	pub, _, fp := newKeypair(t)
	k, err := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID:     a.ID,
		PublicKey:   []byte(pub),
		Fingerprint: fp,
	})
	if err != nil {
		t.Fatalf("InsertAgentKey: %v", err)
	}
	if k.Status != "active" {
		t.Errorf("expected default status 'active', got %q", k.Status)
	}

	byFP, err := q.GetAgentKeyByFingerprint(ctx, fp)
	if err != nil {
		t.Fatalf("GetAgentKeyByFingerprint: %v", err)
	}
	if byFP.ID != k.ID {
		t.Errorf("fingerprint lookup mismatch")
	}
}

func TestAgentKeys_Rotation(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{GoogleSub: "k-g-2", Email: "k2@e.com", DisplayName: "K2"})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{OwnerID: u.ID, Handle: "k-a-2", DisplayName: "KA2", KeyCustody: "managed"})

	pub1, _, fp1 := newKeypair(t)
	k1, err := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID: a.ID, PublicKey: []byte(pub1), Fingerprint: fp1,
	})
	if err != nil {
		t.Fatalf("InsertAgentKey #1: %v", err)
	}

	active, err := q.GetActiveKeyForAgent(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetActiveKeyForAgent: %v", err)
	}
	if active.ID != k1.ID {
		t.Errorf("initial active key mismatch")
	}

	// Rotate: insert new key, revoke old.
	pub2, _, fp2 := newKeypair(t)
	k2, err := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID: a.ID, PublicKey: []byte(pub2), Fingerprint: fp2,
	})
	if err != nil {
		t.Fatalf("InsertAgentKey #2: %v", err)
	}
	if err := q.RevokeAgentKey(ctx, k1.ID); err != nil {
		t.Fatalf("RevokeAgentKey: %v", err)
	}

	active2, err := q.GetActiveKeyForAgent(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetActiveKeyForAgent post-rotation: %v", err)
	}
	if active2.ID != k2.ID {
		t.Errorf("after rotation, active key should be k2 (id=%d), got id=%d", k2.ID, active2.ID)
	}

	all, err := q.ListKeysForAgent(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListKeysForAgent: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 keys total; got %d", len(all))
	}

	// Confirm the revoked key has status='revoked' and revoked_at set
	revoked, err := q.GetAgentKeyByFingerprint(ctx, fp1)
	if err != nil {
		t.Fatalf("GetAgentKeyByFingerprint(fp1): %v", err)
	}
	if revoked.Status != "revoked" {
		t.Errorf("expected status 'revoked', got %q", revoked.Status)
	}
	if !revoked.RevokedAt.Valid {
		t.Errorf("RevokedAt should be set after Revoke")
	}
}
