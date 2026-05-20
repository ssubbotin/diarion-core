package public

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const (
	defaultLimit   int32 = 20
	maxLimit       int32 = 100
	countSafetyCap int32 = 10_000
)

type entrySummary struct {
	ID          int64    `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Tags        []string `json:"tags"`
	Project     *string  `json:"project,omitempty"`
	PublishedAt string   `json:"published_at"`
	Permalink   string   `json:"permalink"`
}

type listEntriesResponse struct {
	AgentHandle string         `json:"agent_handle"`
	Entries     []entrySummary `json:"entries"`
	Limit       int32          `json:"limit"`
	Offset      int32          `json:"offset"`
	Total       int64          `json:"total"`
}

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

// ListEntries handles GET /api/v1/agents/{handle}/entries.
func (h *Handlers) ListEntries(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	rows, err := h.Queries.ListEntriesByAgent(r.Context(), dbgen.ListEntriesByAgentParams{
		AgentID: agent.ID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "ListEntriesByAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Count total (separate query — fine at v1 volume; revisit when search lands).
	total := int64(len(rows))
	if offset > 0 || int64(len(rows)) == int64(limit) {
		// Only run COUNT(*) when pagination is in play.
		c, err := h.countEntries(r, agent.ID)
		if err == nil {
			total = c
		}
	}

	out := make([]entrySummary, 0, len(rows))
	for i := range rows {
		out = append(out, entrySummary{
			ID:          rows[i].ID,
			Slug:        rows[i].Slug,
			Title:       rows[i].Title,
			Tags:        rows[i].Tags,
			Project:     rows[i].Project,
			PublishedAt: rows[i].PublishedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			Permalink:   "/" + agent.Handle + "/" + rows[i].Slug,
		})
	}

	writeJSON(w, http.StatusOK, listEntriesResponse{
		AgentHandle: agent.Handle,
		Entries:     out,
		Limit:       limit,
		Offset:      offset,
		Total:       total,
	})
}

// countEntries returns an approximate total. Reuses ListEntriesByAgent with
// a safety cap because no dedicated COUNT query exists yet (deferred to a
// later milestone alongside cursor-based pagination). When the result hits
// countSafetyCap, total is reported as a lower bound and a warning is logged.
func (h *Handlers) countEntries(r *http.Request, agentID int64) (int64, error) {
	rows, err := h.Queries.ListEntriesByAgent(r.Context(), dbgen.ListEntriesByAgentParams{
		AgentID: agentID,
		Limit:   countSafetyCap,
		Offset:  0,
	})
	if err != nil {
		return 0, err
	}
	n := int64(len(rows))
	if n == int64(countSafetyCap) {
		slog.WarnContext(r.Context(), "entry count saturated at safety cap; total is a lower bound",
			"agent_id", agentID, "cap", countSafetyCap)
	}
	return n, nil
}
