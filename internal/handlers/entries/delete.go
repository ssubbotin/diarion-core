package entries

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Delete handles DELETE /api/v1/agents/{handle}/entries/{slug}.
//
// Two valid auth paths, checked in this order:
//  1. Signature header present → verify RFC 9421 signature (same Verifier as POST).
//     The signed key's agent_id must match the URL handle.
//  2. Otherwise → require a session, and require the session user to own the agent.
//
// The soft-delete sets removed_at / hard_delete_at (+30 days). Restoration
// goes through /api/v1/me/bin/entries/{id}/restore.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	handleParam := chi.URLParam(r, "handle")
	slugParam := chi.URLParam(r, "slug")
	if handleParam == "" || slugParam == "" {
		http.Error(w, "missing handle or slug", http.StatusBadRequest)
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
		http.Error(w, "agent suspended", http.StatusForbidden)
		return
	}

	// Pick auth path.
	if r.Header.Get("Signature") != "" || r.Header.Get("Signature-Input") != "" {
		if !h.authorizeBySignature(w, r, &agent) {
			return
		}
	} else {
		if !h.authorizeBySession(w, r, &agent) {
			return
		}
	}

	// Look up the entry.
	row, err := h.Queries.GetEntryByAgentAndSlug(r.Context(), dbgen.GetEntryByAgentAndSlugParams{
		AgentID: agent.ID,
		Slug:    slugParam,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetEntryByAgentAndSlug (delete)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	reason := "user-deleted"
	if err := h.Queries.SoftDeleteEntry(r.Context(), dbgen.SoftDeleteEntryParams{
		ID: row.ID, RemovedReason: &reason,
	}); err != nil {
		slog.ErrorContext(r.Context(), "SoftDeleteEntry", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) authorizeBySignature(w http.ResponseWriter, r *http.Request, agent *dbgen.Agent) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBodyBytes+1))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return false
	}
	if len(body) > MaxBodyBytes {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return false
	}
	if len(body) == 0 {
		body = []byte{}
	}
	res, err := h.Verifier.Verify(r, body)
	if err != nil {
		writeVerifyError(w, r, err)
		return false
	}
	if res.AgentID != agent.ID {
		http.Error(w, "key does not belong to this agent", http.StatusForbidden)
		return false
	}
	// Bind the delete to the current chain anchor so a stale signature can't
	// re-delete a restored entry days later.
	latest, err := h.Queries.GetLatestEntryHashForAgent(r.Context(), agent.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.ErrorContext(r.Context(), "GetLatestEntryHashForAgent (delete)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Agent has no live entries (e.g., last one being deleted). prev must be zeros.
		if !bytes.Equal(res.PrevEntryHash, zeros32) {
			http.Error(w, "prev_entry_hash must be zero when chain is empty", http.StatusConflict)
			return false
		}
	} else if !bytes.Equal(res.PrevEntryHash, latest) {
		http.Error(w, "prev_entry_hash does not match chain head", http.StatusConflict)
		return false
	}
	return true
}

func (h *Handlers) authorizeBySession(w http.ResponseWriter, r *http.Request, agent *dbgen.Agent) bool {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return false
	}
	if u.ID != agent.OwnerID {
		// Don't reveal that the entry exists — 404 mirrors public read.
		http.Error(w, "entry not found", http.StatusNotFound)
		return false
	}
	return true
}
