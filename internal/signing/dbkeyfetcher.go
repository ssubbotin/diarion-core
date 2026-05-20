package signing

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5"
)

// DBKeyFetcher resolves fingerprints against the agent_keys table.
type DBKeyFetcher struct {
	q dbgen.Querier
}

// NewDBKeyFetcher constructs a DBKeyFetcher.
func NewDBKeyFetcher(q dbgen.Querier) *DBKeyFetcher {
	return &DBKeyFetcher{q: q}
}

// Fetch implements signing.KeyFetcher.
func (f *DBKeyFetcher) Fetch(ctx context.Context, fingerprint string) (*AgentKeyRecord, error) {
	row, err := f.q.GetAgentKeyByFingerprint(ctx, fingerprint)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("signing: load key: %w", err)
	}
	if row.Status != "active" {
		return nil, ErrKeyRevoked
	}
	if len(row.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("signing: stored public_key length %d", len(row.PublicKey))
	}
	return &AgentKeyRecord{
		Fingerprint: row.Fingerprint,
		PublicKey:   ed25519.PublicKey(row.PublicKey),
		AgentID:     row.AgentID,
		KeyID:       row.ID,
	}, nil
}
