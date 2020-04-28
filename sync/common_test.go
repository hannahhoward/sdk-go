package sync

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	// Avoid collisions in Redis keys over test runs.
	rand.Seed(time.Now().UnixNano())

	// _ = os.Setenv("LOG_LEVEL", "debug")

	// Set fail-fast options for creating the client, capturing the default
	// state to restore it.
	prev := DefaultRedisOpts
	DefaultRedisOpts.PoolTimeout = 500 * time.Millisecond
	DefaultRedisOpts.MaxRetries = 0

	closeFn, err := ensureRedis()
	DefaultRedisOpts = prev
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	v := m.Run()

	_ = closeFn()
	os.Exit(v)
}

// Check if there's a running instance of redis, or start it otherwise. If we
// start an ad-hoc instance, the close function will terminate it.
func ensureRedis() (func() error, error) {
	// Try to obtain a client; if this fails, we'll attempt to start a redis
	// instance.
	client, err := redisClient(context.Background(), zap.S())
	if err == nil {
		_ = client.Close()
		return func() error { return nil }, err
	}

	cmd := exec.Command("redis-server", "-")
	if err := cmd.Start(); err != nil {
		return func() error { return nil }, fmt.Errorf("failed to start redis: %w", err)
	}

	time.Sleep(1 * time.Second)

	// Try to obtain a client again.
	if client, err = redisClient(context.Background(), zap.S()); err != nil {
		return func() error { return nil }, fmt.Errorf("failed to obtain redis client despite starting instance: %v", err)
	}
	defer client.Close()

	return func() error {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed while stopping test-scoped redis: %s", err)
		}
		return nil
	}, nil
}
