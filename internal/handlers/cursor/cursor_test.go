package cursor_test

import (
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/handlers/cursor"
)

func TestEncodeDecodePublishedAtID_RoundTrip(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 5, 20, 12, 34, 56, 789_000_000, time.UTC)
	id := int64(424242)

	encoded := cursor.EncodePublishedAtID(when, id)
	if encoded == "" {
		t.Fatalf("empty encode")
	}
	gotWhen, gotID, err := cursor.DecodePublishedAtID(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !gotWhen.Equal(when) {
		t.Errorf("time mismatch: got %v want %v", gotWhen, when)
	}
	if gotID != id {
		t.Errorf("id mismatch: got %d want %d", gotID, id)
	}
}

func TestEncodeDecodeRankID_RoundTrip(t *testing.T) {
	t.Parallel()
	rank := float32(0.3729)
	id := int64(99)

	encoded := cursor.EncodeRankID(rank, id)
	gotRank, gotID, err := cursor.DecodeRankID(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if gotRank != rank {
		t.Errorf("rank mismatch: got %v want %v", gotRank, rank)
	}
	if gotID != id {
		t.Errorf("id mismatch")
	}
}

func TestDecode_RejectsGarbage(t *testing.T) {
	t.Parallel()
	if _, _, err := cursor.DecodePublishedAtID("not-base64-!!"); err == nil {
		t.Errorf("expected error on garbage")
	}
	if _, _, err := cursor.DecodePublishedAtID(""); err == nil {
		t.Errorf("expected error on empty string")
	}
	if _, _, err := cursor.DecodePublishedAtID("YQ"); err == nil {
		t.Errorf("expected error on wrong length")
	}
}

func TestDecode_RejectsTruncatedRank(t *testing.T) {
	t.Parallel()
	if _, _, err := cursor.DecodeRankID("YQ"); err == nil {
		t.Errorf("expected error on truncated rank cursor")
	}
}

func TestEncodePublishedAtID_NormalisesToUTC(t *testing.T) {
	t.Parallel()
	loc, _ := time.LoadLocation("America/New_York")
	when := time.Date(2026, 1, 1, 10, 0, 0, 0, loc)
	got, _, err := cursor.DecodePublishedAtID(cursor.EncodePublishedAtID(when, 1))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.Equal(when) {
		t.Errorf("equality lost across UTC normalisation: got %v want %v", got, when)
	}
	if got.Location() != time.UTC {
		t.Errorf("expected UTC; got %v", got.Location())
	}
}
