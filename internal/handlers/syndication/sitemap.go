package syndication

import (
	"encoding/xml"
	"log/slog"
	"net/http"
)

type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// Sitemap handles GET /sitemap.xml.
//
// Emits sitemap protocol 0.9: every live, non-suspended agent profile plus
// every live entry permalink. Capped at the natural query limit; Phase 1
// volume is well under the 50k single-sitemap ceiling. A sitemap index lands
// when the corpus grows past 10k URLs (M7+ task).
func (h *Handlers) Sitemap(w http.ResponseWriter, r *http.Request) {
	agents, err := h.Queries.ListAgentsForSitemap(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "ListAgentsForSitemap", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	entries, err := h.Queries.ListEntriesForSitemap(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "ListEntriesForSitemap", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	set := sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  make([]sitemapURL, 0, len(agents)+len(entries)+1),
	}
	set.URLs = append(set.URLs, sitemapURL{Loc: h.BaseURL + "/"})
	for i := range agents {
		u := sitemapURL{Loc: h.BaseURL + "/" + agents[i].Handle}
		if agents[i].UpdatedAt.Valid {
			u.LastMod = agents[i].UpdatedAt.Time.UTC().Format("2006-01-02")
		}
		set.URLs = append(set.URLs, u)
	}
	for i := range entries {
		u := sitemapURL{Loc: h.BaseURL + "/" + entries[i].AgentHandle + "/" + entries[i].Slug}
		if entries[i].PublishedAt.Valid {
			u.LastMod = entries[i].PublishedAt.Time.UTC().Format("2006-01-02")
		}
		set.URLs = append(set.URLs, u)
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		slog.ErrorContext(r.Context(), "xml.MarshalIndent (sitemap)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}
