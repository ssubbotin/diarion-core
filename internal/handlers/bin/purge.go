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

// EmptyAll handles DELETE /api/v1/me/bin — hard-delete every item in the
// caller's bin in one transaction. Order matters:
//  1. Entries belonging to binned agents (cascade those agents' entries too).
//  2. Any other entries explicitly binned by the user.
//  3. The binned agents themselves (agent_keys cascade by FK).
func (h *Handlers) EmptyAll(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	if err := h.emptyAllTx(r.Context(), u.ID); err != nil {
		slog.ErrorContext(r.Context(), "emptyAllTx", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) emptyAllTx(ctx context.Context, ownerID int64) error {
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)

	// Find binned agents to cascade their entries (live or not).
	binnedAgents, err := q.ListBinnedAgentsByOwner(ctx, dbgen.ListBinnedAgentsByOwnerParams{
		OwnerID: ownerID, Limit: 10000, Offset: 0,
	})
	if err != nil {
		return err
	}
	for _, a := range binnedAgents {
		if _, err := q.HardDeleteEntriesForAgent(ctx, a.ID); err != nil {
			return err
		}
	}

	// Hard-delete any remaining binned entries owned by this user (entries
	// explicitly binned while their agent is still live).
	if _, err := q.HardDeleteEntriesByOwner(ctx, ownerID); err != nil {
		return err
	}

	// Hard-delete the binned agents.
	if _, err := q.HardDeleteAgentsByOwner(ctx, ownerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
