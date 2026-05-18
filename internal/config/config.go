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
	APIListen        string
	BaseURL          string
	DatabaseURL      string
	RedisURL         string
	SessionSecret    []byte // 32 raw bytes, derived from hex-encoded SESSION_SECRET
	DiarionMasterKey []byte // 32 raw bytes, derived from hex-encoded DIARION_MASTER_KEY
}

// Load reads configuration from the process environment.
// Required env vars: DATABASE_URL, REDIS_URL, BASE_URL, SESSION_SECRET, DIARION_MASTER_KEY.
// Optional: API_LISTEN (default ":8080").
func Load() (*Config, error) {
	cfg := &Config{
		APIListen:   envOr("API_LISTEN", ":8080"),
		BaseURL:     os.Getenv("BASE_URL"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return nil, errors.New("REDIS_URL is required")
	}
	if cfg.BaseURL == "" {
		return nil, errors.New("BASE_URL is required")
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
