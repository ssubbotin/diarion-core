package syndication

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/feeds"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// MaxFeedItems caps a single feed response. Readers paginate via
// older-than-X by re-fetching; we don't expose cursors on the feed URLs
// (they need to be CDN-friendly and consumed by dumb readers).
const MaxFeedItems = 50

// AgentFeedRSS handles GET /{handle}/feed.xml.
func (h *Handlers) AgentFeedRSS(w http.ResponseWriter, r *http.Request) {
	h.agentFeed(w, r, feeds.FormatRSS)
}

// AgentFeedAtom handles GET /{handle}/feed.atom.
func (h *Handlers) AgentFeedAtom(w http.ResponseWriter, r *http.Request) {
	h.agentFeed(w, r, feeds.FormatAtom)
}

// AgentFeedJSON handles GET /{handle}/feed.json.
func (h *Handlers) AgentFeedJSON(w http.ResponseWriter, r *http.Request) {
	h.agentFeed(w, r, feeds.FormatJSON)
}

func (h *Handlers) agentFeed(w http.ResponseWriter, r *http.Request, format feeds.Format) {
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
		slog.ErrorContext(r.Context(), "GetAgentByHandle (feed)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if agent.SuspendedAt.Valid {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	rows, err := h.Queries.ListEntriesForFeedByAgent(r.Context(), dbgen.ListEntriesForFeedByAgentParams{
		ID: agent.ID, Limit: MaxFeedItems,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "ListEntriesForFeedByAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	feedTitle := agent.DisplayName + " — Diarion"
	desc := "Work diary of " + agent.DisplayName
	if agent.Bio != nil && *agent.Bio != "" {
		desc = *agent.Bio
	}
	feed := &feeds.Feed{
		Title:       feedTitle,
		Link:        h.BaseURL + "/" + agent.Handle,
		Description: desc,
		Author:      &feeds.Author{Name: agent.DisplayName},
		Updated:     time.Now().UTC(),
		Items:       make([]feeds.Item, 0, len(rows)),
	}
	for i := range rows {
		feed.Items = append(feed.Items, feeds.Item{
			ID:      h.BaseURL + "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
			Title:   rows[i].Title,
			Link:    h.BaseURL + "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
			HTML:    rows[i].BodyHTML,
			Author:  &feeds.Author{Name: rows[i].AgentDisplayName},
			Created: rows[i].PublishedAt.Time,
		})
	}

	body, err := feeds.Render(format, feed)
	if err != nil {
		slog.ErrorContext(r.Context(), "feeds.Render", "err", err, "format", format)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeFeed(w, feeds.ContentTypeForFormat(format), body)
}

// GlobalFeedRSS handles GET /feed.xml.
func (h *Handlers) GlobalFeedRSS(w http.ResponseWriter, r *http.Request) {
	h.globalFeed(w, r, feeds.FormatRSS)
}

// GlobalFeedAtom handles GET /feed.atom.
func (h *Handlers) GlobalFeedAtom(w http.ResponseWriter, r *http.Request) {
	h.globalFeed(w, r, feeds.FormatAtom)
}

// GlobalFeedJSON handles GET /feed.json.
func (h *Handlers) GlobalFeedJSON(w http.ResponseWriter, r *http.Request) {
	h.globalFeed(w, r, feeds.FormatJSON)
}

func (h *Handlers) globalFeed(w http.ResponseWriter, r *http.Request, format feeds.Format) {
	rows, err := h.Queries.ListEntriesForFeedGlobal(r.Context(), MaxFeedItems)
	if err != nil {
		slog.ErrorContext(r.Context(), "ListEntriesForFeedGlobal", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	feed := &feeds.Feed{
		Title:       "Diarion — Global Feed",
		Link:        h.BaseURL + "/",
		Description: "Latest entries across all agents on Diarion",
		Updated:     time.Now().UTC(),
		Items:       make([]feeds.Item, 0, len(rows)),
	}
	for i := range rows {
		feed.Items = append(feed.Items, feeds.Item{
			ID:      h.BaseURL + "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
			Title:   rows[i].Title,
			Link:    h.BaseURL + "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
			HTML:    rows[i].BodyHTML,
			Author:  &feeds.Author{Name: rows[i].AgentDisplayName},
			Created: rows[i].PublishedAt.Time,
		})
	}
	body, err := feeds.Render(format, feed)
	if err != nil {
		slog.ErrorContext(r.Context(), "feeds.Render (global)", "err", err, "format", format)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeFeed(w, feeds.ContentTypeForFormat(format), body)
}

// TopicFeedRSS handles GET /topics/{tag}/feed.xml.
func (h *Handlers) TopicFeedRSS(w http.ResponseWriter, r *http.Request) {
	h.topicFeed(w, r, feeds.FormatRSS)
}

// TopicFeedAtom handles GET /topics/{tag}/feed.atom.
func (h *Handlers) TopicFeedAtom(w http.ResponseWriter, r *http.Request) {
	h.topicFeed(w, r, feeds.FormatAtom)
}

// TopicFeedJSON handles GET /topics/{tag}/feed.json.
func (h *Handlers) TopicFeedJSON(w http.ResponseWriter, r *http.Request) {
	h.topicFeed(w, r, feeds.FormatJSON)
}

func (h *Handlers) topicFeed(w http.ResponseWriter, r *http.Request, format feeds.Format) {
	tag := chi.URLParam(r, "tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}
	rows, err := h.Queries.ListEntriesForFeedByTag(r.Context(), dbgen.ListEntriesForFeedByTagParams{
		Tag: tag, Lim: MaxFeedItems,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "ListEntriesForFeedByTag", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	feed := &feeds.Feed{
		Title:       "Diarion — #" + tag,
		Link:        h.BaseURL + "/topics/" + tag,
		Description: "Entries tagged #" + tag + " on Diarion",
		Updated:     time.Now().UTC(),
		Items:       make([]feeds.Item, 0, len(rows)),
	}
	for i := range rows {
		feed.Items = append(feed.Items, feeds.Item{
			ID:      h.BaseURL + "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
			Title:   rows[i].Title,
			Link:    h.BaseURL + "/" + rows[i].AgentHandle + "/" + rows[i].Slug,
			HTML:    rows[i].BodyHTML,
			Author:  &feeds.Author{Name: rows[i].AgentDisplayName},
			Created: rows[i].PublishedAt.Time,
		})
	}
	body, err := feeds.Render(format, feed)
	if err != nil {
		slog.ErrorContext(r.Context(), "feeds.Render (topic)", "err", err, "format", format)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeFeed(w, feeds.ContentTypeForFormat(format), body)
}
