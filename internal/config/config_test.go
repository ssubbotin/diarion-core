package config

import (
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://diarion:diarion@localhost:5432/diarion?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("SESSION_SECRET", strings.Repeat("a", 64))
	t.Setenv("DIARION_MASTER_KEY", strings.Repeat("b", 64))
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "google-client-id-fixture")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "google-client-secret-fixture")
}

func TestLoad_RequiredFields(t *testing.T) {
	setBaseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseURL == "" {
		t.Errorf("DatabaseURL is empty")
	}
	if cfg.APIListen != ":8080" {
		t.Errorf("APIListen default not applied; got %q", cfg.APIListen)
	}
	if len(cfg.SessionSecret) != 32 {
		t.Errorf("SessionSecret should be 32 bytes; got %d", len(cfg.SessionSecret))
	}
	if len(cfg.DiarionMasterKey) != 32 {
		t.Errorf("DiarionMasterKey should be 32 bytes; got %d", len(cfg.DiarionMasterKey))
	}
	if cfg.GoogleOAuthClientID != "google-client-id-fixture" {
		t.Errorf("GoogleOAuthClientID mismatch: %q", cfg.GoogleOAuthClientID)
	}
	if cfg.GoogleOAuthClientSecret != "google-client-secret-fixture" {
		t.Errorf("GoogleOAuthClientSecret mismatch: %q", cfg.GoogleOAuthClientSecret)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL; got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error should mention DATABASE_URL; got %v", err)
	}
}

func TestLoad_MissingGoogleClientID(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing GOOGLE_OAUTH_CLIENT_ID")
	}
	if !strings.Contains(err.Error(), "GOOGLE_OAUTH_CLIENT_ID") {
		t.Errorf("error should mention GOOGLE_OAUTH_CLIENT_ID; got %v", err)
	}
}

func TestLoad_BadSessionSecret(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("SESSION_SECRET", "not-hex")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed SESSION_SECRET")
	}
}
