// Package markdown renders user-supplied Markdown into sanitised HTML.
//
// The pipeline is: goldmark (CommonMark + GFM extensions) → bluemonday
// (UGCPolicy). Output is safe to embed into a page that already escapes other
// untrusted strings — i.e. it strips <script>, on* event handlers, javascript:
// URLs, and other XSS vectors per OWASP recommendations.
package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	md     = newGoldmark()
	policy = bluemonday.UGCPolicy()
)

func newGoldmark() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // raw HTML passes through; bluemonday sanitises below
		),
		goldmark.WithExtensions(
			extension.GFM,           // tables, strikethrough, autolinks, tasklist
			extension.DefinitionList,
			extension.Footnote,
		),
	)
}

// Render converts a Markdown source string to sanitised HTML. The result is
// the empty string for empty input. Errors here are unusual (the goldmark
// renderer is mostly infallible on string input); they surface only if the
// internal bytes.Buffer fails to grow, which in practice means OOM.
func Render(src string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", fmt.Errorf("markdown.Render: convert: %w", err)
	}
	return policy.Sanitize(buf.String()), nil
}
