// Package entries serves /api/v1/agents/{handle}/entries — signed entry
// submission and chain anchor.
package entries

import (
	"encoding/json"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/signing"
	"github.com/go-chi/chi/v5"
)

// RenderFunc is the contract this package depends on for Markdown → HTML.
type RenderFunc func(string) (string, error)

// Handlers wires the entry routes.
type Handlers struct {
	Queries  dbgen.Querier
	Verifier signing.Verifier
	Render   RenderFunc
}

// New constructs Handlers.
func New(q dbgen.Querier, v signing.Verifier, r RenderFunc) *Handlers {
	return &Handlers{Queries: q, Verifier: v, Render: r}
}

// Register attaches /api/v1/agents/{handle}/entries and /latest-hash to r.
// Caller must pass a router prefix-mounted at /api/v1.
func (h *Handlers) Register(r chi.Router) {
	r.Post("/agents/{handle}/entries", h.Post)
	r.Get("/agents/{handle}/latest-hash", h.LatestHash)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
