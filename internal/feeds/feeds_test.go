package feeds_test

import (
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/feeds"
)

func sampleFeed() *feeds.Feed {
	return &feeds.Feed{
		Title:       "Sample Diary",
		Link:        "https://diarion.app/sample",
		Description: "A diary",
		Updated:     time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Author:      &feeds.Author{Name: "Sample Agent"},
		Items: []feeds.Item{
			{
				ID:         "https://diarion.app/sample/post-1",
				Title:      "Post 1",
				Link:       "https://diarion.app/sample/post-1",
				HTML:       "<p>hello world</p>",
				Author:     &feeds.Author{Name: "Sample Agent"},
				Created:    time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
				Categories: []string{"tag-a"},
			},
		},
	}
}

func TestRender_RSS(t *testing.T) {
	t.Parallel()
	out, err := feeds.Render(feeds.FormatRSS, sampleFeed())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<rss") {
		t.Errorf("expected <rss> tag; got: %s", s)
	}
	if !strings.Contains(s, "<title>Sample Diary</title>") {
		t.Errorf("expected channel title")
	}
	if !strings.Contains(s, "<title>Post 1</title>") {
		t.Errorf("expected item title")
	}
}

func TestRender_Atom(t *testing.T) {
	t.Parallel()
	out, err := feeds.Render(feeds.FormatAtom, sampleFeed())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<feed xmlns=") {
		t.Errorf("expected atom <feed> tag; got: %s", s)
	}
	if !strings.Contains(s, "Sample Diary") {
		t.Errorf("expected feed title")
	}
}

func TestRender_JSON(t *testing.T) {
	t.Parallel()
	out, err := feeds.Render(feeds.FormatJSON, sampleFeed())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"version"`) {
		t.Errorf("expected JSON Feed version field")
	}
	if !strings.Contains(s, `"Sample Diary"`) {
		t.Errorf("expected feed title in JSON")
	}
}

func TestRender_UnknownFormat(t *testing.T) {
	t.Parallel()
	_, err := feeds.Render("xml-junk", sampleFeed())
	if err == nil {
		t.Errorf("expected error for unknown format")
	}
}

func TestContentTypeForFormat(t *testing.T) {
	t.Parallel()
	cases := map[feeds.Format]string{
		feeds.FormatRSS:  "application/rss+xml; charset=utf-8",
		feeds.FormatAtom: "application/atom+xml; charset=utf-8",
		feeds.FormatJSON: "application/feed+json; charset=utf-8",
	}
	for f, expected := range cases {
		if got := feeds.ContentTypeForFormat(f); got != expected {
			t.Errorf("ContentTypeForFormat(%q) = %q, want %q", f, got, expected)
		}
	}
}
