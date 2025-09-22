package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/redis/go-redis/v9"
)

// RedisServer wraps a *redis.Client and exposes key/value, list, hash, and
// pub/sub tools. All keys are silently prefixed with REDIS_PREFIX before
// being sent to Redis, and stripped from output so callers never see the
// namespace.
type RedisServer struct {
	mcp    *server.MCPServer
	rdb    *redis.Client
	prefix string
}

func (r *RedisServer) MCP() *server.MCPServer { return r.mcp }
func (r *RedisServer) Prefix() string         { return r.prefix }

// Close closes the underlying Redis client.
func (r *RedisServer) Close() error {
	if r.rdb != nil {
		return r.rdb.Close()
	}
	return nil
}

// NewRedisServer builds the server from environment configuration:
//
//	REDIS_ADDR     required, "host:port"
//	REDIS_PASSWORD optional
//	REDIS_DB       optional, default 0
//	REDIS_PREFIX   optional, namespace prepended to every key
func NewRedisServer() (*RedisServer, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR environment variable is required")
	}
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid REDIS_DB %q", v)
		}
		db = n
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	prefix := os.Getenv("REDIS_PREFIX")

	mcp := server.NewMCPServer("redis", "1.0.0", server.WithToolCapabilities(true))
	rs := &RedisServer{mcp: mcp, rdb: client, prefix: prefix}
	rs.registerTools()
	return rs, nil
}

// keyIn applies the namespace prefix before sending to Redis.
func (r *RedisServer) keyIn(k string) string { return r.prefix + k }

// keyOut strips the namespace prefix from a key returned by Redis.
func (r *RedisServer) keyOut(k string) string { return strings.TrimPrefix(k, r.prefix) }
