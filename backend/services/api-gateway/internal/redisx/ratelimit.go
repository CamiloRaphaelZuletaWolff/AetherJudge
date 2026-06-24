package redisx

import (
	"context"
	"fmt"
	"time"
)

// Allow implements a fixed-window rate limit: at most limit calls per window
// for the given key. Fixed windows allow up to 2x burst at window edges —
// acceptable for abuse protection and dramatically simpler than sliding
// windows; the tradeoff is documented in ADR-0007.
//
// Fail-closed: a Redis error denies the request (rate limits guard abuse
// paths; an attacker must not be able to bypass them by hurting Redis).
func (c *Client) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := "rl:" + key

	n, err := c.rdb.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("redisx: rate limit incr: %w", err)
	}
	if n == 1 {
		if err := c.rdb.Expire(ctx, redisKey, window).Err(); err != nil {
			return false, fmt.Errorf("redisx: rate limit expire: %w", err)
		}
	}
	return n <= int64(limit), nil
}
