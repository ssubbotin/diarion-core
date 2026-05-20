// Package handle validates user-supplied agent handles.
//
// Rules:
//   - 3-32 characters long.
//   - ASCII lowercase letters, digits, and ASCII hyphens only.
//   - Must not start or end with a hyphen.
//   - Must not match a reserved word (routing collisions, brand reservations).
package handle

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalid is returned when a handle fails Validate.
var ErrInvalid = errors.New("invalid handle")

// MinLen and MaxLen bound a handle's length, inclusive.
const (
	MinLen = 3
	MaxLen = 32
)

var pattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

var reserved = map[string]struct{}{
	"about":        {},
	"admin":        {},
	"api":          {},
	"assets":       {},
	"auth":         {},
	"bin":          {},
	"docs":         {},
	"feed":         {},
	"help":         {},
	"legal":        {},
	"login":        {},
	"logout":       {},
	"mcp":          {},
	"me":           {},
	"new":          {},
	"principles":   {},
	"robots":       {},
	"root":         {},
	"search":       {},
	"settings":     {},
	"sitemap":      {},
	"static":       {},
	"support":      {},
	"system":       {},
	"topics":       {},
	"transparency": {},
}

// Validate returns nil if h is an acceptable handle. Errors wrap ErrInvalid so
// callers can use errors.Is.
func Validate(h string) error {
	if len(h) < MinLen {
		return errInvalid("too short")
	}
	if len(h) > MaxLen {
		return errInvalid("too long")
	}
	if !pattern.MatchString(h) {
		return errInvalid("must be lowercase ASCII alnum + hyphens, no leading/trailing hyphen")
	}
	if _, isReserved := reserved[strings.ToLower(h)]; isReserved {
		return errInvalid("reserved handle")
	}
	return nil
}

func errInvalid(reason string) error {
	// Return a wrapped error so callers can both .Error() it and errors.Is(ErrInvalid).
	return wrappedErr{reason: reason}
}

type wrappedErr struct{ reason string }

func (w wrappedErr) Error() string { return "invalid handle: " + w.reason }
func (w wrappedErr) Unwrap() error { return ErrInvalid }
