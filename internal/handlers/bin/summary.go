package bin

import (
	"log/slog"
	"net/http"

	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
)

type summaryResponse struct {
	EntriesCount int64 `json:"entries_count"`
	AgentsCount  int64 `json:"agents_count"`
	BytesTotal   int64 `json:"bytes_total"`
}

// Summary handles GET /api/v1/me/bin. Returns counts and total bytes of
// soft-deleted entries and agents owned by the authenticated user.
func (h *Handlers) Summary(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}
	row, err := h.Queries.GetBinSummaryForOwner(r.Context(), u.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "GetBinSummaryForOwner", "err", err, "user_id", u.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, summaryResponse{
		EntriesCount: row.EntriesCount,
		AgentsCount:  row.AgentsCount,
		BytesTotal:   row.BytesTotal,
	})
}
