package signing

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sfv "github.com/dunglas/httpsfv"
)

// Window is the symmetric tolerance the verifier applies to the `created`
// signature metadata parameter. Signatures whose `created` falls outside
// [now-Window, now+Window] are rejected with ErrStaleSignature.
const Window = 5 * time.Minute

// Components is the exact, ordered list of RFC 9421 message components that
// every Diarion signed entry covers. Both signer and verifier read this
// list — do not rebuild it inline.
var Components = []string{
	"@method",
	"@authority",
	"@path",
	"@query",
	"content-digest",
	"diarion-key-id",
	"diarion-prev-entry-hash",
}

// signatureLabel is the label under which the entry's signature is stored
// inside the structured Signature / Signature-Input headers. Diarion
// signs each entry exactly once so a single fixed label is enough.
const signatureLabel = "sig1"

// Sign signs req over body using priv. It sets the Content-Digest,
// Diarion-Key-Id, Diarion-Prev-Entry-Hash, Signature-Input, and Signature
// headers. `body` must be the byte-exact request body the verifier will read
// on the other side; req.Body is also rewound so subsequent transport code
// can re-read it.
//
// fingerprint is the hex sha256 of priv's public key — it goes into
// Diarion-Key-Id and the Signature-Input `keyid` metadata parameter so that
// the verifier can look the key up before checking the signature.
//
// prevEntryHash must be 32 bytes (zeros for the agent's first entry).
func Sign(req *http.Request, body []byte, fingerprint string, priv ed25519.PrivateKey, prevEntryHash []byte) error {
	return SignAt(req, body, fingerprint, priv, prevEntryHash, time.Now())
}

// SignAt is Sign with an explicit clock — useful for tests that need to
// produce signatures whose `created` parameter is far enough in the past or
// future to exercise the verifier's stale-signature path.
func SignAt(req *http.Request, body []byte, fingerprint string, priv ed25519.PrivateKey, prevEntryHash []byte, t time.Time) error {
	if req == nil {
		return fmt.Errorf("signing: nil request")
	}
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("signing: ed25519 private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(priv))
	}
	if len(prevEntryHash) != 32 {
		return fmt.Errorf("signing: prev-entry-hash must be 32 bytes, got %d", len(prevEntryHash))
	}
	if fingerprint == "" {
		return fmt.Errorf("signing: empty fingerprint")
	}

	// Content-Digest: sha-256=:<base64>: per RFC 9530.
	sum := sha256.Sum256(body)
	digestHeader, err := formatContentDigest(sum[:])
	if err != nil {
		return fmt.Errorf("signing: format Content-Digest: %w", err)
	}
	req.Header.Set("Content-Digest", digestHeader)
	req.Header.Set("Diarion-Key-Id", fingerprint)
	req.Header.Set("Diarion-Prev-Entry-Hash", hex.EncodeToString(prevEntryHash))

	// Restore req.Body so callers can still send the request — http.NewRequest
	// will have wrapped a bytes.Reader, but if the body was already consumed
	// (e.g. by a transport) we resurrect it from `body`.
	req.Body = io.NopCloser(strings.NewReader(string(body)))

	created := t.Unix()

	// Build the Signature-Input structured value:
	//   ("@method" "@authority" "@path" "@query" "content-digest"
	//    "diarion-key-id" "diarion-prev-entry-hash");created=<ts>;keyid="<fp>";alg="ed25519"
	innerList := buildSignatureInput(created, fingerprint)
	sigInputDict := sfv.NewDictionary()
	sigInputDict.Add(signatureLabel, innerList)
	sigInputHeader, err := sfv.Marshal(sigInputDict)
	if err != nil {
		return fmt.Errorf("signing: marshal Signature-Input: %w", err)
	}

	base, err := buildSignatureBase(req, innerList)
	if err != nil {
		return err
	}

	sig := ed25519.Sign(priv, []byte(base))

	sigDict := sfv.NewDictionary()
	sigDict.Add(signatureLabel, sfv.NewItem(sig))
	sigHeader, err := sfv.Marshal(sigDict)
	if err != nil {
		return fmt.Errorf("signing: marshal Signature: %w", err)
	}

	req.Header.Set("Signature-Input", sigInputHeader)
	req.Header.Set("Signature", sigHeader)
	return nil
}

// buildSignatureInput constructs the inner list (the signed component list
// plus signature metadata parameters) used both in the signature base and in
// the Signature-Input header. created is the Unix timestamp in seconds.
func buildSignatureInput(created int64, fingerprint string) sfv.InnerList {
	items := make([]sfv.Item, 0, len(Components))
	for _, name := range Components {
		items = append(items, sfv.NewItem(name))
	}
	list := sfv.InnerList{
		Items:  items,
		Params: sfv.NewParams(),
	}
	// Order matters in the structured-fields wire form, but the params map is
	// iterated in insertion order — these three appear after `created`
	// because that's the conventional ordering and matches what most
	// implementations produce. The verifier reads them by name, so the order
	// only affects the wire bytes and the signature base.
	list.Params.Add("created", created)
	list.Params.Add("keyid", fingerprint)
	list.Params.Add("alg", "ed25519")
	return list
}

// buildSignatureBase produces the RFC 9421 signature base — the ASCII string
// fed into ed25519.Sign / ed25519.Verify. Each covered component contributes
// one line of the form `"name": value\n`, then the trailing line is
// `"@signature-params": <inner-list>`.
func buildSignatureBase(req *http.Request, sigInput sfv.InnerList) (string, error) {
	var b strings.Builder
	for _, name := range Components {
		val, err := componentValue(req, name)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\"%s\": %s\n", name, val)
	}
	params, err := sfv.Marshal(sigInput)
	if err != nil {
		return "", fmt.Errorf("signing: marshal signature params: %w", err)
	}
	fmt.Fprintf(&b, "\"@signature-params\": %s", params)
	return b.String(), nil
}

// componentValue resolves a single signed component to its canonicalised
// value as defined in RFC 9421 §2. We only handle the seven components in
// Components — anything else is a programmer error.
func componentValue(req *http.Request, name string) (string, error) {
	switch name {
	case "@method":
		return strings.ToUpper(req.Method), nil
	case "@authority":
		return strings.ToLower(req.Host), nil
	case "@path":
		p := req.URL.Path
		if p == "" {
			p = "/"
		}
		return p, nil
	case "@query":
		// RFC 9421 §2.2.7: the canonical value is "?" followed by the raw
		// query string. Empty query → "?".
		return "?" + req.URL.RawQuery, nil
	case "content-digest", "diarion-key-id", "diarion-prev-entry-hash":
		// http.Header.Get canonicalises the key internally.
		v := req.Header.Get(name)
		if v == "" {
			return "", fmt.Errorf("signing: missing required header %q", name)
		}
		return v, nil
	default:
		return "", fmt.Errorf("signing: unsupported component %q", name)
	}
}

// formatContentDigest produces the RFC 9530 Content-Digest header value:
//
//	sha-256=:<base64-of-32-bytes>:
//
// Implemented via httpsfv to get correct binary-item encoding.
func formatContentDigest(digest []byte) (string, error) {
	d := sfv.NewDictionary()
	d.Add("sha-256", sfv.NewItem(digest))
	return sfv.Marshal(d)
}
