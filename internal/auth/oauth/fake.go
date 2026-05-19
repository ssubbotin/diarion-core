package oauth

import (
	"context"
	"errors"
	"net/url"
)

// FakeProvider is a deterministic Provider for tests.
//
//   - AuthCodeURL returns a placeholder URL.
//   - Exchange returns ScriptedClaims keyed by the auth code; if the code is
//     "fail", it returns ScriptedError.
type FakeProvider struct {
	ScriptedClaims map[string]*Claims
	ScriptedError  error
}

// AuthCodeURL returns a deterministic placeholder URL encoding state and codeChallenge.
func (p *FakeProvider) AuthCodeURL(state, codeChallenge string) string {
	v := url.Values{}
	v.Set("state", state)
	v.Set("code_challenge", codeChallenge)
	return "https://fake-oauth.example.test/auth?" + v.Encode()
}

// Exchange returns ScriptedClaims for the given code, or an error if code is "fail" or unknown.
func (p *FakeProvider) Exchange(_ context.Context, code, _ string) (*Claims, error) {
	if code == "fail" {
		if p.ScriptedError != nil {
			return nil, p.ScriptedError
		}
		return nil, errors.New("fake: exchange failed")
	}
	if c, ok := p.ScriptedClaims[code]; ok {
		return c, nil
	}
	return nil, errors.New("fake: no scripted claim for code " + code)
}
