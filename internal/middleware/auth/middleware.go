// Package auth provides Chi middleware that resolves a session cookie OR a
// PAT bearer token to a user and attaches it to the request context.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/diarion/diarion-core/internal/auth/pat"
	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db/dbgen"
)

type ctxKey struct{}

// User is the authenticated principal exposed via context.
type User struct {
	*dbgen.User
	ViaPAT bool
}

// FromContext returns the authenticated user attached by Middleware, or nil if
// the request was unauthenticated.
func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(ctxKey{}).(*User)
	return u
}

// RequireUser returns the user or writes 401 to the response. The caller MUST
// return early after a 401.
func RequireUser(w http.ResponseWriter, r *http.Request) (*User, bool) {
	u := FromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return u, true
}

// Middleware resolves cookie OR Bearer token to a user and attaches to context.
// Unauthenticated requests pass through with no user attached.
func Middleware(q dbgen.Querier, sessions *session.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if u := resolveFromSession(ctx, q, sessions, r); u != nil {
				r = r.WithContext(context.WithValue(ctx, ctxKey{}, u))
				next.ServeHTTP(w, r)
				return
			}

			if u := resolveFromPAT(ctx, q, r); u != nil {
				r = r.WithContext(context.WithValue(ctx, ctxKey{}, u))
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func resolveFromSession(ctx context.Context, q dbgen.Querier, sessions *session.Manager, r *http.Request) *User {
	s, err := sessions.Lookup(ctx, r)
	if err != nil {
		if !errors.Is(err, session.ErrNoSession) {
			slog.WarnContext(ctx, "session lookup failed", "err", err)
		}
		return nil
	}
	user, err := q.GetUserByID(ctx, s.UserID)
	if err != nil {
		slog.WarnContext(ctx, "session user lookup failed", "err", err, "user_id", s.UserID)
		return nil
	}
	if user.SuspendedAt.Valid || user.DeletedAt.Valid {
		return nil
	}
	return &User{User: &user, ViaPAT: false}
}

func resolveFromPAT(ctx context.Context, q dbgen.Querier, r *http.Request) *User {
	plain, err := pat.FromRequest(r)
	if err != nil {
		return nil
	}
	hash := pat.Hash(plain)
	row, err := q.GetPATByHash(ctx, hash)
	if err != nil {
		return nil
	}
	user, err := q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return nil
	}
	if user.SuspendedAt.Valid || user.DeletedAt.Valid {
		return nil
	}
	_ = q.TouchPAT(ctx, row.ID)
	return &User{User: &user, ViaPAT: true}
}
