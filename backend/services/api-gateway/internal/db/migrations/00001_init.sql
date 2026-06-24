-- +goose Up

-- citext gives case-insensitive uniqueness for usernames and emails without
-- lower() shenanigans in every query.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      citext NOT NULL UNIQUE CHECK (length(username) BETWEEN 3 AND 32),
    email         citext NOT NULL UNIQUE CHECK (position('@' IN email) > 1 AND length(email) <= 254),
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Refresh tokens are stored hashed (SHA-256); a stolen database dump must
-- not yield usable tokens. replaced_by records rotation chains so reuse of
-- a rotated token can be detected and the whole family revoked.
CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  bytea NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz,
    replaced_by uuid REFERENCES refresh_tokens (id)
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

CREATE TABLE contests (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{2,63}$'),
    title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '',
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE TABLE contest_participants (
    contest_id uuid NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contest_id, user_id)
);

CREATE TABLE problems (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id      uuid NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    ord             int NOT NULL CHECK (ord >= 1),
    title           text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    statement_md    text NOT NULL,
    time_limit_ms   int NOT NULL DEFAULT 2000 CHECK (time_limit_ms BETWEEN 100 AND 10000),
    memory_limit_mb int NOT NULL DEFAULT 128 CHECK (memory_limit_mb BETWEEN 16 AND 512),
    UNIQUE (contest_id, ord)
);

CREATE TABLE test_cases (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id      uuid NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    ord             int NOT NULL CHECK (ord >= 1),
    stdin           text NOT NULL,
    expected_output text NOT NULL,
    UNIQUE (problem_id, ord)
);

CREATE TABLE submissions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    problem_id   uuid NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    -- Denormalized from problems for cheap contest-wide queries.
    contest_id   uuid NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    language     text NOT NULL CHECK (language IN ('cpp', 'python', 'go')),
    code         text NOT NULL CHECK (length(code) <= 262144),
    status       text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done')),
    verdict      text CHECK (verdict IN ('accepted', 'wrong_answer', 'runtime_error',
                                         'compilation_error', 'time_limit_exceeded',
                                         'memory_limit_exceeded', 'internal_error')),
    time_used_ms int,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    judged_at    timestamptz,
    CHECK (status <> 'done' OR verdict IS NOT NULL)
);

CREATE INDEX submissions_contest_recent_idx ON submissions (contest_id, submitted_at DESC);
CREATE INDEX submissions_user_idx ON submissions (user_id);
-- Startup re-enqueue scans only unjudged rows.
CREATE INDEX submissions_pending_idx ON submissions (status) WHERE status <> 'done';

-- Durable standings truth; the Redis ZSET is a rebuildable mirror of this
-- table (ADR-0004).
CREATE TABLE leaderboard_entries (
    contest_id uuid NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    solved     int NOT NULL DEFAULT 0 CHECK (solved >= 0),
    penalty_s  bigint NOT NULL DEFAULT 0 CHECK (penalty_s >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contest_id, user_id)
);

CREATE INDEX leaderboard_rank_idx ON leaderboard_entries (contest_id, solved DESC, penalty_s ASC);

-- Tracks which problems a user has already solved, so duplicate accepts
-- don't double-score. Also the source for wrong-attempt penalty counting.
CREATE TABLE problem_solves (
    contest_id uuid NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    problem_id uuid NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    solved_at  timestamptz NOT NULL,
    PRIMARY KEY (problem_id, user_id)
);

-- +goose Down
DROP TABLE problem_solves;
DROP TABLE leaderboard_entries;
DROP TABLE submissions;
DROP TABLE test_cases;
DROP TABLE problems;
DROP TABLE contest_participants;
DROP TABLE contests;
DROP TABLE refresh_tokens;
DROP TABLE users;
DROP EXTENSION citext;
