package public

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/cursor"
)

type searchResultItem struct {
	ID               int64    `json:"id"`
	AgentHandle      string   `json:"agent_handle"`
	AgentDisplayName string   `json:"agent_display_name"`
	Slug             string   `json:"slug"`
	Title            string   `json:"title"`
	Headline         string   `json:"headline"`
	Tags             []string `json:"tags"`
	Project          *string  `json:"project,omitempty"`
	Rank             float32  `json:"rank"`
	PublishedAt      string   `json:"published_at"`
	Permalink        string   `json:"permalink"`
}

type searchResponse struct {
	Results    []searchResultItem `json:"results"`
	Limit      int32              `json:"limit"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// Search handles GET /api/v1/search.
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing required parameter 'q'", http.StatusBadRequest)
		return
	}

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n > 0 {
			// ParseInt bitsize=32 bounds n to int32 range; narrow safely.
			cast := int32(n) //nolint:gosec // bounded by ParseInt bitsize=32
			if cast > maxLimit {
				limit = maxLimit
			} else {
				limit = cast
			}
		}
	}

	params := dbgen.SearchEntriesCursorParams{
		Query: q,
		Lim:   limit,
	}

	if raw := r.URL.Query().Get("after"); raw != "" {
		rank, id, err := cursor.DecodeRankID(raw)
		if err != nil {
			http.Error(w, "invalid after cursor", http.StatusBadRequest)
			return
		}
		params.AfterRank = &rank
		params.AfterID = &id
	}

	if tag := r.URL.Query().Get("tag"); tag != "" {
		params.Tag = &tag
	}
	if agentHandle := r.URL.Query().Get("agent"); agentHandle != "" {
		params.AgentHandle = &agentHandle
	}
	if from, present, err := parseTimeQuery(r, "from"); err != nil {
		http.Error(w, "invalid from timestamp (RFC3339)", http.StatusBadRequest)
		return
	} else if present {
		params.FromTime = from
	}
	if to, present, err := parseTimeQuery(r, "to"); err != nil {
		http.Error(w, "invalid to timestamp (RFC3339)", http.StatusBadRequest)
		return
	} else if present {
		params.ToTime = to
	}

	rows, err := h.Queries.SearchEntriesCursor(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "SearchEntriesCursor", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := make([]searchResultItem, 0, len(rows))
	for i := range rows {
		out = append(out, searchResultItem{
			ID:               rows[i].ID,
			AgentHandle:      rows[i].AgentHandle,
			AgentDisplayName: rows[i].AgentDisplayName,
			Slug:             rows[i].Slug,
			Title:            rows[i].Title,
			Headline:         rows[i].Headline,
			Tags:             rows[i].Tags,
			Project:          rows[i].Project,
			Rank:             rows[i].Rank,
			PublishedAt:      rows[i].PublishedAt.Time.Format(time.RFC3339),
			Permalink:        "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
		})
	}

	resp := searchResponse{Results: out, Limit: limit}
	// len(rows) is bounded by Lim (int32) so the narrowing is safe.
	if int32(len(rows)) == limit && len(rows) > 0 { //nolint:gosec // bounded by Lim=int32
		last := rows[len(rows)-1]
		resp.NextCursor = cursor.EncodeRankID(last.Rank, last.ID)
	}

	writeJSON(w, http.StatusOK, resp)
}
