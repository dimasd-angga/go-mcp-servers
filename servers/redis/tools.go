package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/redis/go-redis/v9"
)

func (r *RedisServer) registerTools() {
	r.addGet()
	r.addSet()
	r.addDelete()
	r.addListKeys()
	r.addGetType()
	r.addLPush()
	r.addLPop()
	r.addLRange()
	r.addPublish()
	r.addHSet()
	r.addHGet()
	r.addHGetAll()
	r.addExpire()
	r.addTTL()
}

// ----- string ops -------------------------------------------------------

func (r *RedisServer) addGet() {
	r.mcp.AddTool(
		mcp.NewTool("get",
			mcp.WithDescription("GET a string value. Returns the raw string or an error if the key is missing."),
			mcp.WithString("key", mcp.Required(), mcp.Description("Key name (will be prefixed with REDIS_PREFIX)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			key, _ := req.GetArguments()["key"].(string)
			v, err := r.rdb.Get(ctx, r.keyIn(key)).Result()
			if errors.Is(err, redis.Nil) {
				return mcp.NewToolResultError("key not found"), nil
			}
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
			}
			return mcp.NewToolResultText(v), nil
		},
	)
}

func (r *RedisServer) addSet() {
	r.mcp.AddTool(
		mcp.NewTool("set",
			mcp.WithDescription("SET a string value, optionally with an expiry in seconds (ex>0)."),
			mcp.WithString("key", mcp.Required()),
			mcp.WithString("value", mcp.Required()),
			mcp.WithNumber("ex", mcp.Description("Expiry in seconds. Omit or 0 for no expiry.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			key, _ := args["key"].(string)
			value, _ := args["value"].(string)
			var ttl time.Duration
			switch ex := args["ex"].(type) {
			case float64:
				if ex > 0 {
					ttl = time.Duration(ex) * time.Second
				}
			case int:
				if ex > 0 {
					ttl = time.Duration(ex) * time.Second
				}
			}
			if err := r.rdb.Set(ctx, r.keyIn(key), value, ttl).Err(); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("set: %v", err)), nil
			}
			return mcp.NewToolResultText("OK"), nil
		},
	)
}

func (r *RedisServer) addDelete() {
	r.mcp.AddTool(
		mcp.NewTool("delete",
			mcp.WithDescription("DEL one or more keys. Returns the number of keys removed."),
			mcp.WithString("key", mcp.Required(), mcp.Description("Key to delete")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			key, _ := req.GetArguments()["key"].(string)
			n, err := r.rdb.Del(ctx, r.keyIn(key)).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("del: %v", err)), nil
			}
			return mcp.NewToolResultText(strconv.FormatInt(n, 10)), nil
		},
	)
}

func (r *RedisServer) addListKeys() {
	r.mcp.AddTool(
		mcp.NewTool("list_keys",
			mcp.WithDescription("List keys matching a pattern. Pattern is applied AFTER the REDIS_PREFIX. "+
				"Returns keys with the prefix stripped."),
			mcp.WithString("pattern", mcp.Description("Glob pattern; defaults to '*' (all keys in prefix).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pattern, _ := req.GetArguments()["pattern"].(string)
			if pattern == "" {
				pattern = "*"
			}
			full := r.keyIn(pattern)
			out := make([]string, 0)
			var cursor uint64
			for {
				keys, next, err := r.rdb.Scan(ctx, cursor, full, 200).Result()
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("scan: %v", err)), nil
				}
				for _, k := range keys {
					out = append(out, r.keyOut(k))
				}
				if next == 0 {
					break
				}
				cursor = next
				if len(out) >= 5000 {
					break
				}
			}
			body, _ := json.MarshalIndent(out, "", "  ")
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

func (r *RedisServer) addGetType() {
	r.mcp.AddTool(
		mcp.NewTool("get_type",
			mcp.WithDescription("Return the type (string, list, hash, set, zset, stream, none) of a key."),
			mcp.WithString("key", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			key, _ := req.GetArguments()["key"].(string)
			t, err := r.rdb.Type(ctx, r.keyIn(key)).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("type: %v", err)), nil
			}
			return mcp.NewToolResultText(t), nil
		},
	)
}

// ----- list ops --------------------------------------------------------

func (r *RedisServer) addLPush() {
	r.mcp.AddTool(
		mcp.NewTool("lpush",
			mcp.WithDescription("LPUSH a value onto the head of a list. Returns the new list length."),
			mcp.WithString("key", mcp.Required()),
			mcp.WithString("value", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			key, _ := args["key"].(string)
			value, _ := args["value"].(string)
			n, err := r.rdb.LPush(ctx, r.keyIn(key), value).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("lpush: %v", err)), nil
			}
			return mcp.NewToolResultText(strconv.FormatInt(n, 10)), nil
		},
	)
}

func (r *RedisServer) addLPop() {
	r.mcp.AddTool(
		mcp.NewTool("lpop",
			mcp.WithDescription("LPOP a value from the head of a list."),
			mcp.WithString("key", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			key, _ := req.GetArguments()["key"].(string)
			v, err := r.rdb.LPop(ctx, r.keyIn(key)).Result()
			if errors.Is(err, redis.Nil) {
				return mcp.NewToolResultError("list empty"), nil
			}
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("lpop: %v", err)), nil
			}
			return mcp.NewToolResultText(v), nil
		},
	)
}

