// Package syndication serves the syndication endpoints — RSS/Atom/JSON feeds,
// /sitemap.xml, and /robots.txt — directly from diarion-api at the root
// (not under /api/v1). Per Phase 1 spec §5.8 these are JS-free and
// CDN-cacheable.
package syndication

import (
	"net/http"
	"strings"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/go-chi/chi/v5"
)

const cacheControl = "public, max-age=300"

// Handlers wires the syndication routes.
type Handlers struct {
	Queries dbgen.Querier
	BaseURL string // e.g. https://diarion.app — used to render absolute URLs in feeds & sitemap.
}

// New constructs Handlers.
func New(q dbgen.Querier, baseURL string) *Handlers {
	return &Handlers{Queries: q, BaseURL: strings.TrimRight(baseURL, "/")}
}

// Register attaches the syndication routes to r. Caller passes the top-level
// router (NOT prefix-mounted at /api/v1).
func (h *Handlers) Register(r chi.Router) {
	r.Get("/feed.xml", h.GlobalFeedRSS)
	r.Get("/feed.atom", h.GlobalFeedAtom)
	r.Get("/feed.json", h.GlobalFeedJSON)
	r.Get("/topics/{tag}/feed.xml", h.TopicFeedRSS)
	r.Get("/topics/{tag}/feed.atom", h.TopicFeedAtom)
	r.Get("/topics/{tag}/feed.json", h.TopicFeedJSON)
	r.Get("/sitemap.xml", h.Sitemap)
	r.Get("/robots.txt", h.Robots)
	r.Get("/{handle}/feed.xml", h.AgentFeedRSS)
	r.Get("/{handle}/feed.atom", h.AgentFeedAtom)
	r.Get("/{handle}/feed.json", h.AgentFeedJSON)
}

func writeFeed(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// Stubs filled in by subsequent tasks (8, 9, 10, 11).

// GlobalFeedRSS handles GET /feed.xml.
func (h *Handlers) GlobalFeedRSS(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GlobalFeedAtom handles GET /feed.atom.
func (h *Handlers) GlobalFeedAtom(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GlobalFeedJSON handles GET /feed.json.
func (h *Handlers) GlobalFeedJSON(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// TopicFeedRSS handles GET /topics/{tag}/feed.xml.
func (h *Handlers) TopicFeedRSS(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// TopicFeedAtom handles GET /topics/{tag}/feed.atom.
func (h *Handlers) TopicFeedAtom(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// TopicFeedJSON handles GET /topics/{tag}/feed.json.
func (h *Handlers) TopicFeedJSON(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Sitemap handles GET /sitemap.xml.
func (h *Handlers) Sitemap(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Robots handles GET /robots.txt.
func (h *Handlers) Robots(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
