package signing

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sfv "github.com/dunglas/httpsfv"
)

// Verifier checks RFC 9421 HTTP signatures over the Diarion signed-entry
// envelope. It is concurrency-safe — keep one instance per process.
type Verifier interface {
	Verify(r *http.Request, body []byte) (*VerifyResult, error)
}

// VerifyResult collects everything a handler needs after a successful verify:
// who signed (KeyFingerprint, AgentID, KeyID), the raw 64-byte ed25519
// signature ready for storage in entries.signature, the sha256 of the body,
// and the prev-entry hash the agent committed to.
type VerifyResult struct {
	KeyFingerprint string
	AgentID        int64
	KeyID          int64
	Signature      []byte // raw 64-byte ed25519 signature
	PrevEntryHash  []byte // 32 bytes
	ContentDigest  []byte // 32 bytes (sha256 of body)
}

// NewVerifier returns a Verifier backed by keys. keys must be non-nil.
func NewVerifier(keys KeyFetcher) Verifier {
	return &verifier{keys: keys, now: time.Now}
}

type verifier struct {
	keys KeyFetcher
	now  func() time.Time
}

// Verify checks the signature on r against body. The relationship between
// body and r.Body matters: callers are responsible for reading the body off
// the wire once (e.g. via io.ReadAll) and handing the exact same bytes here.
// We re-hash body and compare it against the Content-Digest header to be
// sure the bytes the handler will write to the DB match the bytes the
// signature covers.
func (v *verifier) Verify(r *http.Request, body []byte) (*VerifyResult, error) {
	if r == nil {
		return nil, fmt.Errorf("signing: nil request")
	}

	// 1. Recompute Content-Digest and compare against the header. Mismatch
	//    means either body was tampered with after signing or the signer
	//    sent a wrong digest — both are rejected before we touch the key
	//    fetcher.
	wantSum := sha256.Sum256(body)
	gotDigest, err := parseContentDigest(r.Header.Get("Content-Digest"))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadDigest, err)
	}
	if subtle.ConstantTimeCompare(wantSum[:], gotDigest) != 1 {
		return nil, ErrBadDigest
	}

	// 2. Parse Signature-Input and Signature headers. We extract the inner
	//    list, the `created` / `keyid` metadata, and the raw signature
	//    bytes.
	sigInputHeader := r.Header.Get("Signature-Input")
	sigHeader := r.Header.Get("Signature")
	if sigInputHeader == "" || sigHeader == "" {
		return nil, fmt.Errorf("%w: missing Signature/Signature-Input", ErrBadSignature)
	}
	innerList, created, keyid, err := parseSignatureInput(sigInputHeader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}
	rawSig, err := parseSignatureBytes(sigHeader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}
	if len(rawSig) != ed25519.SignatureSize {
		return nil, fmt.Errorf("%w: signature is %d bytes, want %d", ErrBadSignature, len(rawSig), ed25519.SignatureSize)
	}

	// 3. Confirm the inner list covers exactly the components we expect, in
	//    the right order. This stops a permissive verifier from accepting a
	//    signature that omits, say, content-digest or diarion-prev-entry-hash.
	if err := checkComponents(innerList); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}

	// 4. Time window. ±5 minutes either side of `now`. The dedicated
	//    sentinel lets handlers map to a specific 401 response.
	signedAt := time.Unix(created, 0)
	now := v.now()
	if delta := now.Sub(signedAt); delta > Window || delta < -Window {
		return nil, ErrStaleSignature
	}

	// 5. Compare keyid metadata to the Diarion-Key-Id header — they must
	//    agree, otherwise an attacker could swap the looked-up key.
	headerKeyID := r.Header.Get("Diarion-Key-Id")
	if headerKeyID == "" || !strings.EqualFold(headerKeyID, keyid) {
		return nil, fmt.Errorf("%w: Diarion-Key-Id (%q) does not match Signature-Input keyid (%q)", ErrBadSignature, headerKeyID, keyid)
	}

	// 6. Fetch the public key. Surface ErrKeyNotFound / ErrKeyRevoked
	//    directly — handlers want to map these to specific HTTP responses.
	rec, err := v.keys.Fetch(r.Context(), keyid)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) || errors.Is(err, ErrKeyRevoked) {
			return nil, err
		}
		return nil, fmt.Errorf("signing: key fetch: %w", err)
	}
	if rec == nil {
		return nil, ErrKeyNotFound
	}
	if rec.Fingerprint != keyid {
		return nil, fmt.Errorf("%w: fetcher returned mismatched fingerprint", ErrKeyNotFound)
	}
	if len(rec.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: stored public key is %d bytes, want %d", ErrBadSignature, len(rec.PublicKey), ed25519.PublicKeySize)
	}

	// 7. Verify the signature. The signature base must be reconstructed from
	//    the *exact* inner list bytes that came on the wire — we re-emit it
	//    through httpsfv to canonicalise once. componentValue pulls the same
	//    derived values the signer produced (method, authority, path, query
	//    and the three required headers).
	base, err := buildSignatureBase(r, innerList)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}
	if !ed25519.Verify(rec.PublicKey, []byte(base), rawSig) {
		return nil, fmt.Errorf("%w: ed25519 verify failed", ErrBadSignature)
	}

	// 8. Parse the prev-entry hash from its header.
	prevHex := r.Header.Get("Diarion-Prev-Entry-Hash")
	prev, err := hex.DecodeString(prevHex)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid Diarion-Prev-Entry-Hash: %w", ErrBadSignature, err)
	}
	if len(prev) != 32 {
		return nil, fmt.Errorf("%w: Diarion-Prev-Entry-Hash is %d bytes, want 32", ErrBadSignature, len(prev))
	}

	return &VerifyResult{
		KeyFingerprint: rec.Fingerprint,
		AgentID:        rec.AgentID,
		KeyID:          rec.KeyID,
		Signature:      rawSig,
		PrevEntryHash:  prev,
		ContentDigest:  wantSum[:],
	}, nil
}

