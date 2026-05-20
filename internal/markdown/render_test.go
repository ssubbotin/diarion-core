package markdown_test

import (
	"strings"
	"testing"

	"github.com/diarion/diarion-core/internal/markdown"
)

func TestRender_BasicMarkdown(t *testing.T) {
	t.Parallel()
	html, err := markdown.Render("# Hello\n\nA paragraph with **bold**.")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Hello</h1>") {
		t.Errorf("expected h1 with 'Hello'; got %q", html)
	}
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Errorf("expected <strong>bold</strong>; got %q", html)
	}
}

func TestRender_GFMTable(t *testing.T) {
	t.Parallel()
	src := "| a | b |\n|---|---|\n| 1 | 2 |\n"
	html, err := markdown.Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, "<table") {
		t.Errorf("GFM table not rendered: %q", html)
	}
}

func TestRender_StripsScript(t *testing.T) {
	t.Parallel()
	src := "Hello <script>alert(1)</script> world"
	html, err := markdown.Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(html, "<script") || strings.Contains(html, "alert(1)") {
		t.Errorf("script tag survived sanitisation: %q", html)
	}
}

func TestRender_StripsOnEventHandlers(t *testing.T) {
	t.Parallel()
	src := `Click <a href="https://example.com" onclick="evil()">me</a>.`
	html, err := markdown.Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(html, "onclick") {
		t.Errorf("onclick survived sanitisation: %q", html)
	}
	if !strings.Contains(html, `href="https://example.com"`) {
		t.Errorf("legitimate href stripped: %q", html)
	}
}

func TestRender_StripsJavaScriptURL(t *testing.T) {
	t.Parallel()
	src := `[click](javascript:alert(1))`
	html, err := markdown.Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(strings.ToLower(html), "javascript:") {
		t.Errorf("javascript: URL survived sanitisation: %q", html)
	}
}

func TestRender_Autolink(t *testing.T) {
	t.Parallel()
	html, _ := markdown.Render("See https://example.com for info.")
	if !strings.Contains(html, `href="https://example.com"`) {
		t.Errorf("expected autolinked URL; got %q", html)
	}
}

func TestRender_EmptyInput(t *testing.T) {
	t.Parallel()
	html, err := markdown.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if html != "" {
		t.Errorf("empty input should yield empty html; got %q", html)
	}
}
