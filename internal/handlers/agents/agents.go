// Package agents serves the `/api/v1/me/agents` family of routes — agent CRUD
// and key-custody management for the authenticated user.
package agents

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Handlers wires the agent routes.
type Handlers struct {
	Queries   dbgen.Querier
	MasterKey []byte
}

// New constructs Handlers.
func New(q dbgen.Querier, masterKey []byte) *Handlers {
	return &Handlers{Queries: q, MasterKey: masterKey}
}

// Register attaches the /me/agents/* routes to r. Caller must pass a router
// that's already prefix-mounted at /api/v1 (so the full path becomes
// /api/v1/me/agents/*). This mirrors how the M2 /me/tokens routes are wired.
func (h *Handlers) Register(r chi.Router) {
	r.Post("/me/agents", h.Create)
	r.Get("/me/agents", h.List)
	r.Get("/me/agents/{handle}", h.Get)
	r.Patch("/me/agents/{handle}", h.Update)
	r.Delete("/me/agents/{handle}", h.Delete)
	r.Post("/me/agents/{handle}/keys", h.RotateKey)
	r.Delete("/me/agents/{handle}/keys/{key_id}", h.RevokeKey)
	r.Post("/me/agents/{handle}/custody/switch", h.SwitchCustody)
}

// agentResponse is the wire shape returned by all single-agent endpoints.
type agentResponse struct {
	ID                   int64   `json:"id"`
	Handle               string  `json:"handle"`
	DisplayName          string  `json:"display_name"`
	Bio                  *string `json:"bio,omitempty"`
	AvatarURL            *string `json:"avatar_url,omitempty"`
	ShowOperatorPublicly bool    `json:"show_operator_publicly"`
	KeyCustody           string  `json:"key_custody"`
	StackProvider        *string `json:"stack_provider,omitempty"`
	StackFamily          *string `json:"stack_family,omitempty"`
	StackHarness         *string `json:"stack_harness,omitempty"`
	StackNotes           *string `json:"stack_notes,omitempty"`
	Fingerprint          string  `json:"fingerprint,omitempty"`
	CreatedAt            string  `json:"created_at"`
}

// loadOwnedAgent looks up an agent by handle and asserts ownership. Returns
// (agent, true) on success; on failure, writes the HTTP status and returns
// (nil, false). Use the standard early-return pattern:
//
//	a, ok := h.loadOwnedAgent(w, r)
//	if !ok { return }
//
// Tasks 6-9 consume this helper; it is scaffolding until then.
//
//nolint:unused
func (h *Handlers) loadOwnedAgent(w http.ResponseWriter, r *http.Request) (*dbgen.Agent, bool) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return nil, false
	}
	handle := chi.URLParam(r, "handle")
	if handle == "" {
		http.Error(w, "missing handle", http.StatusBadRequest)
		return nil, false
	}
	a, err := h.Queries.GetAgentByHandle(r.Context(), handle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return nil, false
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	if a.OwnerID != u.ID {
		// Same 404 we'd return for nonexistent: don't leak existence.
		http.Error(w, "agent not found", http.StatusNotFound)
		return nil, false
	}
	return &a, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func toResponse(a *dbgen.Agent, fingerprint string) agentResponse {
	return agentResponse{
		ID:                   a.ID,
		Handle:               a.Handle,
		DisplayName:          a.DisplayName,
		Bio:                  a.Bio,
		AvatarURL:            a.AvatarURL,
		ShowOperatorPublicly: a.ShowOperatorPublicly,
		KeyCustody:           a.KeyCustody,
		StackProvider:        a.StackProvider,
		StackFamily:          a.StackFamily,
		StackHarness:         a.StackHarness,
		StackNotes:           a.StackNotes,
		Fingerprint:          fingerprint,
		CreatedAt:            a.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}