func (r *RedisServer) addLRange() {
	r.mcp.AddTool(
		mcp.NewTool("lrange",
			mcp.WithDescription("LRANGE — return a slice of a list as a JSON array."),
			mcp.WithString("key", mcp.Required()),
			mcp.WithNumber("start", mcp.Description("Start index. Default 0.")),
			mcp.WithNumber("stop", mcp.Description("Stop index. Default -1 (end).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			key, _ := args["key"].(string)
			start := int64(0)
			stop := int64(-1)
			if v, ok := args["start"].(float64); ok {
				start = int64(v)
			}
			if v, ok := args["stop"].(float64); ok {
				stop = int64(v)
			}
			xs, err := r.rdb.LRange(ctx, r.keyIn(key), start, stop).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("lrange: %v", err)), nil
			}
			body, _ := json.MarshalIndent(xs, "", "  ")
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

// ----- pubsub ----------------------------------------------------------

func (r *RedisServer) addPublish() {
	r.mcp.AddTool(
		mcp.NewTool("publish",
			mcp.WithDescription("PUBLISH a message to a channel. Returns the number of subscribers reached."),
			mcp.WithString("channel", mcp.Required()),
			mcp.WithString("message", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			ch, _ := args["channel"].(string)
			msg, _ := args["message"].(string)
			n, err := r.rdb.Publish(ctx, r.keyIn(ch), msg).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("publish: %v", err)), nil
			}
			return mcp.NewToolResultText(strconv.FormatInt(n, 10)), nil
		},
	)
}

// ----- hash ops --------------------------------------------------------

func (r *RedisServer) addHSet() {
	r.mcp.AddTool(
		mcp.NewTool("hset",
			mcp.WithDescription("HSET a field on a hash. Returns 1 if the field was new, 0 if updated."),
			mcp.WithString("key", mcp.Required()),
			mcp.WithString("field", mcp.Required()),
			mcp.WithString("value", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			key, _ := args["key"].(string)
			field, _ := args["field"].(string)
			value, _ := args["value"].(string)
			n, err := r.rdb.HSet(ctx, r.keyIn(key), field, value).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("hset: %v", err)), nil
			}
			return mcp.NewToolResultText(strconv.FormatInt(n, 10)), nil
		},
	)
}

func (r *RedisServer) addHGet() {
	r.mcp.AddTool(
		mcp.NewTool("hget",
			mcp.WithDescription("HGET a field from a hash."),
			mcp.WithString("key", mcp.Required()),
			mcp.WithString("field", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			key, _ := args["key"].(string)
			field, _ := args["field"].(string)
			v, err := r.rdb.HGet(ctx, r.keyIn(key), field).Result()
			if errors.Is(err, redis.Nil) {
				return mcp.NewToolResultError("field not found"), nil
			}
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("hget: %v", err)), nil
			}
			return mcp.NewToolResultText(v), nil
		},
	)
}

func (r *RedisServer) addHGetAll() {
	r.mcp.AddTool(
		mcp.NewTool("hgetall",
			mcp.WithDescription("HGETALL — return a hash as JSON object."),
			mcp.WithString("key", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			key, _ := req.GetArguments()["key"].(string)
			m, err := r.rdb.HGetAll(ctx, r.keyIn(key)).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("hgetall: %v", err)), nil
			}
			body, _ := json.MarshalIndent(m, "", "  ")
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

// ----- ttl -------------------------------------------------------------

func (r *RedisServer) addExpire() {
	r.mcp.AddTool(
		mcp.NewTool("expire",
			mcp.WithDescription("Set a TTL in seconds on a key. Returns 1 if applied, 0 if the key is missing."),
			mcp.WithString("key", mcp.Required()),
			mcp.WithNumber("seconds", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			key, _ := args["key"].(string)
			sec, _ := args["seconds"].(float64)
			ok, err := r.rdb.Expire(ctx, r.keyIn(key), time.Duration(sec)*time.Second).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("expire: %v", err)), nil
			}
			if ok {
				return mcp.NewToolResultText("1"), nil
			}
			return mcp.NewToolResultText("0"), nil
		},
	)
}

func (r *RedisServer) addTTL() {
	r.mcp.AddTool(
		mcp.NewTool("ttl",
			mcp.WithDescription("TTL — return the remaining seconds on a key (-1 if no expiry, -2 if missing)."),
			mcp.WithString("key", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			key, _ := req.GetArguments()["key"].(string)
			d, err := r.rdb.TTL(ctx, r.keyIn(key)).Result()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("ttl: %v", err)), nil
			}
			return mcp.NewToolResultText(strconv.FormatInt(int64(d.Seconds()), 10)), nil
		},
	)
}
