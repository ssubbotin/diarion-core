// Package config loads Diarion's runtime configuration from environment variables.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// Config holds all runtime knobs sourced from env vars.
type Config struct {
	APIListen               string
	BaseURL                 string
	DatabaseURL             string
	RedisURL                string
	SessionSecret           []byte
	DiarionMasterKey        []byte
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
}

// Load reads configuration from the process environment.
func Load() (*Config, error) {
	cfg := &Config{
		APIListen:               envOr("API_LISTEN", ":8080"),
		BaseURL:                 os.Getenv("BASE_URL"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		RedisURL:                os.Getenv("REDIS_URL"),
		GoogleOAuthClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
	}

	for _, r := range []struct {
		name, val string
	}{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"REDIS_URL", cfg.RedisURL},
		{"BASE_URL", cfg.BaseURL},
		{"GOOGLE_OAUTH_CLIENT_ID", cfg.GoogleOAuthClientID},
		{"GOOGLE_OAUTH_CLIENT_SECRET", cfg.GoogleOAuthClientSecret},
	} {
		if r.val == "" {
			return nil, errors.New(r.name + " is required")
		}
	}

	var err error
	cfg.SessionSecret, err = decodeKey("SESSION_SECRET", 32)
	if err != nil {
		return nil, err
	}
	cfg.DiarionMasterKey, err = decodeKey("DIARION_MASTER_KEY", 32)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func decodeKey(envName string, wantBytes int) ([]byte, error) {
	raw := os.Getenv(envName)
	if raw == "" {
		return nil, fmt.Errorf("%s is required", envName)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be hex-encoded: %w", envName, err)
	}
	if len(decoded) != wantBytes {
		return nil, fmt.Errorf("%s must decode to %d bytes; got %d", envName, wantBytes, len(decoded))
	}
	return decoded, nil
}
