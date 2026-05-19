//go:build tools

// Package tools pins build-time and future-use dependencies so go mod tidy
// does not prune them before the importing packages are written.
package tools

import (
	_ "github.com/redis/go-redis/v9"
	_ "golang.org/x/oauth2"
	_ "golang.org/x/oauth2/google"
	_ "google.golang.org/api/idtoken"
)
