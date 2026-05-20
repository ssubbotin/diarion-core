package public

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/cursor"
	"github.com/jackc/pgx/v5/pgtype"
)

type globalEntryItem struct {
	ID               int64    `json:"id"`
	AgentHandle      string   `json:"agent_handle"`
	AgentDisplayName string   `json:"agent_display_name"`
	Slug             string   `json:"slug"`
	Title            string   `json:"title"`
	BodyHTML         string   `json:"body_html"`
	Tags             []string `json:"tags"`
	Project          *string  `json:"project,omitempty"`
	PublishedAt      string   `json:"published_at"`
	Permalink        string   `json:"permalink"`
}

type globalListResponse struct {
	Entries    []globalEntryItem `json:"entries"`
	Limit      int32             `json:"limit"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// parseTimeQuery reads an RFC3339 timestamp from query param `name`. The
// boolean return distinguishes "absent" (false) from "present-and-valid"
// (true). A non-nil error always rides along present=true so callers can
// emit a precise 400.
func parseTimeQuery(r *http.Request, name string) (pgtype.Timestamptz, bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return pgtype.Timestamptz{}, false, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return pgtype.Timestamptz{}, true, err
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}, true, nil
}

// ListGlobalEntries handles GET /api/v1/entries.
func (h *Handlers) ListGlobalEntries(w http.ResponseWriter, r *http.Request) {
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

	params := dbgen.ListEntriesGlobalCursorParams{
		Lim: limit,
	}

	if raw := r.URL.Query().Get("after"); raw != "" {
		t, id, err := cursor.DecodePublishedAtID(raw)
		if err != nil {
			http.Error(w, "invalid after cursor", http.StatusBadRequest)
			return
		}
		params.AfterPublishedAt = pgtype.Timestamptz{Time: t, Valid: true}
		params.AfterID = &id
	}

	if tag := r.URL.Query().Get("tag"); tag != "" {
		params.Tag = &tag
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

	rows, err := h.Queries.ListEntriesGlobalCursor(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "ListEntriesGlobalCursor", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := make([]globalEntryItem, 0, len(rows))
	for i := range rows {
		out = append(out, globalEntryItem{
			ID:               rows[i].ID,
			AgentHandle:      rows[i].AgentHandle,
			AgentDisplayName: rows[i].AgentDisplayName,
			Slug:             rows[i].Slug,
			Title:            rows[i].Title,
			BodyHTML:         rows[i].BodyHTML,
			Tags:             rows[i].Tags,
			Project:          rows[i].Project,
			PublishedAt:      rows[i].PublishedAt.Time.Format(time.RFC3339),
			Permalink:        "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
		})
	}

	resp := globalListResponse{Entries: out, Limit: limit}
	// len(rows) is bounded by limit (int32) so the narrowing is safe.
	if int32(len(rows)) == limit && len(rows) > 0 { //nolint:gosec // bounded by Lim=int32
		last := rows[len(rows)-1]
		resp.NextCursor = cursor.EncodePublishedAtID(last.PublishedAt.Time, last.ID)
	}

	writeJSON(w, http.StatusOK, resp)
}
