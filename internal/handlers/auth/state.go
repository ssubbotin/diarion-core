package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// StateTTL is how long an OAuth state nonce remains valid.
const StateTTL = 10 * time.Minute

// statePayload is JSON-encoded into Redis under key oauth:state:<state>.
type statePayload struct {
	CodeVerifier string    `json:"code_verifier"`
	ReturnURL    string    `json:"return_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// stateStore is Redis-backed OAuth state.
type stateStore struct {
	r *redis.Client
}

func newStateStore(r *redis.Client) *stateStore {
	return &stateStore{r: r}
}

// Issue allocates a new state nonce and PKCE code verifier+challenge, stores
// the verifier in Redis, and returns the (state, challenge) pair the caller
// should forward to Google.
func (s *stateStore) Issue(ctx context.Context, returnURL string) (state, challenge string, err error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("oauth-state: rand state: %w", err)
	}
	state = hex.EncodeToString(stateBytes)

	verifierBytes := make([]byte, 48)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", fmt.Errorf("oauth-state: rand verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	challengeBytes := sha256.Sum256([]byte(codeVerifier))
	challenge = base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	payload, _ := json.Marshal(statePayload{
		CodeVerifier: codeVerifier,
		ReturnURL:    returnURL,
		CreatedAt:    time.Now(),
	})

	if err := s.r.Set(ctx, redisKey(state), payload, StateTTL).Err(); err != nil {
		return "", "", fmt.Errorf("oauth-state: redis set: %w", err)
	}
	return state, challenge, nil
}

// Consume looks up and atomically deletes the state, returning the stored
// code verifier and return-URL. Returns error if the state is unknown or
// expired.
func (s *stateStore) Consume(ctx context.Context, state string) (codeVerifier, returnURL string, err error) {
	key := redisKey(state)
	val, err := s.r.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", "", errors.New("oauth-state: unknown or expired state")
		}
		return "", "", fmt.Errorf("oauth-state: redis getdel: %w", err)
	}
	var p statePayload
	if err := json.Unmarshal(val, &p); err != nil {
		return "", "", fmt.Errorf("oauth-state: unmarshal: %w", err)
	}
	return p.CodeVerifier, p.ReturnURL, nil
}

func redisKey(state string) string { return "oauth:state:" + state }
