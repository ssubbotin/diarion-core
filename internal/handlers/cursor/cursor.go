// Package cursor encodes/decodes opaque pagination cursors.
//
// Two shapes:
//
//   - Time cursor (published_at + id): used by the global feed at
//     /api/v1/entries. Encodes as 16 bytes (8 int64 unix nanos UTC + 8 int64 id)
//     wrapped in URL-safe base64. Stable across server restarts and time zones.
//   - Rank cursor (rank + id): used by /api/v1/search. Encodes as 12 bytes
//     (4 float32 bits + 8 int64 id) wrapped in URL-safe base64. Float bits are
//     compared exactly by Postgres so the cursor produces deterministic
//     pagination across pages.
//
// Decode errors map to HTTP 400 — the cursor is from a previous response, so
// any tampering or version skew is a client bug.
package cursor

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"time"
)

const (
	timeCursorLen = 16 // 8 (unix nanos) + 8 (id)
	rankCursorLen = 12 // 4 (rank bits) + 8 (id)
)

// ErrInvalidCursor is returned for any decode failure (bad base64, wrong
// length, etc.). Handlers should treat this as a 400 Bad Request.
var ErrInvalidCursor = errors.New("cursor: invalid")

// EncodePublishedAtID encodes a time + id pair into an opaque cursor string.
// The time is normalised to UTC; nanosecond precision is preserved.
func EncodePublishedAtID(t time.Time, id int64) string {
	t = t.UTC()
	var buf [timeCursorLen]byte
	// Bit-exact round-trip via uint encoding; signedness is preserved by
	// the matching int64() cast in DecodePublishedAtID.
	binary.BigEndian.PutUint64(buf[0:8], uint64(t.UnixNano())) //nolint:gosec // bit-exact cursor encoding
	binary.BigEndian.PutUint64(buf[8:16], uint64(id))          //nolint:gosec // bit-exact cursor encoding
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

// DecodePublishedAtID reverses EncodePublishedAtID. Returned time is in UTC.
func DecodePublishedAtID(s string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) != timeCursorLen {
		return time.Time{}, 0, ErrInvalidCursor
	}
	// Reverse of EncodePublishedAtID — bit-exact, matched signed/unsigned pair.
	nanos := int64(binary.BigEndian.Uint64(raw[0:8])) //nolint:gosec // bit-exact cursor decoding
	id := int64(binary.BigEndian.Uint64(raw[8:16]))   //nolint:gosec // bit-exact cursor decoding
	return time.Unix(0, nanos).UTC(), id, nil
}

// EncodeRankID encodes a search rank + id pair into an opaque cursor string.
// Rank is a float32 (ts_rank produces real / float4); bit-exact preservation
// is important so Postgres comparisons match what the prior page produced.
func EncodeRankID(rank float32, id int64) string {
	var buf [rankCursorLen]byte
	binary.BigEndian.PutUint32(buf[0:4], math.Float32bits(rank))
	binary.BigEndian.PutUint64(buf[4:12], uint64(id)) //nolint:gosec // bit-exact cursor encoding
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

// DecodeRankID reverses EncodeRankID.
func DecodeRankID(s string) (float32, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) != rankCursorLen {
		return 0, 0, ErrInvalidCursor
	}
	rank := math.Float32frombits(binary.BigEndian.Uint32(raw[0:4]))
	id := int64(binary.BigEndian.Uint64(raw[4:12])) //nolint:gosec // bit-exact cursor decoding
	return rank, id, nil
}
