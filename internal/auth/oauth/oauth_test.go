package oauth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/diarion/diarion-core/internal/auth/oauth"
)

func TestFakeProvider_Exchange(t *testing.T) {
	t.Parallel()
	p := &oauth.FakeProvider{
		ScriptedClaims: map[string]*oauth.Claims{
			"ok-code": {
				Sub:           "google-sub-1",
				Email:         "alice@example.com",
				EmailVerified: true,
				Name:          "Alice",
			},
		},
	}

	claims, err := p.Exchange(context.Background(), "ok-code", "any-verifier")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if claims.Sub != "google-sub-1" || claims.Email != "alice@example.com" {
		t.Errorf("claims mismatch: %+v", claims)
	}

	if _, err := p.Exchange(context.Background(), "fail", "v"); err == nil {
		t.Errorf("expected error for 'fail' code")
	}
}

func TestGoogleProvider_AuthCodeURL_Shape(t *testing.T) {
	t.Parallel()
	p := oauth.NewGoogleProvider("client-id-1", "client-secret-1", "http://localhost:8080/auth/google/callback")
	u := p.AuthCodeURL("state-nonce-1", "code-challenge-1")
	for _, want := range []string{
		"accounts.google.com",
		"client_id=client-id-1",
		"state=state-nonce-1",
		"code_challenge=code-challenge-1",
		"code_challenge_method=S256",
		"scope=openid+email+profile",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("AuthCodeURL missing %q\nfull URL: %s", want, u)
		}
	}
}
