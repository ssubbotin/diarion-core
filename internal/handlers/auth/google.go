// Package auth contains HTTP handlers for the OAuth login flow and session
// teardown.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/diarion/diarion-core/internal/auth/oauth"
	authsess "github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// Handlers carries the dependencies the auth handlers need.
type Handlers struct {
	Provider oauth.Provider
	Sessions *authsess.Manager
	Queries  dbgen.Querier
	State    *stateStore
}

// New constructs the Handlers.
func New(p oauth.Provider, sessions *authsess.Manager, q dbgen.Querier, r *redis.Client) *Handlers {
	return &Handlers{
		Provider: p,
		Sessions: sessions,
		Queries:  q,
		State:    newStateStore(r),
	}
}

// GoogleStart begins the OAuth flow by issuing a state nonce + PKCE challenge
// and redirecting to Google's auth endpoint.
//
// Optional ?return_url=/path query param is preserved through the round-trip.
func (h *Handlers) GoogleStart(w http.ResponseWriter, r *http.Request) {
	returnURL := r.URL.Query().Get("return_url")
	if !isSafeReturn(returnURL) {
		returnURL = "/"
	}

	state, challenge, err := h.State.Issue(r.Context(), returnURL)
	if err != nil {
		slog.ErrorContext(r.Context(), "oauth: issue state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	authURL := h.Provider.AuthCodeURL(state, challenge)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// GoogleCallback completes the OAuth flow: verifies state, exchanges code,
// upserts the user, and issues a session.
func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	if errParam := q.Get("error"); errParam != "" {
		slog.WarnContext(ctx, "oauth: provider error", "error", errParam, "description", q.Get("error_description"))
		http.Error(w, "oauth provider error: "+errParam, http.StatusBadRequest)
		return
	}

	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	verifier, returnURL, err := h.State.Consume(ctx, state)
	if err != nil {
		slog.WarnContext(ctx, "oauth: state consume", "err", err)
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	claims, err := h.Provider.Exchange(ctx, code, verifier)
	if err != nil {
		slog.ErrorContext(ctx, "oauth: exchange", "err", err)
		http.Error(w, "exchange failed", http.StatusBadGateway)
		return
	}

	user, err := h.upsertUser(ctx, claims)
	if err != nil {
		slog.ErrorContext(ctx, "oauth: upsert user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := h.Sessions.Issue(ctx, w, user.ID); err != nil {
		slog.ErrorContext(ctx, "oauth: issue session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !isSafeReturn(returnURL) {
		returnURL = "/"
	}
	http.Redirect(w, r, returnURL, http.StatusFound)
}

// upsertUser finds-or-creates a user keyed by Google's `sub` claim. The user's
// email/display name/avatar are refreshed from the claims on every login.
func (h *Handlers) upsertUser(ctx context.Context, c *oauth.Claims) (*dbgen.User, error) {
	existing, err := h.Queries.GetUserByGoogleSub(ctx, c.Sub)
	switch {
	case err == nil:
		display := c.Name
		if display == "" {
			display = existing.DisplayName
		}
		var avatar *string
		if c.Picture != "" {
			avatar = &c.Picture
		} else {
			avatar = existing.AvatarURL
		}
		updated, err := h.Queries.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{
			ID:          existing.ID,
			DisplayName: display,
			AvatarURL:   avatar,
		})
		if err != nil {
			return nil, err
		}
		return &updated, nil
	case errors.Is(err, pgx.ErrNoRows):
		var avatar *string
		if c.Picture != "" {
			avatar = &c.Picture
		}
		display := c.Name
		if display == "" {
			display = c.Email
		}
		created, err := h.Queries.InsertUser(ctx, dbgen.InsertUserParams{
			GoogleSub:     c.Sub,
			Email:         c.Email,
			EmailVerified: c.EmailVerified,
			DisplayName:   display,
			AvatarURL:     avatar,
		})
		if err != nil {
			return nil, err
		}
		return &created, nil
	default:
		return nil, err
	}
}

// Logout deletes the current session and clears the cookie.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Sessions.Revoke(r.Context(), w, r); err != nil {
		slog.ErrorContext(r.Context(), "logout: revoke", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll deletes every session for the current user.
func (h *Handlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := h.Sessions.RevokeAllForUser(r.Context(), w, user.ID); err != nil {
		slog.ErrorContext(r.Context(), "logout-all: revoke", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// isSafeReturn keeps an open-redirect by allowing only relative same-origin paths.
func isSafeReturn(u string) bool {
	if u == "" {
		return false
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return false
	}
	if len(u) == 0 || u[0] != '/' {
		return false
	}
	return true
}

// requireUser reads the authenticated user from the request context (set by
// internal/middleware/auth).
func requireUser(w http.ResponseWriter, r *http.Request) (*dbgen.User, bool) {
	u := authmw.FromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return u.User, true
}
