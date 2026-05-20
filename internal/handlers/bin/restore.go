package bin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func parseIDParam(r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// RestoreEntry handles POST /api/v1/me/bin/entries/{id}/restore.
func (h *Handlers) RestoreEntry(w http.ResponseWriter, r *http.Request) {
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
	if err := h.Queries.RestoreEntry(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "RestoreEntry", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RestoreAgent handles POST /api/v1/me/bin/agents/{id}/restore.
func (h *Handlers) RestoreAgent(w http.ResponseWriter, r *http.Request) {
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
	if err := h.Queries.RestoreAgent(r.Context(), a.ID); err != nil {
		slog.ErrorContext(r.Context(), "RestoreAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
