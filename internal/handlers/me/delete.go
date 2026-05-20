package me

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
)

// Delete handles DELETE /api/v1/me — initiate account self-deletion.
//
// Soft-deletes the user, cascade-soft-deletes their agents so the bin shows
// them, destroys all sessions for this user (so the cookie they hold no
// longer authenticates), and returns 202 Accepted with the projected
// hard_delete_at timestamp.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	if err := h.softDeleteUserTx(ctx, u.ID); err != nil {
		slog.ErrorContext(ctx, "softDeleteUserTx", "err", err, "user_id", u.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// RevokeAllForUser already does both DB deletion AND clears the cookie
	// on the response writer. Best-effort; if it fails the soft-delete is
	// already persisted so we still return 202.
	if err := h.Sessions.RevokeAllForUser(ctx, w, u.ID); err != nil {
		slog.WarnContext(ctx, "RevokeAllForUser", "err", err, "user_id", u.ID)
	}

	hardDelete := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"deleted":        true,
		"hard_delete_at": hardDelete,
	})
}

// softDeleteUserTx atomically marks the user deleted and cascade-soft-deletes
// all their agents. Either both succeed or neither does — important because
// the user→agents FK is RESTRICT, so a partial state would block the eventual
// hourly purge.
func (h *Handlers) softDeleteUserTx(ctx context.Context, userID int64) error {
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	if err := q.MarkUserDeleted(ctx, userID); err != nil {
		return err
	}
	if err := q.SoftDeleteAgentsByUser(ctx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
