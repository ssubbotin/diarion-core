// Package slug converts entry titles into URL-safe slugs.
//
// Rules:
//   - Unicode → ASCII via NFKD-decomposition + diacritic stripping.
//   - Lowercase.
//   - Non-alphanumeric runs collapse to single hyphens.
//   - Leading/trailing hyphens trimmed.
//   - Empty / unslugifiable inputs return "untitled".
//   - Capped at MaxLen (80) to leave room for `-N` collision suffixes.
package slug

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// MaxLen caps the slug length before any `-N` suffix is appended.
const MaxLen = 80

var asciiFolder = transform.Chain(
	norm.NFKD,
	runes.Remove(runes.In(unicode.Mn)), // strip combining marks (diacritics)
	norm.NFC,
)

// Slugify converts a title to a slug. Always returns at least "untitled".
func Slugify(title string) string {
	folded, _, err := transform.String(asciiFolder, title)
	if err != nil {
		folded = title
	}
	folded = strings.ToLower(folded)

	var b strings.Builder
	b.Grow(len(folded))
	lastHyphen := true // suppresses leading hyphen
	for _, r := range folded {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	if len(out) > MaxLen {
		out = strings.TrimRight(out[:MaxLen], "-")
	}
	return out
}

// WithSuffix returns base for n==1; base + "-" + n for n>=2. Used by the
// handler when colliding on (agent_id, slug) UNIQUE constraint.
func WithSuffix(base string, n int) string {
	if n <= 1 {
		return base
	}
	return base + "-" + strconv.Itoa(n)
}
