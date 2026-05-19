// Package redis owns the application's Redis client. Sessions don't use Redis
// (they're DB-backed for revocation), but OAuth state nonces and rate-limit
// counters do.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient parses redisURL and returns a connected, ping-verified client.
// Caller must Close on shutdown.
func NewClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis.NewClient: parse url: %w", err)
	}

	opts.PoolSize = 10
	opts.MinIdleConns = 1
	opts.ReadTimeout = 2 * time.Second
	opts.WriteTimeout = 2 * time.Second
	opts.DialTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis.NewClient: ping: %w", err)
	}

	return client, nil
}
