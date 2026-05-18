package db_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestModerationActions_Insert(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "mod-g-1", Email: "mod1@e.com", DisplayName: "M1",
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}

	notes := "auto-suspended after 1000 entries/hour"
	appealUntil := pgtype.Timestamptz{Time: time.Now().Add(30 * 24 * time.Hour), Valid: true}

	action, err := q.InsertModerationAction(ctx, dbgen.InsertModerationActionParams{
		TargetType:        "user",
		TargetID:          u.ID,
		Action:            "suspend",
		Category:          "spam",
		Moderator:         "system:rate-limiter",
		Notes:             &notes,
		AppealWindowUntil: appealUntil,
	})
	if err != nil {
		t.Fatalf("InsertModerationAction: %v", err)
	}
	if action.ID == 0 {
		t.Errorf("id not assigned")
	}
	if action.Notes == nil || *action.Notes != notes {
		t.Errorf("notes roundtrip failed: %v", action.Notes)
	}

	list, err := q.ListModerationActionsForTarget(ctx, dbgen.ListModerationActionsForTargetParams{
		TargetType: "user",
		TargetID:   u.ID,
	})
	if err != nil {
		t.Fatalf("ListModerationActionsForTarget: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 action, got %d", len(list))
	}
}

func TestRejectedSubmissions_PurgeWindow(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "rej-g-1", Email: "rej1@e.com", DisplayName: "R1",
	})
	a, _ := q.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: u.ID, Handle: "rej-a-1", DisplayName: "RA1", KeyCustody: "managed",
	})

	body := "abusive content"
	title := "spam"
	ch := sha256.Sum256([]byte(body))

	// Insert two rejected submissions:
	// 1. expired (appeal_window_until in the past) — should be purged
	// 2. live (appeal_window_until in the future) — should NOT be purged
	expired := pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true}
	live := pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true}

	_, err := q.InsertRejectedSubmission(ctx, dbgen.InsertRejectedSubmissionParams{
		AgentID:           a.ID,
		BodyMarkdown:      &body,
		Title:             &title,
		Tags:              []string{"spam"},
		ContentHash:       ch[:],
		RejectionCategory: "spam",
		AppealWindowUntil: expired,
	})
	if err != nil {
		t.Fatalf("InsertRejectedSubmission (expired): %v", err)
	}

	_, err = q.InsertRejectedSubmission(ctx, dbgen.InsertRejectedSubmissionParams{
		AgentID:           a.ID,
		BodyMarkdown:      &body,
		Title:             &title,
		Tags:              []string{"spam"},
		ContentHash:       ch[:],
		RejectionCategory: "spam",
		AppealWindowUntil: live,
	})
	if err != nil {
		t.Fatalf("InsertRejectedSubmission (live): %v", err)
	}

	n, err := q.PurgeExpiredRejectedBodies(ctx)
	if err != nil {
		t.Fatalf("PurgeExpiredRejectedBodies: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 purged; got %d", n)
	}

	// Re-running should be a no-op
	n2, _ := q.PurgeExpiredRejectedBodies(ctx)
	if n2 != 0 {
		t.Errorf("re-running purge should be no-op; got %d", n2)
	}
}

func TestModerationActions_MultipleActionsForTarget(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "mod-g-2", Email: "mod2@e.com", DisplayName: "M2",
	})

	for _, action := range []string{"suspend", "restore", "suspend"} {
		_, err := q.InsertModerationAction(ctx, dbgen.InsertModerationActionParams{
			TargetType: "user",
			TargetID:   u.ID,
			Action:     action,
			Category:   "spam",
			Moderator:  "system:test",
		})
		if err != nil {
			t.Fatalf("InsertModerationAction(%s): %v", action, err)
		}
	}

	list, err := q.ListModerationActionsForTarget(ctx, dbgen.ListModerationActionsForTargetParams{
		TargetType: "user",
		TargetID:   u.ID,
	})
	if err != nil {
		t.Fatalf("ListModerationActionsForTarget: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 actions, got %d", len(list))
	}
	// Confirm ordering: most recent first
	for i := 1; i < len(list); i++ {
		if list[i].CreatedAt.Time.After(list[i-1].CreatedAt.Time) {
			t.Errorf("actions not ordered DESC by created_at at index %d", i)
		}
	}
}
