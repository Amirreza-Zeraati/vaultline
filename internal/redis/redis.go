// Package redis owns the Redis client lifecycle. Other packages depend on this
// wrapper rather than importing go-redis directly.
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
)

// fixedWindowScript increments a counter and, only when it created the key,
// attaches the window TTL.
//
// Doing this in Lua matters for three reasons: it is atomic (INCR and PEXPIRE
// can't be split by a crash, which would otherwise leave a key with no TTL
// blocking that client forever), it is a single round-trip, and it works on any
// Redis version — unlike EXPIRE ... NX, which needs Redis 7.
var fixedWindowScript = goredis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// Nil is returned by the client when a key does not exist. Re-exported so
// callers can check for it without importing go-redis directly.
var Nil = goredis.Nil

// Client wraps the go-redis client so swapping libraries touches one file.
type Client struct {
	*goredis.Client
}

// New builds a Redis client. It does not ping; call Ping separately so the
// caller controls the timeout.
func New(cfg config.Redis) *Client {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb}
}

// Ping verifies connectivity, honoring the context deadline. Use in readiness
// checks.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Close releases the connection pool.
func (c *Client) Close() error {
	return c.Client.Close()
}

// FixedWindowIncr increments the counter at key and returns its new value,
// setting the expiry to window only on the first increment of each window.
// Used by the rate-limiting middleware.
func (c *Client) FixedWindowIncr(ctx context.Context, key string, window time.Duration) (int64, error) {
	return fixedWindowScript.Run(ctx, c.Client, []string{key}, window.Milliseconds()).Int64()
}
