// Package session manages server-side session creation, cookie marshalling,
// and lookup against the sessions table.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// CookieName is the HTTP cookie the server uses for sessions.
	CookieName = "diarion_session"
	// TTL is the default session lifetime.
	TTL = 30 * 24 * time.Hour
)

// ErrNoSession is returned when no valid session is found.
var ErrNoSession = errors.New("no session")

// Manager issues, looks up, and revokes sessions. It wraps the sqlc Queries
// type so handlers can mock it via Querier in tests.
type Manager struct {
	q      dbgen.Querier
	secure bool // set Secure cookie attribute (true in production)
}

// NewManager constructs a Manager. `secure` should be true iff the deployment
// serves HTTPS only.
func NewManager(q dbgen.Querier, secure bool) *Manager {
	return &Manager{q: q, secure: secure}
}

// Issue creates a session row and returns the cookie to set on the response.
func (m *Manager) Issue(ctx context.Context, w http.ResponseWriter, userID int64) (*dbgen.Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("session.Issue: token: %w", err)
	}

	expires := time.Now().Add(TTL)
	s, err := m.q.InsertSession(ctx, dbgen.InsertSessionParams{
		SessionToken: token,
		UserID:       userID,
		ExpiresAt:    pgtype.Timestamptz{Time: expires, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("session.Issue: insert: %w", err)
	}

	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configurable by design
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(TTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return &s, nil
}

// Lookup resolves the request's session cookie to a Session row, or returns
// ErrNoSession if there's no cookie / expired / revoked.
func (m *Manager) Lookup(ctx context.Context, r *http.Request) (*dbgen.Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil, ErrNoSession
	}
	token := strings.TrimSpace(c.Value)
	if token == "" {
		return nil, ErrNoSession
	}

	s, err := m.q.GetSessionByToken(ctx, token)
	if err != nil {
		return nil, ErrNoSession
	}

	// Touch is best-effort — failure shouldn't block the request.
	_ = m.q.TouchSession(ctx, s.ID)

	return &s, nil
}

// Revoke deletes the request's session (logout) and clears the cookie.
func (m *Manager) Revoke(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	s, err := m.Lookup(ctx, r)
	if err != nil {
		// Even if there's no session, clear the cookie for hygiene.
		m.clearCookie(w)
		return nil
	}
	if err := m.q.DeleteSession(ctx, s.ID); err != nil {
		return fmt.Errorf("session.Revoke: delete: %w", err)
	}
	m.clearCookie(w)
	return nil
}

// RevokeAllForUser deletes every active session for the given user.
func (m *Manager) RevokeAllForUser(ctx context.Context, w http.ResponseWriter, userID int64) error {
	if err := m.q.DeleteAllSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("session.RevokeAllForUser: %w", err)
	}
	m.clearCookie(w)
	return nil
}

func (m *Manager) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configurable by design
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
