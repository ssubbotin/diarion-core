package me

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/diarion/diarion-core/internal/auth/pat"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type createTokenRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type createTokenResponse struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Plaintext string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type listTokenItem struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateToken handles POST /api/v1/me/tokens.
func (h *Handlers) CreateToken(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	if u.ViaPAT {
		http.Error(w, "tokens may only be created from a browser session", http.StatusForbidden)
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	plain, hash, err := pat.Generate()
	if err != nil {
		slog.ErrorContext(r.Context(), "pat generate", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	expires := pgtype.Timestamptz{Valid: false}
	if req.ExpiresAt != nil {
		expires = pgtype.Timestamptz{Time: *req.ExpiresAt, Valid: true}
	}

	row, err := h.Queries.InsertPAT(r.Context(), dbgen.InsertPATParams{
		UserID:    u.ID,
		TokenHash: hash,
		Name:      req.Name,
		ExpiresAt: expires,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "InsertPAT", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := createTokenResponse{
		ID:        row.ID,
		Name:      row.Name,
		Plaintext: plain,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		resp.ExpiresAt = &t
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// ListTokens handles GET /api/v1/me/tokens.
func (h *Handlers) ListTokens(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListPATsForUser(r.Context(), u.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "ListPATsForUser", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]listTokenItem, 0, len(rows))
	for _, row := range rows {
		it := listTokenItem{
			ID:        row.ID,
			Name:      row.Name,
			CreatedAt: row.CreatedAt.Time,
		}
		if row.LastUsedAt.Valid {
			t := row.LastUsedAt.Time
			it.LastUsedAt = &t
		}
		if row.ExpiresAt.Valid {
			t := row.ExpiresAt.Time
			it.ExpiresAt = &t
		}
		items = append(items, it)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

// RevokeToken handles DELETE /api/v1/me/tokens/{id}.
func (h *Handlers) RevokeToken(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid token id", http.StatusBadRequest)
		return
	}

	// RevokePAT updates 0 rows if (id, user_id) doesn't match; sqlc's :exec
	// signature returns nil even on 0 rows affected. That's the desired
	// idempotent-revoke behaviour (no leak about whether the token existed).
	if err := h.Queries.RevokePAT(r.Context(), dbgen.RevokePATParams{
		ID:     id,
		UserID: u.ID,
	}); err != nil {
		slog.ErrorContext(r.Context(), "RevokePAT", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
