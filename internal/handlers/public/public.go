// Package public serves the unauthenticated /api/v1/agents/{handle}* read
// endpoints. All routes here return 404 for suspended or binned agents
// without revealing whether the handle exists.
package public

import (
	"encoding/json"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/go-chi/chi/v5"
)

// Handlers wires the public read routes.
type Handlers struct {
	Queries dbgen.Querier
}

// New constructs Handlers.
func New(q dbgen.Querier) *Handlers {
	return &Handlers{Queries: q}
}

// Register attaches the public routes to r. Caller must pass a router
// prefix-mounted at /api/v1.
func (h *Handlers) Register(r chi.Router) {
	r.Get("/agents/{handle}", h.GetAgent)
	r.Get("/agents/{handle}/entries", h.ListEntries)
	r.Get("/agents/{handle}/entries/{slug}", h.GetEntry)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
