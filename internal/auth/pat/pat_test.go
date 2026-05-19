package pat_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diarion/diarion-core/internal/auth/pat"
)

func TestGenerate_ShapeAndUniqueness(t *testing.T) {
	t.Parallel()
	plain1, hash1, err := pat.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(plain1, pat.Prefix) {
		t.Errorf("plaintext should start with %q; got %q", pat.Prefix, plain1)
	}
	expectedLen := len(pat.Prefix) + 2*pat.TokenLen
	if len(plain1) != expectedLen {
		t.Errorf("plaintext length = %d, want %d", len(plain1), expectedLen)
	}
	if hash1 != pat.Hash(plain1) {
		t.Errorf("Hash mismatch")
	}

	plain2, _, _ := pat.Generate()
	if plain1 == plain2 {
		t.Errorf("two Generate() calls returned the same plaintext")
	}
}

func TestParseFromHeader(t *testing.T) {
	t.Parallel()
	plain, _, _ := pat.Generate()

	cases := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{"valid", "Bearer " + plain, false},
		{"empty", "", true},
		{"no bearer", plain, true},
		{"no prefix", "Bearer not-a-diarion-token", true},
		{"wrong length", "Bearer " + pat.Prefix + "0123", true},
		{"non-hex body", "Bearer " + pat.Prefix + strings.Repeat("z", 2*pat.TokenLen), true},
	}
	for _, tc := range cases {
		got, err := pat.ParseFromHeader(tc.header)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: wantErr=%v, gotErr=%v", tc.name, tc.wantErr, err)
		}
		if !tc.wantErr && got != plain {
			t.Errorf("%s: returned token mismatch", tc.name)
		}
	}
}

func TestFromRequest(t *testing.T) {
	t.Parallel()
	plain, _, _ := pat.Generate()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+plain)
	got, err := pat.FromRequest(r)
	if err != nil {
		t.Fatalf("FromRequest: %v", err)
	}
	if got != plain {
		t.Errorf("token mismatch")
	}
}
