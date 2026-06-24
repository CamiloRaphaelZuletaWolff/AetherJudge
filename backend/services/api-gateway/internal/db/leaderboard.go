package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// wrongAttemptPenalty is the classic ICPC surcharge per failed attempt
// before the first accept of a problem.
const wrongAttemptPenalty = 20 * time.Minute

// ScoreResult reports what ApplyAccepted changed.
type ScoreResult struct {
	// FirstSolve is false when the user had already solved this problem;
	// nothing was scored in that case.
	FirstSolve bool
	// Entry is the user's standing after the update (also populated on
	// duplicate accepts, for callers that want current totals).
	Entry LeaderboardEntry
}

// ApplyAccepted scores an accepted submission transactionally: it records
// the solve (idempotently), counts prior wrong attempts for the penalty, and
// upserts the leaderboard entry. Penalty = seconds from contest start to the
// accept, plus 20 minutes per prior wrong attempt (compilation errors and
// internal errors don't count, per ICPC convention).
func (s *Store) ApplyAccepted(ctx context.Context, sub Submission, acceptedAt time.Time) (res ScoreResult, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("db: begin scoring tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("db: rollback scoring tx: %w", rbErr))
		}
	}()

	var startsAt time.Time
	if err := tx.QueryRow(ctx,
		`SELECT starts_at FROM contests WHERE id = $1`, sub.ContestID).Scan(&startsAt); err != nil {
		return ScoreResult{}, fmt.Errorf("db: load contest start: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO problem_solves (contest_id, problem_id, user_id, solved_at)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		sub.ContestID, sub.ProblemID, sub.UserID, acceptedAt)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("db: record solve: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Already solved earlier; standings unchanged.
		entry, err := getEntryTx(ctx, tx, sub.ContestID, sub.UserID)
		if err != nil {
			return ScoreResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ScoreResult{}, fmt.Errorf("db: commit scoring tx: %w", err)
		}
		return ScoreResult{FirstSolve: false, Entry: entry}, nil
	}

	var wrongAttempts int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM submissions
		 WHERE user_id = $1 AND problem_id = $2 AND id <> $3
		   AND verdict IS NOT NULL
		   AND verdict NOT IN ('accepted', 'compilation_error', 'internal_error')
		   AND submitted_at < $4`,
		sub.UserID, sub.ProblemID, sub.ID, sub.SubmittedAt).Scan(&wrongAttempts); err != nil {
		return ScoreResult{}, fmt.Errorf("db: count wrong attempts: %w", err)
	}

	solveTime := acceptedAt.Sub(startsAt)
	if solveTime < 0 {
		solveTime = 0
	}
	penaltyS := int64(solveTime.Seconds()) + int64(wrongAttempts)*int64(wrongAttemptPenalty.Seconds())

	entry := LeaderboardEntry{ContestID: sub.ContestID, UserID: sub.UserID}
	if err := tx.QueryRow(ctx,
		`INSERT INTO leaderboard_entries (contest_id, user_id, solved, penalty_s)
		 VALUES ($1, $2, 1, $3)
		 ON CONFLICT (contest_id, user_id) DO UPDATE
		   SET solved = leaderboard_entries.solved + 1,
		       penalty_s = leaderboard_entries.penalty_s + EXCLUDED.penalty_s,
		       updated_at = now()
		 RETURNING solved, penalty_s, updated_at`,
		sub.ContestID, sub.UserID, penaltyS).
		Scan(&entry.Solved, &entry.PenaltyS, &entry.UpdatedAt); err != nil {
		return ScoreResult{}, fmt.Errorf("db: upsert leaderboard entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ScoreResult{}, fmt.Errorf("db: commit scoring tx: %w", err)
	}
	return ScoreResult{FirstSolve: true, Entry: entry}, nil
}

// GetLeaderboard returns the top standings with usernames, ordered by
// solved desc, penalty asc. This is the durable read used by REST and by
// Redis ZSET rebuilds.
func (s *Store) GetLeaderboard(ctx context.Context, contestID uuid.UUID, limit int) ([]LeaderboardEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT le.contest_id, le.user_id, u.username, le.solved, le.penalty_s, le.updated_at
		 FROM leaderboard_entries le
		 JOIN users u ON u.id = le.user_id
		 WHERE le.contest_id = $1
		 ORDER BY le.solved DESC, le.penalty_s ASC, u.username ASC
		 LIMIT $2`,
		contestID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: get leaderboard: %w", err)
	}
	defer rows.Close()

	var out []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.ContestID, &e.UserID, &e.Username, &e.Solved, &e.PenaltyS, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("db: scan leaderboard entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: leaderboard rows: %w", err)
	}
	return out, nil
}

func getEntryTx(ctx context.Context, tx pgx.Tx, contestID, userID uuid.UUID) (LeaderboardEntry, error) {
	entry := LeaderboardEntry{ContestID: contestID, UserID: userID}
	err := tx.QueryRow(ctx,
		`SELECT solved, penalty_s, updated_at FROM leaderboard_entries
		 WHERE contest_id = $1 AND user_id = $2`,
		contestID, userID).Scan(&entry.Solved, &entry.PenaltyS, &entry.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Solved before but never scored — possible only mid-migration;
		// return an empty entry rather than failing the accept.
		return entry, nil
	}
	if err != nil {
		return LeaderboardEntry{}, fmt.Errorf("db: get leaderboard entry: %w", err)
	}
	return entry, nil
}
