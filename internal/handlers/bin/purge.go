package bin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/jackc/pgx/v5"
)

// PurgeEntry handles DELETE /api/v1/me/bin/entries/{id} — hard-delete one
// entry. Only allowed if the caller owns the entry AND the entry is binned.
func (h *Handlers) PurgeEntry(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := parseIDParam(r, "id")
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	row, err := h.Queries.GetEntryByIDForOwner(r.Context(), dbgen.GetEntryByIDForOwnerParams{
		ID: id, OwnerID: u.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetEntryByIDForOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !row.RemovedAt.Valid {
		http.Error(w, "entry is not in the bin", http.StatusConflict)
		return
	}
	if _, err := h.Queries.HardDeleteEntry(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "HardDeleteEntry", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PurgeAgent handles DELETE /api/v1/me/bin/agents/{id} — hard-delete one
// agent + cascade its entries inside a transaction.
func (h *Handlers) PurgeAgent(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := parseIDParam(r, "id")
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	a, err := h.Queries.GetBinnedAgentForOwner(r.Context(), dbgen.GetBinnedAgentForOwnerParams{
		ID: id, OwnerID: u.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetBinnedAgentForOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.purgeAgentTx(r.Context(), a.ID); err != nil {
		slog.ErrorContext(r.Context(), "purgeAgentTx", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// purgeAgentTx deletes all entries for an agent, then the agent itself.
// agent_keys cascades automatically (CASCADE on agent_id FK).
func (h *Handlers) purgeAgentTx(ctx context.Context, agentID int64) error {
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	if _, err := q.HardDeleteEntriesForAgent(ctx, agentID); err != nil {
		return err
	}
	if _, err := q.HardDeleteAgent(ctx, agentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
