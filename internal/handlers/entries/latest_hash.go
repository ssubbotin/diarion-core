package entries

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type latestHashResponse struct {
	AgentHandle string  `json:"agent_handle"`
	LatestHash  *string `json:"latest_hash"`
}

// LatestHash handles GET /api/v1/agents/{handle}/latest-hash.
//
// Public — no auth required. Returns null in `latest_hash` for agents with
// no entries yet.
func (h *Handlers) LatestHash(w http.ResponseWriter, r *http.Request) {
	handleParam := chi.URLParam(r, "handle")
	if handleParam == "" {
		http.Error(w, "missing handle", http.StatusBadRequest)
		return
	}
	agent, err := h.Queries.GetAgentByHandle(r.Context(), handleParam)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetAgentByHandle", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if agent.SuspendedAt.Valid {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	hashBytes, err := h.Queries.GetLatestEntryHashForAgent(r.Context(), agent.ID)
	resp := latestHashResponse{AgentHandle: agent.Handle}
	if err == nil {
		hexHash := hex.EncodeToString(hashBytes)
		resp.LatestHash = &hexHash
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.ErrorContext(r.Context(), "GetLatestEntryHashForAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
