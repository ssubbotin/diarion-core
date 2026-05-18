package config

import (
	"strings"
	"testing"
)

func TestLoad_RequiredFields(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://diarion:diarion@localhost:5432/diarion?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("SESSION_SECRET", strings.Repeat("a", 64))
	t.Setenv("DIARION_MASTER_KEY", strings.Repeat("b", 64))

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
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("SESSION_SECRET", strings.Repeat("a", 64))
	t.Setenv("DIARION_MASTER_KEY", strings.Repeat("b", 64))
	// DATABASE_URL not set

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL; got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error should mention DATABASE_URL; got %v", err)
	}
}

func TestLoad_BadSessionSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x")
	t.Setenv("BASE_URL", "http://x")
	t.Setenv("SESSION_SECRET", "not-hex")
	t.Setenv("DIARION_MASTER_KEY", strings.Repeat("b", 64))

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed SESSION_SECRET")
	}
}
