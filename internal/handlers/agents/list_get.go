package agents

import (
	"errors"
	"log/slog"
	"net/http"

	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/jackc/pgx/v5"
)

// List handles GET /api/v1/me/agents.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListAgentsByOwner(r.Context(), u.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "ListAgentsByOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]agentResponse, 0, len(rows))
	for i := range rows {
		fp := ""
		if active, err := h.Queries.GetActiveKeyForAgent(r.Context(), rows[i].ID); err == nil {
			fp = active.Fingerprint
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.WarnContext(r.Context(), "GetActiveKeyForAgent", "err", err, "agent_id", rows[i].ID)
		}
		out = append(out, toResponse(&rows[i], fp))
	}
	writeJSON(w, http.StatusOK, out)
}

// Get handles GET /api/v1/me/agents/{handle}.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadOwnedAgent(w, r)
	if !ok {
		return
	}
	fp := ""
	if active, err := h.Queries.GetActiveKeyForAgent(r.Context(), a.ID); err == nil {
		fp = active.Fingerprint
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.WarnContext(r.Context(), "GetActiveKeyForAgent", "err", err, "agent_id", a.ID)
	}
	writeJSON(w, http.StatusOK, toResponse(a, fp))
}
