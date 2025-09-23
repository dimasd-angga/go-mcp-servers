package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dimasd-angga/go-mcp-servers/shared/testutil"
	"github.com/mark3labs/mcp-go/client"
)

func newRClient(t *testing.T) (*client.Client, *RedisServer) {
	t.Helper()
	addr := integrationAddr(t)
	t.Setenv("REDIS_ADDR", addr)
	t.Setenv("REDIS_PREFIX", "mcpt:")
	r, err := NewRedisServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// purge our namespace
		ctx := context.Background()
		var cursor uint64
		for {
			keys, next, err := r.rdb.Scan(ctx, cursor, r.prefix+"*", 200).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = r.rdb.Del(ctx, keys...).Err()
			}
			if next == 0 {
				break
			}
			cursor = next
		}
		_ = r.Close()
	})
	return testutil.NewInProcessClient(t, r.MCP()), r
}

func TestSetGetDelete(t *testing.T) {
	c, r := newRClient(t)
	testutil.CallTool(t, c, "set", map[string]any{"key": "greeting", "value": "hi"})
	got := testutil.CallTool(t, c, "get", map[string]any{"key": "greeting"})
	if got != "hi" {
		t.Errorf("want hi, got %q", got)
	}
	// confirm prefix applied server-side
	raw, _ := r.rdb.Get(context.Background(), "mcpt:greeting").Result()
	if raw != "hi" {
		t.Errorf("prefix not applied, raw=%q", raw)
	}
	del := testutil.CallTool(t, c, "delete", map[string]any{"key": "greeting"})
	if del != "1" {
		t.Errorf("delete count: %q", del)
	}
}

func TestGet_Missing(t *testing.T) {
	c, _ := newRClient(t)
	r := testutil.CallToolRaw(t, c, "get", map[string]any{"key": "nope"})
	if !r.IsError {
		t.Error("missing key should error")
	}
}

func TestSetWithEx(t *testing.T) {
	c, r := newRClient(t)
	testutil.CallTool(t, c, "set", map[string]any{"key": "ephemeral", "value": "boom", "ex": 60})
	ttl, err := r.rdb.TTL(context.Background(), "mcpt:ephemeral").Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl.Seconds() <= 0 || ttl.Seconds() > 60 {
		t.Errorf("ttl not applied: %v", ttl)
	}
}

func TestListKeys(t *testing.T) {
	c, _ := newRClient(t)
	testutil.CallTool(t, c, "set", map[string]any{"key": "a", "value": "1"})
	testutil.CallTool(t, c, "set", map[string]any{"key": "b", "value": "2"})
	out := testutil.CallTool(t, c, "list_keys", map[string]any{"pattern": "*"})
	var keys []string
	if err := json.Unmarshal([]byte(out), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) < 2 {
		t.Errorf("want >=2, got %v", keys)
	}
	for _, k := range keys {
		if strings.HasPrefix(k, "mcpt:") {
			t.Errorf("prefix not stripped: %q", k)
		}
	}
}

func TestGetType(t *testing.T) {
	c, _ := newRClient(t)
	testutil.CallTool(t, c, "set", map[string]any{"key": "k", "value": "v"})
	got := testutil.CallTool(t, c, "get_type", map[string]any{"key": "k"})
	if got != "string" {
		t.Errorf("want string, got %q", got)
	}
}

func TestListOps(t *testing.T) {
	c, _ := newRClient(t)
	testutil.CallTool(t, c, "lpush", map[string]any{"key": "q", "value": "first"})
	testutil.CallTool(t, c, "lpush", map[string]any{"key": "q", "value": "second"})
	rangeOut := testutil.CallTool(t, c, "lrange", map[string]any{"key": "q"})
	var xs []string
	_ = json.Unmarshal([]byte(rangeOut), &xs)
	if len(xs) != 2 || xs[0] != "second" || xs[1] != "first" {
		t.Errorf("lrange wrong: %v", xs)
	}
	popped := testutil.CallTool(t, c, "lpop", map[string]any{"key": "q"})
	if popped != "second" {
		t.Errorf("lpop wrong: %q", popped)
	}
}

func TestLPop_Empty(t *testing.T) {
	c, _ := newRClient(t)
	r := testutil.CallToolRaw(t, c, "lpop", map[string]any{"key": "empty"})
	if !r.IsError {
		t.Error("empty pop should error")
	}
}

func TestHashOps(t *testing.T) {
	c, _ := newRClient(t)
	testutil.CallTool(t, c, "hset", map[string]any{"key": "h", "field": "a", "value": "1"})
	testutil.CallTool(t, c, "hset", map[string]any{"key": "h", "field": "b", "value": "2"})
	got := testutil.CallTool(t, c, "hget", map[string]any{"key": "h", "field": "a"})
	if got != "1" {
		t.Errorf("hget a: %q", got)
	}
	all := testutil.CallTool(t, c, "hgetall", map[string]any{"key": "h"})
	var m map[string]string
	_ = json.Unmarshal([]byte(all), &m)
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("hgetall: %v", m)
	}
}

func TestHGet_Missing(t *testing.T) {
	c, _ := newRClient(t)
	r := testutil.CallToolRaw(t, c, "hget", map[string]any{"key": "no_hash", "field": "x"})
	if !r.IsError {
		t.Error("missing field should error")
	}
}

func TestExpireAndTTL(t *testing.T) {
	c, _ := newRClient(t)
	testutil.CallTool(t, c, "set", map[string]any{"key": "k", "value": "v"})
	res := testutil.CallTool(t, c, "expire", map[string]any{"key": "k", "seconds": 60})
	if res != "1" {
		t.Errorf("expire: %q", res)
	}
	ttl := testutil.CallTool(t, c, "ttl", map[string]any{"key": "k"})
	if ttl == "-1" || ttl == "-2" {
		t.Errorf("ttl not set: %q", ttl)
	}
}

func TestExpire_MissingKey(t *testing.T) {
	c, _ := newRClient(t)
	res := testutil.CallTool(t, c, "expire", map[string]any{"key": "nope", "seconds": 60})
	if res != "0" {
		t.Errorf("expire missing key: %q", res)
	}
}

func TestPublish(t *testing.T) {
	c, r := newRClient(t)
	// Subscribe before publishing.
	sub := r.rdb.Subscribe(context.Background(), r.keyIn("ch1"))
	defer sub.Close()
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := testutil.CallTool(t, c, "publish", map[string]any{"channel": "ch1", "message": "hello"})
	if got != "1" {
		t.Errorf("want 1 subscriber, got %q", got)
	}
}
