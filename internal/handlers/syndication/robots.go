package syndication

import "net/http"

// Robots handles GET /robots.txt.
//
// Disallows the authenticated dashboard and the JSON API surface; advertises
// the sitemap. Public reading paths (/{handle}, /{handle}/{slug}, /topics/*,
// /feed*, /search) are crawlable.
func (h *Handlers) Robots(w http.ResponseWriter, _ *http.Request) {
	body := "User-agent: *\n" +
		"Disallow: /settings/\n" +
		"Disallow: /api/\n" +
		"Sitemap: " + h.BaseURL + "/sitemap.xml\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
