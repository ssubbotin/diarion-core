// Package signing implements the subset of RFC 9421 HTTP Message Signatures
// that the Diarion signed-entry envelope needs: ed25519 over a fixed set of
// derived/header components, with a `created` metadata parameter the verifier
// enforces against a ±5-minute clock-skew window.
//
// The implementation is library-agnostic on purpose. We use dunglas/httpsfv
// for RFC 8941 structured-fields parsing (the canonical implementation of the
// dictionary/inner-list wire form used by Signature-Input and Content-Digest),
// and go's standard crypto/ed25519 to sign and verify. A higher-level
// RFC 9421 library (remitly-oss/httpsig-go) was evaluated but rejected for
// this package: its Signer always derives `created` from time.Now() and the
// underlying sign() helper is unexported, so there is no way to write the
// SignAt entry point tests require. The narrow surface here — seven
// components, one algorithm, one signature label — does not justify carrying
// the larger library.
//
// The Diarion signed-entry envelope covers exactly these components, in this
// order:
//
//	("@method" "@authority" "@path" "@query"
//	 "content-digest" "diarion-key-id" "diarion-prev-entry-hash")
//
// with algorithm ed25519 and the `created` signature metadata parameter.
package signing

import (
	"context"
	"crypto/ed25519"
	"errors"
)

// Sentinel errors returned by Verify. Wrap these (errors.Is) at call sites
// rather than matching strings.
var (
	ErrKeyNotFound    = errors.New("signing: key not found")
	ErrKeyRevoked     = errors.New("signing: key revoked")
	ErrStaleSignature = errors.New("signing: signature outside 5-minute window")
	ErrBadDigest      = errors.New("signing: Content-Digest does not match body")
	ErrBadSignature   = errors.New("signing: signature verification failed")
)

// AgentKeyRecord is the per-fingerprint key material a KeyFetcher returns. The
// verifier uses PublicKey to check the signature; AgentID and KeyID are carried
// through to VerifyResult so handlers can attribute the request without an
// extra DB lookup.
type AgentKeyRecord struct {
	Fingerprint string
	PublicKey   ed25519.PublicKey
	AgentID     int64
	KeyID       int64
}

// KeyFetcher resolves a public-key fingerprint (the hex sha256 of the public
// key bytes, as carried in the Diarion-Key-Id request header) to the
// corresponding AgentKeyRecord. Implementations must return ErrKeyNotFound
// when no matching key exists and ErrKeyRevoked when a record exists but has
// been revoked.
type KeyFetcher interface {
	Fetch(ctx context.Context, fingerprint string) (*AgentKeyRecord, error)
}
