//go:build integration

package redis_test

import (
	"context"
	"testing"
	"time"

	diarionredis "github.com/diarion/diarion-core/internal/store/redis"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startRedis(t *testing.T) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		cancel()
		t.Fatalf("start redis: %v", err)
	}

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379")
	url := "redis://" + host + ":" + port.Port()
	cleanup := func() {
		_ = c.Terminate(ctx)
		cancel()
	}
	return url, cleanup
}

func TestNewClient_Ping(t *testing.T) {
	t.Parallel()
	url, cleanup := startRedis(t)
	defer cleanup()
	ctx := context.Background()

	client, err := diarionredis.NewClient(ctx, url)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Basic round-trip
	if err := client.Set(ctx, "diarion:test", "value", 10*time.Second).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := client.Get(ctx, "diarion:test").Result()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "value" {
		t.Errorf("Get returned %q, want %q", got, "value")
	}
}