// parseContentDigest pulls the sha-256 byte sequence out of a structured
// dictionary Content-Digest header. Anything else is a malformed header.
func parseContentDigest(header string) ([]byte, error) {
	if header == "" {
		return nil, fmt.Errorf("empty Content-Digest")
	}
	dict, err := sfv.UnmarshalDictionary([]string{header})
	if err != nil {
		return nil, fmt.Errorf("parse Content-Digest: %w", err)
	}
	member, ok := dict.Get("sha-256")
	if !ok {
		return nil, fmt.Errorf("Content-Digest missing sha-256")
	}
	item, ok := member.(sfv.Item)
	if !ok {
		return nil, fmt.Errorf("Content-Digest sha-256 is not an item")
	}
	bs, ok := item.Value.([]byte)
	if !ok {
		return nil, fmt.Errorf("Content-Digest sha-256 is not a byte sequence")
	}
	if len(bs) != 32 {
		return nil, fmt.Errorf("Content-Digest sha-256 is %d bytes, want 32", len(bs))
	}
	return bs, nil
}

// parseSignatureInput extracts the inner list, created timestamp and keyid
// metadata for the conventional "sig1" label.
func parseSignatureInput(header string) (sfv.InnerList, int64, string, error) {
	dict, err := sfv.UnmarshalDictionary([]string{header})
	if err != nil {
		return sfv.InnerList{}, 0, "", fmt.Errorf("parse Signature-Input: %w", err)
	}
	member, ok := dict.Get(signatureLabel)
	if !ok {
		return sfv.InnerList{}, 0, "", fmt.Errorf("Signature-Input missing label %q", signatureLabel)
	}
	list, ok := member.(sfv.InnerList)
	if !ok {
		return sfv.InnerList{}, 0, "", fmt.Errorf("Signature-Input %q is not an inner list", signatureLabel)
	}
	createdVal, ok := list.Params.Get("created")
	if !ok {
		return sfv.InnerList{}, 0, "", fmt.Errorf("Signature-Input missing `created`")
	}
	created, ok := createdVal.(int64)
	if !ok {
		return sfv.InnerList{}, 0, "", fmt.Errorf("Signature-Input `created` is not an integer")
	}
	keyidVal, ok := list.Params.Get("keyid")
	if !ok {
		return sfv.InnerList{}, 0, "", fmt.Errorf("Signature-Input missing `keyid`")
	}
	keyid, ok := keyidVal.(string)
	if !ok {
		return sfv.InnerList{}, 0, "", fmt.Errorf("Signature-Input `keyid` is not a string")
	}
	return list, created, keyid, nil
}

// parseSignatureBytes pulls the raw ed25519 signature bytes out of the
// structured Signature dictionary header.
func parseSignatureBytes(header string) ([]byte, error) {
	dict, err := sfv.UnmarshalDictionary([]string{header})
	if err != nil {
		return nil, fmt.Errorf("parse Signature: %w", err)
	}
	member, ok := dict.Get(signatureLabel)
	if !ok {
		return nil, fmt.Errorf("missing label %q", signatureLabel)
	}
	item, ok := member.(sfv.Item)
	if !ok {
		return nil, fmt.Errorf("label %q is not an item", signatureLabel)
	}
	bs, ok := item.Value.([]byte)
	if !ok {
		return nil, fmt.Errorf("label %q is not a byte sequence", signatureLabel)
	}
	return bs, nil
}

// checkComponents verifies the inner list covers exactly Components in order.
// Anything else — a missing prev-entry-hash, an extra Date header, a reorder —
// is treated as a malformed signature.
func checkComponents(list sfv.InnerList) error {
	if len(list.Items) != len(Components) {
		return fmt.Errorf("expected %d components, got %d", len(Components), len(list.Items))
	}
	for i, want := range Components {
		got, ok := list.Items[i].Value.(string)
		if !ok {
			return fmt.Errorf("component %d is not a string", i)
		}
		if got != want {
			return fmt.Errorf("component %d = %q, want %q", i, got, want)
		}
	}
	return nil
}
