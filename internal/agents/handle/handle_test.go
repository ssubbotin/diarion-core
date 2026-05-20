package handle_test

import (
	"strings"
	"testing"

	"github.com/diarion/diarion-core/internal/agents/handle"
)

func TestValidate_Valid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"alice",
		"alice-bot",
		"diarion-build",
		"a1b",
		strings.Repeat("a", 32),
		"007-jane",
		"double--hyphen",
	}
	for _, h := range cases {
		if err := handle.Validate(h); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", h, err)
		}
	}
}

func TestValidate_Invalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		reason string
	}{
		{"", "empty"},
		{"ab", "too short"},
		{strings.Repeat("a", 33), "too long"},
		{"-leading-hyphen", "leading hyphen"},
		{"trailing-hyphen-", "trailing hyphen"},
		{"UPPER", "uppercase"},
		{"with space", "whitespace"},
		{"under_score", "underscore"},
		{"emoji-🙂", "non-ascii"},
	}
	for _, tc := range cases {
		if err := handle.Validate(tc.in); err == nil {
			t.Errorf("Validate(%q) = nil, want error (%s)", tc.in, tc.reason)
		}
	}
}

func TestValidate_Reserved(t *testing.T) {
	t.Parallel()
	for _, r := range []string{"api", "auth", "me", "settings", "mcp", "admin", "new"} {
		if err := handle.Validate(r); err == nil {
			t.Errorf("Validate(%q) should reject reserved handle", r)
		}
	}
}
