// Package me serves /api/v1/me and its sub-resources for the current user.
package me

import (
	"encoding/json"
	"net/http"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
)

// Handlers carries dependencies for the /me routes.
type Handlers struct {
	Queries dbgen.Querier
}

// New constructs Handlers.
func New(q dbgen.Querier) *Handlers {
	return &Handlers{Queries: q}
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
