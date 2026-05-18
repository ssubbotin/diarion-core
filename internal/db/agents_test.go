//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/diarion/diarion-core/internal/db/dbgen"
)

func TestAgents_CreateGetListUpdate(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "agent-g-1",
		Email:       "ag@example.com",
		DisplayName: "Owner",
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}

	a, err := q.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID:              u.ID,
		Handle:               "diarion-build",
		DisplayName:          "Diarion Build",
		KeyCustody:           "managed",
		ShowOperatorPublicly: false,
	})
	if err != nil {
		t.Fatalf("InsertAgent: %v", err)
	}

	// Case-insensitive handle lookup
	byHandle, err := q.GetAgentByHandle(ctx, "Diarion-Build")
	if err != nil {
		t.Fatalf("GetAgentByHandle: %v", err)
	}
	if byHandle.ID != a.ID {
		t.Errorf("case-insensitive lookup failed")
	}

	// ID lookup
	byID, err := q.GetAgentByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAgentByID: %v", err)
	}
	if byID.Handle != "diarion-build" {
		t.Errorf("handle mismatch: %q", byID.Handle)
	}

	// List by owner
	list, err := q.ListAgentsByOwner(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListAgentsByOwner: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 agent, got %d", len(list))
	}

	// Update profile
	bio := "We build Diarion."
	updated, err := q.UpdateAgentProfile(ctx, dbgen.UpdateAgentProfileParams{
		ID:                   a.ID,
		DisplayName:          "Diarion Build",
		Bio:                  &bio,
		ShowOperatorPublicly: true,
	})
	if err != nil {
		t.Fatalf("UpdateAgentProfile: %v", err)
	}
	if !updated.ShowOperatorPublicly {
		t.Errorf("toggle not applied")
	}
	if updated.Bio == nil || *updated.Bio != bio {
		t.Errorf("bio not set: %v", updated.Bio)
	}
}

func TestAgents_SoftDeleteHidesFromHandleLookup(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "agent-g-2",
		Email:       "ag2@example.com",
		DisplayName: "Owner2",
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}
	a, err := q.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID:     u.ID,
		Handle:      "to-bin",
		DisplayName: "Bin Me",
		KeyCustody:  "managed",
	})
	if err != nil {
		t.Fatalf("InsertAgent: %v", err)
	}

	reason := "self-delete"
	if err := q.SoftDeleteAgent(ctx, dbgen.SoftDeleteAgentParams{
		ID:            a.ID,
		RemovedReason: &reason,
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	// Handle lookup should fail (removed_at IS NULL filter excludes it)
	if _, err := q.GetAgentByHandle(ctx, "to-bin"); err == nil {
		t.Errorf("expected GetAgentByHandle to skip binned agents")
	}

	// Direct ID lookup should still return the agent (with removed_at set)
	byID, err := q.GetAgentByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAgentByID after soft delete: %v", err)
	}
	if !byID.RemovedAt.Valid {
		t.Errorf("RemovedAt should be set")
	}

	// Restore
	if err := q.RestoreAgent(ctx, a.ID); err != nil {
		t.Fatalf("RestoreAgent: %v", err)
	}
	if _, err := q.GetAgentByHandle(ctx, "to-bin"); err != nil {
		t.Errorf("expected GetAgentByHandle to find restored agent: %v", err)
	}
}
