// Package keys composes Ed25519 keypair generation with envelope encryption so
// callers don't need to know the wire format of managed-key storage.
package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/diarion/diarion-core/internal/crypto/envelope"
)

// Custody enumerates the two key-custody modes.
type Custody string

// Custody values for the two supported key-custody modes.
const (
	CustodyManaged Custody = "managed"
	CustodySelf    Custody = "self"
)

// Issued bundles everything a caller needs after generating a fresh keypair.
//
// For managed custody:
//   - EncryptedPrivateKey is set (envelope ciphertext to store in DB).
//   - PlaintextPrivateKey is nil and must never be returned to the user.
//
// For self custody:
//   - EncryptedPrivateKey is nil (server never persists the private key).
//   - PlaintextPrivateKey is set and MUST be returned to the user exactly once.
type Issued struct {
	PublicKey           ed25519.PublicKey
	Fingerprint         string
	EncryptedPrivateKey []byte
	PlaintextPrivateKey ed25519.PrivateKey
}

// ErrInvalidCustody is returned when Custody is neither CustodyManaged nor
// CustodySelf.
var ErrInvalidCustody = errors.New("keys: invalid custody value")

// Issue generates a fresh Ed25519 keypair and packages it for the chosen
// custody mode.
func Issue(c Custody, masterKey []byte) (*Issued, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("keys.Issue: generate: %w", err)
	}

	out := &Issued{
		PublicKey:   pub,
		Fingerprint: Fingerprint(pub),
	}

	switch c {
	case CustodyManaged:
		// Scrub the plaintext private key on the way out — we keep only the
		// envelope-wrapped copy in EncryptedPrivateKey.
		defer func() {
			for i := range priv {
				priv[i] = 0
			}
		}()
		env, err := envelope.Wrap(masterKey, priv)
		if err != nil {
			return nil, fmt.Errorf("keys.Issue: wrap: %w", err)
		}
		out.EncryptedPrivateKey = env
	case CustodySelf:
		// Hand the plaintext private key to the caller; do not persist.
		out.PlaintextPrivateKey = priv
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidCustody, c)
	}

	return out, nil
}

// Unwrap returns the plaintext Ed25519 private key from an envelope.
//
// Callers MUST scrub the returned buffer when done; in practice this means
// pairing every Unwrap with a defer that overwrites the slice.
func Unwrap(masterKey, envelopeBytes []byte) (ed25519.PrivateKey, error) {
	plain, err := envelope.Unwrap(masterKey, envelopeBytes)
	if err != nil {
		return nil, fmt.Errorf("keys.Unwrap: %w", err)
	}
	if len(plain) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("keys.Unwrap: unexpected privkey length %d", len(plain))
	}
	return ed25519.PrivateKey(plain), nil
}

// Fingerprint is hex(sha256(public_key)).
func Fingerprint(pub []byte) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])
}
