// Package envelope implements AES-256-GCM envelope encryption suitable for
// per-record secrets wrapped under a long-lived master KEK.
//
// Layout of a wrapped envelope:
//
//	byte 0          : version (currently 1)
//	bytes 1..12     : 12-byte AES-GCM nonce for the KEK
//	bytes 13..44    : 32-byte DEK ciphertext (the DEK plaintext is 32 bytes; GCM keeps it the same length)
//	bytes 45..60    : 16-byte GCM tag for the wrapped DEK
//	bytes 61..72    : 12-byte AES-GCM nonce for the DEK
//	bytes 73..N+88  : payload ciphertext (= plaintext_len bytes) followed by a 16-byte GCM tag
//
// The total fixed overhead is 89 bytes plus the plaintext length.
package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// Version is the current envelope format version.
const Version byte = 1

// MasterKeyLen is the required length of the KEK.
const MasterKeyLen = 32

// dekLen is the length of the data-encryption key we generate per envelope.
const dekLen = 32

// gcmNonceLen is the AES-GCM nonce length (RFC 5116).
const gcmNonceLen = 12

// gcmTagLen is the AES-GCM authentication tag length.
const gcmTagLen = 16

// ErrTampered is returned when ciphertext fails authentication.
var ErrTampered = errors.New("envelope: ciphertext tampered or wrong key")

// ErrFormat is returned when the envelope bytes don't match the expected layout.
var ErrFormat = errors.New("envelope: invalid format")

// Wrap returns an envelope that encrypts plaintext under a freshly generated
// DEK, with the DEK itself wrapped by masterKey.
func Wrap(masterKey, plaintext []byte) ([]byte, error) {
	if len(masterKey) != MasterKeyLen {
		return nil, fmt.Errorf("envelope.Wrap: master key must be %d bytes, got %d", MasterKeyLen, len(masterKey))
	}

	dek := make([]byte, dekLen)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("envelope.Wrap: dek: %w", err)
	}
	defer scrub(dek)

	kekNonce := make([]byte, gcmNonceLen)
	if _, err := rand.Read(kekNonce); err != nil {
		return nil, fmt.Errorf("envelope.Wrap: kek nonce: %w", err)
	}

	dekNonce := make([]byte, gcmNonceLen)
	if _, err := rand.Read(dekNonce); err != nil {
		return nil, fmt.Errorf("envelope.Wrap: dek nonce: %w", err)
	}

	kekGCM, err := newGCM(masterKey)
	if err != nil {
		return nil, fmt.Errorf("envelope.Wrap: kek gcm: %w", err)
	}
	dekGCM, err := newGCM(dek)
	if err != nil {
		return nil, fmt.Errorf("envelope.Wrap: dek gcm: %w", err)
	}

	wrappedDEK := kekGCM.Seal(nil, kekNonce, dek, nil) // 32 ct + 16 tag = 48 bytes
	payload := dekGCM.Seal(nil, dekNonce, plaintext, nil)

	out := make([]byte, 0, 1+gcmNonceLen+len(wrappedDEK)+gcmNonceLen+len(payload))
	out = append(out, Version)
	out = append(out, kekNonce...)
	out = append(out, wrappedDEK...)
	out = append(out, dekNonce...)
	out = append(out, payload...)
	return out, nil
}

// Unwrap inverts Wrap. Returns ErrTampered if authentication fails,
// ErrFormat if the byte layout is wrong, or other errors for I/O failures.
func Unwrap(masterKey, envelopeBytes []byte) ([]byte, error) {
	if len(masterKey) != MasterKeyLen {
		return nil, fmt.Errorf("envelope.Unwrap: master key must be %d bytes, got %d", MasterKeyLen, len(masterKey))
	}
	const minLen = 1 + gcmNonceLen + dekLen + gcmTagLen + gcmNonceLen + gcmTagLen
	if len(envelopeBytes) < minLen {
		return nil, fmt.Errorf("%w: too short (%d bytes; want >= %d)", ErrFormat, len(envelopeBytes), minLen)
	}
	if envelopeBytes[0] != Version {
		return nil, fmt.Errorf("%w: unknown version %d", ErrFormat, envelopeBytes[0])
	}

	off := 1
	kekNonce := envelopeBytes[off : off+gcmNonceLen]
	off += gcmNonceLen
	wrappedDEK := envelopeBytes[off : off+dekLen+gcmTagLen]
	off += dekLen + gcmTagLen
	dekNonce := envelopeBytes[off : off+gcmNonceLen]
	off += gcmNonceLen
	payload := envelopeBytes[off:]

	kekGCM, err := newGCM(masterKey)
	if err != nil {
		return nil, fmt.Errorf("envelope.Unwrap: kek gcm: %w", err)
	}

	dek, err := kekGCM.Open(nil, kekNonce, wrappedDEK, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: unwrap dek: %w", ErrTampered, err)
	}
	defer scrub(dek)

	dekGCM, err := newGCM(dek)
	if err != nil {
		return nil, fmt.Errorf("envelope.Unwrap: dek gcm: %w", err)
	}

	plain, err := dekGCM.Open(nil, dekNonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt payload: %w", ErrTampered, err)
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// scrub overwrites the buffer in place. Best-effort against accidental log /
// heap-dump leakage; not a defence against memory-attached adversaries.
func scrub(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
