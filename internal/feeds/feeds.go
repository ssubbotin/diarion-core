// Package feeds renders diary entries as RSS 2.0, Atom 1.0, or JSON Feed 1.1.
// Thin wrapper around github.com/gorilla/feeds with a small typed surface.
package feeds

import (
	"fmt"
	"time"

	gfeeds "github.com/gorilla/feeds"
)

// Format identifies a feed serialisation format.
type Format string

// Supported feed formats.
const (
	FormatRSS  Format = "rss"
	FormatAtom Format = "atom"
	FormatJSON Format = "json"
)

// Author is the byline on a feed or item.
type Author struct {
	Name  string
	Email string
}

// Item is one entry in the feed.
type Item struct {
	ID         string
	Title      string
	Link       string
	HTML       string
	Author     *Author
	Created    time.Time
	Updated    time.Time
	Categories []string
}

// Feed is the top-level feed document.
type Feed struct {
	Title       string
	Link        string
	Description string
	Author      *Author
	Updated     time.Time
	Items       []Item
}

// Render serialises a Feed into the chosen format and returns the bytes.
func Render(format Format, f *Feed) ([]byte, error) {
	gf := toGorillaFeed(f)
	switch format {
	case FormatRSS:
		s, err := gf.ToRss()
		if err != nil {
			return nil, fmt.Errorf("feeds.Render: rss: %w", err)
		}
		return []byte(s), nil
	case FormatAtom:
		s, err := gf.ToAtom()
		if err != nil {
			return nil, fmt.Errorf("feeds.Render: atom: %w", err)
		}
		return []byte(s), nil
	case FormatJSON:
		s, err := gf.ToJSON()
		if err != nil {
			return nil, fmt.Errorf("feeds.Render: json: %w", err)
		}
		return []byte(s), nil
	default:
		return nil, fmt.Errorf("feeds.Render: unknown format %q", format)
	}
}

// ContentTypeForFormat returns the HTTP Content-Type header value for a format.
func ContentTypeForFormat(f Format) string {
	switch f {
	case FormatRSS:
		return "application/rss+xml; charset=utf-8"
	case FormatAtom:
		return "application/atom+xml; charset=utf-8"
	case FormatJSON:
		return "application/feed+json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func toGorillaFeed(f *Feed) *gfeeds.Feed {
	out := &gfeeds.Feed{
		Title:       f.Title,
		Link:        &gfeeds.Link{Href: f.Link},
		Description: f.Description,
		Updated:     f.Updated,
	}
	if f.Author != nil {
		out.Author = &gfeeds.Author{Name: f.Author.Name, Email: f.Author.Email}
	}
	for i := range f.Items {
		out.Items = append(out.Items, toGorillaItem(&f.Items[i]))
	}
	return out
}

func toGorillaItem(it *Item) *gfeeds.Item {
	out := &gfeeds.Item{
		Id:      it.ID,
		Title:   it.Title,
		Link:    &gfeeds.Link{Href: it.Link},
		Content: it.HTML,
		Created: it.Created,
		Updated: it.Updated,
	}
	if it.Author != nil {
		out.Author = &gfeeds.Author{Name: it.Author.Name, Email: it.Author.Email}
	}
	return out
}
