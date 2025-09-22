package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func integrationAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set; skipping integration test")
	}
	c := redis.NewClient(&redis.Options{Addr: addr})
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unreachable: %v", err)
	}
	return addr
}

func TestNewRedisServer_RequiresAddr(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	if _, err := NewRedisServer(); err == nil {
		t.Fatal("expected error for empty REDIS_ADDR")
	}
}

func TestNewRedisServer_BadDB(t *testing.T) {
	t.Setenv("REDIS_ADDR", "127.0.0.1:1")
	t.Setenv("REDIS_DB", "not-an-int")
	if _, err := NewRedisServer(); err == nil {
		t.Fatal("expected error for invalid REDIS_DB")
	}
}

func TestKeyInOut(t *testing.T) {
	r := &RedisServer{prefix: "tst:"}
	if got := r.keyIn("foo"); got != "tst:foo" {
		t.Errorf("keyIn: %q", got)
	}
	if got := r.keyOut("tst:foo"); got != "foo" {
		t.Errorf("keyOut: %q", got)
	}
}
