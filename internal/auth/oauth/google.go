package oauth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

// GoogleProvider is the production Provider using Google's OIDC endpoints.
type GoogleProvider struct {
	cfg *oauth2.Config
}

// NewGoogleProvider wires Google's well-known endpoints.
// redirectURL must match what's registered in the Google Cloud OAuth client
// (e.g. https://api.diarion.app/auth/google/callback).
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// AuthCodeURL composes the URL the browser is redirected to.
func (p *GoogleProvider) AuthCodeURL(state, codeChallenge string) string {
	return p.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange swaps the auth code for an ID token, then verifies it.
func (p *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier string) (*Claims, error) {
	tok, err := p.cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("google: exchange: %w", err)
	}

	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, errors.New("google: no id_token in token response")
	}

	payload, err := idtoken.Validate(ctx, rawID, p.cfg.ClientID)
	if err != nil {
		return nil, fmt.Errorf("google: validate id_token: %w", err)
	}

	c := &Claims{
		Sub: payload.Subject,
	}
	if v, ok := payload.Claims["email"].(string); ok {
		c.Email = v
	}
	if v, ok := payload.Claims["email_verified"].(bool); ok {
		c.EmailVerified = v
	}
	if v, ok := payload.Claims["name"].(string); ok {
		c.Name = v
	}
	if v, ok := payload.Claims["picture"].(string); ok {
		c.Picture = v
	}
	if c.Sub == "" || c.Email == "" {
		return nil, errors.New("google: id_token missing sub or email")
	}

	return c, nil
}
