package redisx

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// The leaderboard read path (ADR-0012). Standings live durably in
// PostgreSQL (leaderboard_entries); this is a rebuildable Redis cache for
// the hot REST read. A sorted set orders members by an encoded score; a
// companion hash holds the display fields. A cache miss (cold key, or Redis
// flushed) falls back to PG and warms the cache — losing Redis loses no data
// (ADR-0004).
const (
	lbTTL = 6 * time.Hour

	// solvedWeight ranks "more solved" strictly above "fewer solved": with
	// ZREVRANGE (high→low) a solve outweighs any realistic penalty, and for
	// equal solves a smaller penalty yields a higher score (penalty asc).
	// 1e12 dwarfs any penalty-in-seconds (hours-long contests, ~1e4–1e6) and
	// stays within float64's exact-integer range for dozens of problems.
	solvedWeight = 1e12
)

// LeaderboardMember is one ranked standing (redisx's own type so this
// package stays free of the db layer).
type LeaderboardMember struct {
	UserID   uuid.UUID
	Username string
	Solved   int
	PenaltyS int64
}

type lbMeta struct {
	U string `json:"u"`
	S int    `json:"s"`
	P int64  `json:"p"`
}

func leaderboardKey(contestID uuid.UUID) string { return "arena:lb:" + contestID.String() }
func leaderboardMetaKey(contestID uuid.UUID) string {
	return "arena:lbmeta:" + contestID.String()
}

func rankScore(solved int, penaltyS int64) float64 {
	return float64(solved)*solvedWeight - float64(penaltyS)
}

// SetLeaderboardEntry write-through-updates one member (called on a first
// solve). It is a no-op-safe upsert: re-running with the same values is
// harmless (idempotent under at-least-once judging).
func (c *Client) SetLeaderboardEntry(ctx context.Context, contestID uuid.UUID, m LeaderboardMember) error {
	meta, err := json.Marshal(lbMeta{U: m.Username, S: m.Solved, P: m.PenaltyS})
	if err != nil {
		return fmt.Errorf("redisx: marshal leaderboard meta: %w", err)
	}
	key, metaKey := leaderboardKey(contestID), leaderboardMetaKey(contestID)

	pipe := c.rdb.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: rankScore(m.Solved, m.PenaltyS), Member: m.UserID.String()})
	pipe.HSet(ctx, metaKey, m.UserID.String(), meta)
	pipe.Expire(ctx, key, lbTTL)
	pipe.Expire(ctx, metaKey, lbTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redisx: set leaderboard entry: %w", err)
	}
	return nil
}

// TopLeaderboard returns up to limit ranked members. warm is false when the
// cache is cold (empty key) — the caller should read PG and RebuildLeaderboard.
func (c *Client) TopLeaderboard(ctx context.Context, contestID uuid.UUID, limit int) (members []LeaderboardMember, warm bool, err error) {
	key := leaderboardKey(contestID)
	z, err := c.rdb.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, false, fmt.Errorf("redisx: read leaderboard: %w", err)
	}
	if len(z) == 0 {
		return nil, false, nil // cold cache (or genuinely empty — caller confirms via PG)
	}

	ids := make([]string, len(z))
	for i, member := range z {
		s, _ := member.Member.(string)
		ids[i] = s
	}
	metas, err := c.rdb.HMGet(ctx, leaderboardMetaKey(contestID), ids...).Result()
	if err != nil {
		return nil, false, fmt.Errorf("redisx: read leaderboard meta: %w", err)
	}

	out := make([]LeaderboardMember, 0, len(z))
	for i := range z {
		raw, ok := metas[i].(string)
		if !ok {
			continue // meta evicted out from under the zset; treat as miss-ish
		}
		var m lbMeta
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		id, _ := uuid.Parse(ids[i])
		out = append(out, LeaderboardMember{UserID: id, Username: m.U, Solved: m.S, PenaltyS: m.P})
	}

	// ZREVRANGE breaks score ties in reverse-lexicographic member order;
	// the durable SQL ordering tie-breaks by username ascending. Re-sort to
	// match exactly so the cache and the source of truth never disagree.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Solved != out[j].Solved {
			return out[i].Solved > out[j].Solved
		}
		if out[i].PenaltyS != out[j].PenaltyS {
			return out[i].PenaltyS < out[j].PenaltyS
		}
		return out[i].Username < out[j].Username
	})
	return out, true, nil
}

// RebuildLeaderboard repopulates the cache from the durable standings (used
// to warm a cold key after a PG fallback). Replaces any stale contents.
func (c *Client) RebuildLeaderboard(ctx context.Context, contestID uuid.UUID, members []LeaderboardMember) error {
	key, metaKey := leaderboardKey(contestID), leaderboardMetaKey(contestID)

	pipe := c.rdb.TxPipeline()
	pipe.Del(ctx, key, metaKey)
	for _, m := range members {
		meta, err := json.Marshal(lbMeta{U: m.Username, S: m.Solved, P: m.PenaltyS})
		if err != nil {
			return fmt.Errorf("redisx: marshal leaderboard meta: %w", err)
		}
		pipe.ZAdd(ctx, key, redis.Z{Score: rankScore(m.Solved, m.PenaltyS), Member: m.UserID.String()})
		pipe.HSet(ctx, metaKey, m.UserID.String(), meta)
	}
	pipe.Expire(ctx, key, lbTTL)
	pipe.Expire(ctx, metaKey, lbTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redisx: rebuild leaderboard: %w", err)
	}
	return nil
}
