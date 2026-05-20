package bin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
)

const (
	defaultLimit int32 = 20
	maxLimit     int32 = 100
)

// parsePagination reads ?limit / ?offset with sensible bounds. Returns int32
// directly (matching sqlc-generated params) so the handler can pass values
// through without a gosec-tripping integer narrowing.
func parsePagination(r *http.Request) (limit, offset int32) {
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}

type binnedEntryItem struct {
	ID            int64    `json:"id"`
	AgentHandle   string   `json:"agent_handle"`
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Tags          []string `json:"tags"`
	Project       *string  `json:"project,omitempty"`
	PublishedAt   string   `json:"published_at"`
	RemovedAt     string   `json:"removed_at"`
	HardDeleteAt  string   `json:"hard_delete_at"`
	RemovedReason *string  `json:"removed_reason,omitempty"`
}

type binnedAgentItem struct {
	ID            int64   `json:"id"`
	Handle        string  `json:"handle"`
	DisplayName   string  `json:"display_name"`
	KeyCustody    string  `json:"key_custody"`
	RemovedAt     string  `json:"removed_at"`
	HardDeleteAt  string  `json:"hard_delete_at"`
	RemovedReason *string `json:"removed_reason,omitempty"`
}

// ListEntries handles GET /api/v1/me/bin/entries.
func (h *Handlers) ListEntries(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	limit, offset := parsePagination(r)
	rows, err := h.Queries.ListBinnedEntriesByOwner(r.Context(), dbgen.ListBinnedEntriesByOwnerParams{
		OwnerID: u.ID, Limit: limit, Offset: offset,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "ListBinnedEntriesByOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	total, err := h.Queries.CountBinnedEntriesByOwner(r.Context(), u.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "CountBinnedEntriesByOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]binnedEntryItem, 0, len(rows))
	for i := range rows {
		item := binnedEntryItem{
			ID:            rows[i].ID,
			AgentHandle:   rows[i].AgentHandle,
			Slug:          rows[i].Slug,
			Title:         rows[i].Title,
			Tags:          rows[i].Tags,
			Project:       rows[i].Project,
			PublishedAt:   rows[i].PublishedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			RemovedReason: rows[i].RemovedReason,
		}
		if rows[i].RemovedAt.Valid {
			item.RemovedAt = rows[i].RemovedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		if rows[i].HardDeleteAt.Valid {
			item.HardDeleteAt = rows[i].HardDeleteAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": out, "limit": limit, "offset": offset, "total": total,
	})
}

// ListAgents handles GET /api/v1/me/bin/agents.
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	limit, offset := parsePagination(r)
	rows, err := h.Queries.ListBinnedAgentsByOwner(r.Context(), dbgen.ListBinnedAgentsByOwnerParams{
		OwnerID: u.ID, Limit: limit, Offset: offset,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "ListBinnedAgentsByOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	total, err := h.Queries.CountBinnedAgentsByOwner(r.Context(), u.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "CountBinnedAgentsByOwner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]binnedAgentItem, 0, len(rows))
	for i := range rows {
		item := binnedAgentItem{
			ID:            rows[i].ID,
			Handle:        rows[i].Handle,
			DisplayName:   rows[i].DisplayName,
			KeyCustody:    rows[i].KeyCustody,
			RemovedReason: rows[i].RemovedReason,
		}
		if rows[i].RemovedAt.Valid {
			item.RemovedAt = rows[i].RemovedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		if rows[i].HardDeleteAt.Valid {
			item.HardDeleteAt = rows[i].HardDeleteAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": out, "limit": limit, "offset": offset, "total": total,
	})
}
