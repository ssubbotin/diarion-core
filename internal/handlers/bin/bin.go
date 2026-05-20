// Package bin serves the /api/v1/me/bin family — list / restore / immediate
// purge for the authenticated user's soft-deleted entries and agents.
package bin

import (
	"encoding/json"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/go-chi/chi/v5"
)

// Handlers wires the bin routes.
type Handlers struct {
	Queries dbgen.Querier
}

// New constructs Handlers.
func New(q dbgen.Querier) *Handlers {
	return &Handlers{Queries: q}
}

// Register attaches /me/bin/* routes. Caller must pass a router
// prefix-mounted at /api/v1.
func (h *Handlers) Register(r chi.Router) {
	r.Get("/me/bin", h.Summary)
	r.Get("/me/bin/entries", h.ListEntries)
	r.Get("/me/bin/agents", h.ListAgents)
	r.Post("/me/bin/entries/{id}/restore", h.RestoreEntry)
	r.Post("/me/bin/agents/{id}/restore", h.RestoreAgent)
	r.Delete("/me/bin/entries/{id}", h.PurgeEntry)
	r.Delete("/me/bin/agents/{id}", h.PurgeAgent)
	r.Delete("/me/bin", h.EmptyAll)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Stubs filled in subsequent tasks.

// RestoreEntry un-deletes one of the user's soft-deleted entries.
// Implemented in Task 8.
func (h *Handlers) RestoreEntry(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// RestoreAgent un-deletes one of the user's soft-deleted agents.
// Implemented in Task 8.
func (h *Handlers) RestoreAgent(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// PurgeEntry hard-deletes one of the user's soft-deleted entries.
// Implemented in Task 8.
func (h *Handlers) PurgeEntry(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// PurgeAgent hard-deletes one of the user's soft-deleted agents.
// Implemented in Task 8.
func (h *Handlers) PurgeAgent(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// EmptyAll hard-deletes every item in the authenticated user's bin.
// Implemented in Task 9.
func (h *Handlers) EmptyAll(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
