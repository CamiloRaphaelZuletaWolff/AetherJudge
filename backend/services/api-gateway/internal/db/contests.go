package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const contestColumns = "id, slug, title, description, starts_at, ends_at, created_at"

func scanContest(row pgx.Row) (Contest, error) {
	var c Contest
	err := row.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.StartsAt, &c.EndsAt, &c.CreatedAt)
	return c, err
}

// ContestFilter selects which contests ListContests returns.
type ContestFilter string

// Valid contest filters.
const (
	ContestsActive   ContestFilter = "active"
	ContestsUpcoming ContestFilter = "upcoming"
	ContestsPast     ContestFilter = "past"
	ContestsAll      ContestFilter = "all"
)

// ListContests returns contests matching the filter relative to now,
// soonest-relevant first.
func (s *Store) ListContests(ctx context.Context, filter ContestFilter, now time.Time) ([]Contest, error) {
	var where, order string
	switch filter {
	case ContestsActive:
		where, order = "starts_at <= $1 AND ends_at > $1", "ends_at ASC"
	case ContestsUpcoming:
		where, order = "starts_at > $1", "starts_at ASC"
	case ContestsPast:
		where, order = "ends_at <= $1", "ends_at DESC"
	case ContestsAll:
		where, order = "$1 = $1", "starts_at DESC"
	default:
		return nil, fmt.Errorf("db: unknown contest filter %q", filter)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT `+contestColumns+` FROM contests WHERE `+where+` ORDER BY `+order, now)
	if err != nil {
		return nil, fmt.Errorf("db: list contests: %w", err)
	}
	defer rows.Close()

	var out []Contest
	for rows.Next() {
		c, err := scanContest(rows)
		if err != nil {
			return nil, fmt.Errorf("db: scan contest: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list contests rows: %w", err)
	}
	return out, nil
}

// GetContest fetches one contest or ErrNotFound.
func (s *Store) GetContest(ctx context.Context, id uuid.UUID) (Contest, error) {
	c, err := scanContest(s.pool.QueryRow(ctx,
		`SELECT `+contestColumns+` FROM contests WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Contest{}, ErrNotFound
	}
	if err != nil {
		return Contest{}, fmt.Errorf("db: get contest: %w", err)
	}
	return c, nil
}

// CreateContest inserts a contest (used by seeding and tests).
func (s *Store) CreateContest(ctx context.Context, slug, title, description string, startsAt, endsAt time.Time) (Contest, error) {
	c, err := scanContest(s.pool.QueryRow(ctx,
		`INSERT INTO contests (slug, title, description, starts_at, ends_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title,
		     description = EXCLUDED.description,
		     starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at
		 RETURNING `+contestColumns,
		slug, title, description, startsAt, endsAt))
	if err != nil {
		return Contest{}, fmt.Errorf("db: create contest: %w", err)
	}
	return c, nil
}

// JoinContest registers a participant; joining twice is a no-op. A missing
// contest or user maps to ErrNotFound.
func (s *Store) JoinContest(ctx context.Context, contestID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contest_participants (contest_id, user_id)
		 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		contestID, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key violation
			return ErrNotFound
		}
		return fmt.Errorf("db: join contest: %w", err)
	}
	return nil
}

// ListProblems returns a contest's problems ordered by ord.
func (s *Store) ListProblems(ctx context.Context, contestID uuid.UUID) ([]Problem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, contest_id, ord, title, statement_md, time_limit_ms, memory_limit_mb
		 FROM problems WHERE contest_id = $1 ORDER BY ord`, contestID)
	if err != nil {
		return nil, fmt.Errorf("db: list problems: %w", err)
	}
	defer rows.Close()

	var out []Problem
	for rows.Next() {
		var p Problem
		if err := rows.Scan(&p.ID, &p.ContestID, &p.Ord, &p.Title, &p.StatementMD, &p.TimeLimitMs, &p.MemoryLimitMB); err != nil {
			return nil, fmt.Errorf("db: scan problem: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list problems rows: %w", err)
	}
	return out, nil
}

// GetProblemByOrd fetches a problem by its position within a contest.
func (s *Store) GetProblemByOrd(ctx context.Context, contestID uuid.UUID, ord int) (Problem, error) {
	var p Problem
	err := s.pool.QueryRow(ctx,
		`SELECT id, contest_id, ord, title, statement_md, time_limit_ms, memory_limit_mb
		 FROM problems WHERE contest_id = $1 AND ord = $2`, contestID, ord).
		Scan(&p.ID, &p.ContestID, &p.Ord, &p.Title, &p.StatementMD, &p.TimeLimitMs, &p.MemoryLimitMB)
	if errors.Is(err, pgx.ErrNoRows) {
		return Problem{}, ErrNotFound
	}
	if err != nil {
		return Problem{}, fmt.Errorf("db: get problem: %w", err)
	}
	return p, nil
}

// CreateProblem inserts a problem (seeding and tests).
func (s *Store) CreateProblem(ctx context.Context, p Problem) (Problem, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO problems (contest_id, ord, title, statement_md, time_limit_ms, memory_limit_mb)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (contest_id, ord) DO UPDATE SET title = EXCLUDED.title,
		     statement_md = EXCLUDED.statement_md,
		     time_limit_ms = EXCLUDED.time_limit_ms,
		     memory_limit_mb = EXCLUDED.memory_limit_mb
		 RETURNING id`,
		p.ContestID, p.Ord, p.Title, p.StatementMD, p.TimeLimitMs, p.MemoryLimitMB).Scan(&p.ID)
	if err != nil {
		return Problem{}, fmt.Errorf("db: create problem: %w", err)
	}
	return p, nil
}

// ListTestCases returns a problem's judge cases ordered by ord.
func (s *Store) ListTestCases(ctx context.Context, problemID uuid.UUID) ([]TestCase, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, problem_id, ord, stdin, expected_output
		 FROM test_cases WHERE problem_id = $1 ORDER BY ord`, problemID)
	if err != nil {
		return nil, fmt.Errorf("db: list test cases: %w", err)
	}
	defer rows.Close()

	var out []TestCase
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.ProblemID, &tc.Ord, &tc.Stdin, &tc.ExpectedOutput); err != nil {
			return nil, fmt.Errorf("db: scan test case: %w", err)
		}
		out = append(out, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list test cases rows: %w", err)
	}
	return out, nil
}

// CreateTestCase inserts a test case (seeding and tests).
func (s *Store) CreateTestCase(ctx context.Context, tc TestCase) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO test_cases (problem_id, ord, stdin, expected_output)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (problem_id, ord) DO UPDATE SET stdin = EXCLUDED.stdin,
		     expected_output = EXCLUDED.expected_output`,
		tc.ProblemID, tc.Ord, tc.Stdin, tc.ExpectedOutput); err != nil {
		return fmt.Errorf("db: create test case: %w", err)
	}
	return nil
}
