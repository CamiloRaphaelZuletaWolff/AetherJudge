# ADR-0012: Leaderboard read path (Redis sorted set)

- Status: accepted (2026-06-17)
- Phase: 7
- Related: ADR-0004 (PG truth / Redis rebuildable), ADR-0011 (scaling)

## Context

Standings live durably in PostgreSQL (`leaderboard_entries`, written
transactionally by `ApplyAccepted`). Through Phase 6 every leaderboard read
re-ran an indexed SQL join. The ZSET read-path optimization was deliberately
deferred to this phase (it is the "load" half of Phase 7). The constraint is
ADR-0004: anything in Redis must be rebuildable from PostgreSQL.

## Decision

Cache each contest's standings in a Redis **sorted set** `arena:lb:<contest>`
(member = user ID, score = `solved·1e12 − penaltyS`) plus a companion hash
holding display fields (username/solved/penalty). `ZREVRANGE` yields ranked
standings in O(log N + M).

- **Write-through** on a first solve (inside `score`, after `ApplyAccepted`):
  one `ZADD` + `HSET`. Best-effort — a cache error is logged, never fatal.
- **Read** (`GET …/leaderboard`) serves from the ZSET; a cold key or any
  cache error falls back to the SQL query and warms the cache from that
  authoritative read.
- **Rebuildable**: a flush or eviction costs one SQL read to repopulate. No
  standing exists only in Redis (ADR-0004 preserved).

Score-tie handling: `ZREVRANGE` breaks equal-score ties in reverse-member
order, whereas the durable SQL ordering tie-breaks by username ascending. The
read re-sorts equal-score groups by username so the cache and the source of
truth never disagree (tested: `TestLeaderboardCacheMatchesDB`).

### Rejected alternatives

- **Cache-aside with invalidate-on-write** (DEL the key on each solve): far
  simpler, but every solve forces the next read to rebuild from SQL — under a
  busy contest that is "SQL on nearly every read", defeating the cache.
- **Score encodes the username tie-break**: impossible to pack two ordering
  dimensions plus a string tie-break into one float; the Go re-sort is exact
  and cheap on the top-N slice.

## Consequences

- The hot REST leaderboard read avoids the SQL join when warm, while the WS
  broadcast (first-solve only) keeps using the durable query — both render
  identical orderings.
- A small write-through cost per first solve (one pipelined `ZADD`+`HSET`).
- One more Redis role, rebuildable like all the others (ADR-0004); a 6h TTL
  bounds stale-contest memory.
