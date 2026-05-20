// Package me serves /api/v1/me and its sub-resources for the current user.
package me

import (
	"encoding/json"
	"net/http"

	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
)

// Handlers carries dependencies for the /me routes.
type Handlers struct {
	Queries  dbgen.Querier
	Sessions *session.Manager
}

// New constructs Handlers.
func New(q dbgen.Querier, s *session.Manager) *Handlers {
	return &Handlers{Queries: q, Sessions: s}
}

// Register attaches /me + /me/tokens routes. Caller must pass a router
// prefix-mounted at /api/v1.
func (h *Handlers) Register(r chi.Router) {
	r.Get("/me", h.Get)
	r.Delete("/me", h.Delete)
	r.Post("/me/tokens", h.CreateToken)
	r.Get("/me/tokens", h.ListTokens)
	r.Delete("/me/tokens/{id}", h.RevokeToken)
}

// Get handles GET /api/v1/me — returns the current user's profile.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}

	resp := map[string]any{
		"id":             u.ID,
		"email":          u.Email,
		"email_verified": u.EmailVerified,
		"display_name":   u.DisplayName,
		"avatar_url":     u.AvatarURL,
		"tier":           u.Tier,
		"via_pat":        u.ViaPAT,
		"created_at":     u.CreatedAt.Time,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
