// Package oauth abstracts the OAuth 2.0 auth-code + PKCE flow behind a Provider
// interface so tests can swap Google for a deterministic fake.
package oauth

import "context"

// Claims is the subset of OpenID Connect ID-token claims Diarion needs.
type Claims struct {
	Sub           string // OpenID subject (Google's unique user ID)
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// Provider runs the auth-code + PKCE flow against a specific OIDC issuer.
type Provider interface {
	// AuthCodeURL returns the URL the browser should be redirected to. The
	// caller supplies a state nonce (server-side CSRF) and a PKCE code
	// challenge (S256-encoded).
	AuthCodeURL(state, codeChallenge string) string

	// Exchange swaps an auth code for an ID token using the PKCE code verifier
	// and returns the verified Claims. Errors here always indicate that the
	// caller MUST refuse the login.
	Exchange(ctx context.Context, code, codeVerifier string) (*Claims, error)
}
