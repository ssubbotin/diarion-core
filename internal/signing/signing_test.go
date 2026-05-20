package signing_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/signing"
)

func newKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	sum := sha256.Sum256(pub)
	return pub, priv, hex.EncodeToString(sum[:])
}

// fakeFetcher implements signing.KeyFetcher with a single in-memory entry.
type fakeFetcher struct {
	fingerprint string
	pub         ed25519.PublicKey
	agentID     int64
	keyID       int64
	revoked     bool
	notFound    bool
}

func (f *fakeFetcher) Fetch(_ context.Context, fp string) (*signing.AgentKeyRecord, error) {
	if f.notFound || fp != f.fingerprint {
		return nil, signing.ErrKeyNotFound
	}
	if f.revoked {
		return nil, signing.ErrKeyRevoked
	}
	return &signing.AgentKeyRecord{
		Fingerprint: f.fingerprint,
		PublicKey:   f.pub,
		AgentID:     f.agentID,
		KeyID:       f.keyID,
	}, nil
}

func TestSignAndVerify_RoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv, fp := newKeypair(t)

	body := []byte(`{"title":"hello","body_markdown":"# hi","tags":["test"]}`)
	prev := bytes.Repeat([]byte{0}, 32)

	req, err := http.NewRequest(http.MethodPost,
		"http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Host = "example.test"
	if err := signing.Sign(req, body, fp, priv, prev); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	for _, h := range []string{"Content-Digest", "Signature-Input", "Signature", "Diarion-Key-Id", "Diarion-Prev-Entry-Hash"} {
		if req.Header.Get(h) == "" {
			t.Errorf("expected header %q to be set", h)
		}
	}

	v := signing.NewVerifier(&fakeFetcher{fingerprint: fp, pub: pub, agentID: 7, keyID: 13})
	got, err := v.Verify(req, body)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.KeyFingerprint != fp {
		t.Errorf("KeyFingerprint = %q, want %q", got.KeyFingerprint, fp)
	}
	if got.AgentID != 7 || got.KeyID != 13 {
		t.Errorf("agent/key mapping wrong: %+v", got)
	}
	if len(got.ContentDigest) != 32 {
		t.Errorf("ContentDigest len = %d, want 32", len(got.ContentDigest))
	}
	if !bytes.Equal(got.PrevEntryHash, prev) {
		t.Errorf("PrevEntryHash mismatch")
	}
	if len(got.Signature) == 0 {
		t.Errorf("Signature bytes not extracted")
	}
}

func TestVerify_TamperedBody(t *testing.T) {
	t.Parallel()
	pub, priv, fp := newKeypair(t)
	body := []byte(`{"title":"hi","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)

	req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	req.Host = "example.test"
	_ = signing.Sign(req, body, fp, priv, prev)

	tampered := append([]byte(nil), body...)
	tampered[1] ^= 0x01

	v := signing.NewVerifier(&fakeFetcher{fingerprint: fp, pub: pub})
	if _, err := v.Verify(req, tampered); err == nil {
		t.Errorf("Verify must fail when body bytes don't match signed Content-Digest")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	t.Parallel()
	_, priv, fp := newKeypair(t)
	otherPub, _, _ := newKeypair(t)
	body := []byte(`{}`)
	prev := bytes.Repeat([]byte{0}, 32)

	req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	req.Host = "example.test"
	_ = signing.Sign(req, body, fp, priv, prev)

	v := signing.NewVerifier(&fakeFetcher{fingerprint: fp, pub: otherPub})
	if _, err := v.Verify(req, body); err == nil {
		t.Errorf("Verify must fail when the fetcher returns a different pubkey")
	}
}

func TestVerify_KeyNotFound(t *testing.T) {
	t.Parallel()
	_, priv, fp := newKeypair(t)
	body := []byte(`{}`)
	prev := bytes.Repeat([]byte{0}, 32)

	req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	req.Host = "example.test"
	_ = signing.Sign(req, body, fp, priv, prev)

	v := signing.NewVerifier(&fakeFetcher{notFound: true})
	_, err := v.Verify(req, body)
	if err == nil || !errors.Is(err, signing.ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound; got %v", err)
	}
}

func TestVerify_RevokedKey(t *testing.T) {
	t.Parallel()
	pub, priv, fp := newKeypair(t)
	body := []byte(`{}`)
	prev := bytes.Repeat([]byte{0}, 32)

	req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	req.Host = "example.test"
	_ = signing.Sign(req, body, fp, priv, prev)

	v := signing.NewVerifier(&fakeFetcher{fingerprint: fp, pub: pub, revoked: true})
	_, err := v.Verify(req, body)
	if err == nil || !errors.Is(err, signing.ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked; got %v", err)
	}
}

func TestVerify_StaleCreated(t *testing.T) {
	t.Parallel()
	pub, priv, fp := newKeypair(t)
	body := []byte(`{}`)
	prev := bytes.Repeat([]byte{0}, 32)

	req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	req.Host = "example.test"

	if err := signing.SignAt(req, body, fp, priv, prev, time.Now().Add(-6*time.Minute)); err != nil {
		t.Fatalf("SignAt: %v", err)
	}
	v := signing.NewVerifier(&fakeFetcher{fingerprint: fp, pub: pub})
	if _, err := v.Verify(req, body); err == nil || !errors.Is(err, signing.ErrStaleSignature) {
		t.Errorf("expected ErrStaleSignature for old `created`; got %v", err)
	}
}

func TestVerify_PrevHashFromHeader(t *testing.T) {
	t.Parallel()
	pub, priv, fp := newKeypair(t)
	body := []byte(`{}`)
	prev := bytes.Repeat([]byte{0xAB}, 32)

	req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/v1/agents/ada/entries", bytes.NewReader(body))
	req.Host = "example.test"
	_ = signing.Sign(req, body, fp, priv, prev)

	if got := req.Header.Get("Diarion-Prev-Entry-Hash"); !strings.EqualFold(got, hex.EncodeToString(prev)) {
		t.Errorf("Diarion-Prev-Entry-Hash header = %q, want %s", got, hex.EncodeToString(prev))
	}

	v := signing.NewVerifier(&fakeFetcher{fingerprint: fp, pub: pub})
	got, err := v.Verify(req, body)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !bytes.Equal(got.PrevEntryHash, prev) {
		t.Errorf("PrevEntryHash round-trip mismatch")
	}
}
