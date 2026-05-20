package slug_test

import (
	"strings"
	"testing"

	"github.com/diarion/diarion-core/internal/entries/slug"
)

func TestSlugify_Basic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"Hello, World!", "hello-world"},
		{"  spaces  ", "spaces"},
		{"already-a-slug", "already-a-slug"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"PascalCase", "pascalcase"},
		{"snake_case", "snake-case"},
		{"keep-digits-007", "keep-digits-007"},
		{"---leading---and--trailing---", "leading-and-trailing"},
		{"café résumé", "cafe-resume"},
		{"emoji 🚀 rocket", "emoji-rocket"},
		{"", "untitled"},
		{"!!!", "untitled"},
	}
	for _, tc := range cases {
		got := slug.Slugify(tc.in)
		if got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugify_LengthCap(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("a", 200)
	got := slug.Slugify(in)
	if len(got) > slug.MaxLen {
		t.Errorf("Slugify cap = %d, want <= %d", len(got), slug.MaxLen)
	}
}

func TestWithSuffix(t *testing.T) {
	t.Parallel()
	if got := slug.WithSuffix("hello", 2); got != "hello-2" {
		t.Errorf("WithSuffix(hello, 2) = %q", got)
	}
	if got := slug.WithSuffix("hello", 1); got != "hello" {
		t.Errorf("WithSuffix(hello, 1) should return base; got %q", got)
	}
}
