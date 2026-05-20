package envelope_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/diarion/diarion-core/internal/crypto/envelope"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestWrapUnwrap_RoundTrip(t *testing.T) {
	t.Parallel()
	master := mustKey(t)
	plain := []byte("the quick brown ed25519 private key fox")

	wrapped, err := envelope.Wrap(master, plain)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if bytes.Contains(wrapped, plain) {
		t.Fatalf("plaintext leaked into ciphertext")
	}

	got, err := envelope.Unwrap(master, wrapped)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("Unwrap returned %x, want %x", got, plain)
	}
}

func TestWrap_DistinctOutputsPerCall(t *testing.T) {
	t.Parallel()
	master := mustKey(t)
	plain := []byte("the quick brown ed25519 private key fox")

	w1, _ := envelope.Wrap(master, plain)
	w2, _ := envelope.Wrap(master, plain)

	if bytes.Equal(w1, w2) {
		t.Errorf("two Wrap() calls of identical plaintext returned identical ciphertext; nonce reuse?")
	}
}

func TestUnwrap_RejectsTamperedCiphertext(t *testing.T) {
	t.Parallel()
	master := mustKey(t)
	plain := []byte("hello")

	wrapped, _ := envelope.Wrap(master, plain)

	// Flip one bit in the middle (in the encrypted DEK region).
	tampered := append([]byte(nil), wrapped...)
	tampered[20] ^= 0x01

	if _, err := envelope.Unwrap(master, tampered); err == nil {
		t.Errorf("Unwrap should fail on tampered ciphertext")
	}
}

func TestUnwrap_RejectsWrongMasterKey(t *testing.T) {
	t.Parallel()
	master1 := mustKey(t)
	master2 := mustKey(t)
	plain := []byte("hello")
	wrapped, _ := envelope.Wrap(master1, plain)

	if _, err := envelope.Unwrap(master2, wrapped); err == nil {
		t.Errorf("Unwrap should fail when master key differs")
	}
}

func TestUnwrap_RejectsBadVersion(t *testing.T) {
	t.Parallel()
	master := mustKey(t)
	plain := []byte("hello")
	wrapped, _ := envelope.Wrap(master, plain)

	// Bump version byte.
	bad := append([]byte(nil), wrapped...)
	bad[0] = 99
	if _, err := envelope.Unwrap(master, bad); err == nil {
		t.Errorf("Unwrap should reject unknown version")
	}
}

func TestWrap_RejectsBadMasterKey(t *testing.T) {
	t.Parallel()
	if _, err := envelope.Wrap(make([]byte, 16), []byte("data")); err == nil {
		t.Errorf("Wrap should reject non-32-byte master key")
	}
}

func TestErrTampered_IsForReturnedError(t *testing.T) {
	t.Parallel()
	master := mustKey(t)
	plain := []byte("hello")
	wrapped, _ := envelope.Wrap(master, plain)
	tampered := append([]byte(nil), wrapped...)
	tampered[len(tampered)-3] ^= 0xFF // flip in the payload-ciphertext tag region

	_, err := envelope.Unwrap(master, tampered)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, envelope.ErrTampered) {
		t.Errorf("expected ErrTampered, got %v", err)
	}
}
