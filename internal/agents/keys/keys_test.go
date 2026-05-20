package keys_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/diarion/diarion-core/internal/agents/keys"
)

func newMasterKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestIssue_Managed_RoundTrip(t *testing.T) {
	t.Parallel()
	master := newMasterKey(t)

	is, err := keys.Issue(keys.CustodyManaged, master)
	if err != nil {
		t.Fatalf("Issue managed: %v", err)
	}

	if len(is.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("PublicKey len = %d, want %d", len(is.PublicKey), ed25519.PublicKeySize)
	}
	if is.Fingerprint == "" {
		t.Errorf("Fingerprint should be set")
	}
	if is.EncryptedPrivateKey == nil {
		t.Fatalf("EncryptedPrivateKey should be set for managed custody")
	}
	if is.PlaintextPrivateKey != nil {
		t.Errorf("PlaintextPrivateKey must be nil for managed custody")
	}

	// Decrypt the envelope and check signing works.
	priv, err := keys.Unwrap(master, is.EncryptedPrivateKey)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	msg := []byte("hello diarion")
	sig := ed25519.Sign(priv, msg)
	if !ed25519.Verify(is.PublicKey, msg, sig) {
		t.Errorf("signature did not verify against returned pubkey")
	}
}

func TestIssue_Self_ReturnsPlaintextNoEnvelope(t *testing.T) {
	t.Parallel()
	master := newMasterKey(t)

	is, err := keys.Issue(keys.CustodySelf, master)
	if err != nil {
		t.Fatalf("Issue self: %v", err)
	}
	if is.EncryptedPrivateKey != nil {
		t.Errorf("self custody must not produce an envelope")
	}
	if len(is.PlaintextPrivateKey) != ed25519.PrivateKeySize {
		t.Fatalf("PlaintextPrivateKey len = %d, want %d", len(is.PlaintextPrivateKey), ed25519.PrivateKeySize)
	}

	// Signing with the returned plaintext should verify against PublicKey.
	sig := ed25519.Sign(is.PlaintextPrivateKey, []byte("self-mode test"))
	if !ed25519.Verify(is.PublicKey, []byte("self-mode test"), sig) {
		t.Errorf("self-mode signature did not verify")
	}
}

func TestIssue_InvalidCustody(t *testing.T) {
	t.Parallel()
	_, err := keys.Issue("nope", newMasterKey(t))
	if err == nil {
		t.Fatalf("expected error for invalid custody value")
	}
	if !errors.Is(err, keys.ErrInvalidCustody) {
		t.Errorf("expected ErrInvalidCustody in the chain, got %v", err)
	}
}

func TestFingerprint_DeterministicAndHex(t *testing.T) {
	t.Parallel()
	pub := bytes.Repeat([]byte{0xAB}, ed25519.PublicKeySize)
	fp1 := keys.Fingerprint(pub)
	fp2 := keys.Fingerprint(pub)
	if fp1 != fp2 {
		t.Errorf("fingerprint must be deterministic")
	}
	if len(fp1) != 64 { // 32-byte sha256 → 64 hex chars
		t.Errorf("fingerprint len = %d, want 64", len(fp1))
	}
}
