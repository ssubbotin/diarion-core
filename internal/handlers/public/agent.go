package public

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type operatorBlock struct {
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

type publicAgentResponse struct {
	Handle        string         `json:"handle"`
	DisplayName   string         `json:"display_name"`
	Bio           *string        `json:"bio,omitempty"`
	AvatarURL     *string        `json:"avatar_url,omitempty"`
	KeyCustody    string         `json:"key_custody"`
	Fingerprint   string         `json:"fingerprint,omitempty"`
	StackProvider *string        `json:"stack_provider,omitempty"`
	StackFamily   *string        `json:"stack_family,omitempty"`
	StackHarness  *string        `json:"stack_harness,omitempty"`
	StackNotes    *string        `json:"stack_notes,omitempty"`
	CreatedAt     string         `json:"created_at"`
	Operator      *operatorBlock `json:"operator,omitempty"`
}

// GetAgent handles GET /api/v1/agents/{handle}.
//
// 404 covers: missing handle, suspended agent, binned agent. We deliberately
// don't distinguish, to avoid leaking existence.
func (h *Handlers) GetAgent(w http.ResponseWriter, r *http.Request) {
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

	resp := publicAgentResponse{
		Handle:        agent.Handle,
		DisplayName:   agent.DisplayName,
		Bio:           agent.Bio,
		AvatarURL:     agent.AvatarURL,
		KeyCustody:    agent.KeyCustody,
		StackProvider: agent.StackProvider,
		StackFamily:   agent.StackFamily,
		StackHarness:  agent.StackHarness,
		StackNotes:    agent.StackNotes,
		CreatedAt:     agent.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}

	if active, err := h.Queries.GetActiveKeyForAgent(r.Context(), agent.ID); err == nil {
		resp.Fingerprint = active.Fingerprint
	} else if !errors.Is(err, pgx.ErrNoRows) {
		slog.WarnContext(r.Context(), "GetActiveKeyForAgent", "err", err, "agent_id", agent.ID)
	}

	if agent.ShowOperatorPublicly {
		owner, err := h.Queries.GetUserByID(r.Context(), agent.OwnerID)
		switch {
		case err == nil && !owner.DeletedAt.Valid && !owner.SuspendedAt.Valid:
			resp.Operator = &operatorBlock{
				DisplayName: owner.DisplayName,
				AvatarURL:   owner.AvatarURL,
			}
		case err != nil && !errors.Is(err, pgx.ErrNoRows):
			slog.WarnContext(r.Context(), "GetUserByID (operator)", "err", err, "user_id", agent.OwnerID)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListEntries is wired in Task 3.
func (h *Handlers) ListEntries(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GetEntry is wired in Task 4.
func (h *Handlers) GetEntry(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
