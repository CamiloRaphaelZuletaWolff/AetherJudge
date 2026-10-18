// Package redisx owns all Redis access for the gateway: live-leaderboard
// sorted sets, contest event pub/sub, and rate-limit counters. Everything
// here is rebuildable from PostgreSQL (ADR-0004) — losing Redis must never
// lose data.
package redisx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Client wraps the go-redis client with Arena's operations.
type Client struct {
	rdb *redis.Client
	log *slog.Logger
}

// Connect opens a Redis client and verifies connectivity. Commands emit
// OTel spans (a no-op when tracing is disabled).
//
// addr is either a bare host:port (plaintext, no auth — local dev, the Helm
// chart) or a redis:// / rediss:// URL, which may carry credentials, a DB
// number and TLS. The URL form lets a Render Blueprint wire a Key Value's
// `connectionString` straight into REDIS_ADDR.
func Connect(ctx context.Context, addr string, log *slog.Logger) (*Client, error) {
	opts, err := clientOptions(addr)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opts)
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, fmt.Errorf("redisx: instrument tracing: %w", err)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		closeErr := rdb.Close()
		if closeErr != nil {
			log.Warn("close redis client after failed ping", "error", closeErr)
		}
		return nil, fmt.Errorf("redisx: ping (is Redis running? task infra:up): %w", err)
	}
	return &Client{rdb: rdb, log: log}, nil
}

// Ping verifies Redis connectivity (readiness probes).
func (c *Client) Ping(ctx context.Context) error {
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redisx: ping: %w", err)
	}
	return nil
}

// Close releases the connection pool.
func (c *Client) Close() error {
	if err := c.rdb.Close(); err != nil {
		return fmt.Errorf("redisx: close: %w", err)
	}
	return nil
}

// clientOptions turns REDIS_ADDR into client options. URLs are parsed by
// go-redis (redis://[user:pass@]host:port[/db], rediss:// for TLS); anything
// else is treated as a bare host:port.
func clientOptions(addr string) (*redis.Options, error) {
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		opts, err := redis.ParseURL(addr)
		if err != nil {
			// Deliberately not wrapping err: it may echo the URL, password included.
			return nil, errors.New("redisx: REDIS_ADDR is not a valid redis:// or rediss:// URL")
		}
		return opts, nil
	}
	return &redis.Options{Addr: addr}, nil
}
