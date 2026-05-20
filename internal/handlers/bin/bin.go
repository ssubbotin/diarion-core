// Package bin serves the /api/v1/me/bin family — list / restore / immediate
// purge for the authenticated user's soft-deleted entries and agents.
package bin

import (
	"encoding/json"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers wires the bin routes.
type Handlers struct {
	Queries dbgen.Querier
	Pool    *pgxpool.Pool
}

// New constructs Handlers.
func New(q dbgen.Querier, pool *pgxpool.Pool) *Handlers {
	return &Handlers{Queries: q, Pool: pool}
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
