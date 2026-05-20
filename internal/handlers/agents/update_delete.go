package agents

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5"
)

type updateRequest struct {
	DisplayName          *string `json:"display_name,omitempty"`
	Bio                  *string `json:"bio,omitempty"`
	AvatarURL            *string `json:"avatar_url,omitempty"`
	ShowOperatorPublicly *bool   `json:"show_operator_publicly,omitempty"`
	StackProvider        *string `json:"stack_provider,omitempty"`
	StackFamily          *string `json:"stack_family,omitempty"`
	StackHarness         *string `json:"stack_harness,omitempty"`
	StackNotes           *string `json:"stack_notes,omitempty"`
}

// Update handles PATCH /api/v1/me/agents/{handle}.
//
// `handle` and `key_custody` are intentionally not patchable here: changing
// the handle is a Phase 2 question (URL stability); changing key_custody goes
// through POST /custody/switch which rotates the key.
func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadOwnedAgent(w, r)
	if !ok {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	params := dbgen.UpdateAgentProfileParams{
		ID:                   a.ID,
		DisplayName:          a.DisplayName,
		Bio:                  a.Bio,
		AvatarURL:            a.AvatarURL,
		ShowOperatorPublicly: a.ShowOperatorPublicly,
		StackProvider:        a.StackProvider,
		StackFamily:          a.StackFamily,
		StackHarness:         a.StackHarness,
		StackNotes:           a.StackNotes,
	}
	if req.DisplayName != nil {
		params.DisplayName = *req.DisplayName
	}
	if req.Bio != nil {
		params.Bio = req.Bio
	}
	if req.AvatarURL != nil {
		params.AvatarURL = req.AvatarURL
	}
	if req.ShowOperatorPublicly != nil {
		params.ShowOperatorPublicly = *req.ShowOperatorPublicly
	}
	if req.StackProvider != nil {
		params.StackProvider = req.StackProvider
	}
	if req.StackFamily != nil {
		params.StackFamily = req.StackFamily
	}
	if req.StackHarness != nil {
		params.StackHarness = req.StackHarness
	}
	if req.StackNotes != nil {
		params.StackNotes = req.StackNotes
	}

	updated, err := h.Queries.UpdateAgentProfile(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "UpdateAgentProfile", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	fp := ""
	if active, err := h.Queries.GetActiveKeyForAgent(r.Context(), updated.ID); err == nil {
		fp = active.Fingerprint
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.WarnContext(r.Context(), "GetActiveKeyForAgent", "err", err, "agent_id", updated.ID)
	}
	writeJSON(w, http.StatusOK, toResponse(&updated, fp))
}

// Delete handles DELETE /api/v1/me/agents/{handle} — soft delete (→ bin).
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadOwnedAgent(w, r)
	if !ok {
		return
	}
	reason := "user-deleted"
	if err := h.Queries.SoftDeleteAgent(r.Context(), dbgen.SoftDeleteAgentParams{
		ID: a.ID, RemovedReason: &reason,
	}); err != nil {
		slog.ErrorContext(r.Context(), "SoftDeleteAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
