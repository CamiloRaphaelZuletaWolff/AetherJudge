package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const submissionColumns = "id, user_id, problem_id, contest_id, language, code, status, verdict, time_used_ms, submitted_at, judged_at"

func scanSubmission(row pgx.Row) (Submission, error) {
	var sub Submission
	err := row.Scan(&sub.ID, &sub.UserID, &sub.ProblemID, &sub.ContestID, &sub.Language, &sub.Code,
		&sub.Status, &sub.Verdict, &sub.TimeUsedMs, &sub.SubmittedAt, &sub.JudgedAt)
	return sub, err
}

// CreateSubmission inserts a queued submission.
func (s *Store) CreateSubmission(ctx context.Context, userID, problemID, contestID uuid.UUID, language, code string) (Submission, error) {
	sub, err := scanSubmission(s.pool.QueryRow(ctx,
		`INSERT INTO submissions (user_id, problem_id, contest_id, language, code)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+submissionColumns,
		userID, problemID, contestID, language, code))
	if err != nil {
		return Submission{}, fmt.Errorf("db: create submission: %w", err)
	}
	return sub, nil
}

// GetSubmission fetches one submission or ErrNotFound.
func (s *Store) GetSubmission(ctx context.Context, id uuid.UUID) (Submission, error) {
	sub, err := scanSubmission(s.pool.QueryRow(ctx,
		`SELECT `+submissionColumns+` FROM submissions WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("db: get submission: %w", err)
	}
	return sub, nil
}

// MarkSubmissionRunning transitions queued → running.
func (s *Store) MarkSubmissionRunning(ctx context.Context, id uuid.UUID) error {
	if _, err := s.pool.Exec(ctx,
		`UPDATE submissions SET status = 'running' WHERE id = $1`, id); err != nil {
		return fmt.Errorf("db: mark submission running: %w", err)
	}
	return nil
}

// FinishSubmission records the final verdict.
func (s *Store) FinishSubmission(ctx context.Context, id uuid.UUID, verdict string, timeUsedMs int) error {
	if _, err := s.pool.Exec(ctx,
		`UPDATE submissions SET status = 'done', verdict = $2, time_used_ms = $3, judged_at = now()
		 WHERE id = $1`, id, verdict, timeUsedMs); err != nil {
		return fmt.Errorf("db: finish submission: %w", err)
	}
	return nil
}

// ListPendingSubmissionIDs returns submissions not yet judged — the startup
// re-enqueue source after a crash (the in-process queue dies with the
// process; see docs/phases/phase-3-core-backend.md).
func (s *Store) ListPendingSubmissionIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM submissions WHERE status <> 'done' ORDER BY submitted_at`)
	if err != nil {
		return nil, fmt.Errorf("db: list pending submissions: %w", err)
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: scan pending submission id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: pending submissions rows: %w", err)
	}
	return out, nil
}

// ContestSubmission is a submission enriched with its author's username and
// problem ordinal — the moderator "all submissions" read (RBAC
// submission.viewAll; see ADR-0014).
type ContestSubmission struct {
	Submission
	Username   string
	ProblemOrd int
}

// ListContestSubmissions returns every submission in a contest, newest first,
// joined with the author's username and the problem ordinal. For moderators and
// admins only — enforced server-side by requirePermission.
func (s *Store) ListContestSubmissions(ctx context.Context, contestID uuid.UUID, limit int) ([]ContestSubmission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT s.id, s.user_id, s.problem_id, s.contest_id, s.language, s.code, s.status,
		        s.verdict, s.time_used_ms, s.submitted_at, s.judged_at, u.username, p.ord
		 FROM submissions s
		 JOIN users u ON u.id = s.user_id
		 JOIN problems p ON p.id = s.problem_id
		 WHERE s.contest_id = $1
		 ORDER BY s.submitted_at DESC LIMIT $2`,
		contestID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: list contest submissions: %w", err)
	}
	defer rows.Close()

	var out []ContestSubmission
	for rows.Next() {
		var cs ContestSubmission
		if err := rows.Scan(&cs.ID, &cs.UserID, &cs.ProblemID, &cs.ContestID, &cs.Language, &cs.Code,
			&cs.Status, &cs.Verdict, &cs.TimeUsedMs, &cs.SubmittedAt, &cs.JudgedAt,
			&cs.Username, &cs.ProblemOrd); err != nil {
			return nil, fmt.Errorf("db: scan contest submission: %w", err)
		}
		out = append(out, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: contest submissions rows: %w", err)
	}
	return out, nil
}

// ListUserContestSubmissions returns a user's submissions in a contest,
// newest first.
func (s *Store) ListUserContestSubmissions(ctx context.Context, contestID, userID uuid.UUID, limit int) ([]Submission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+submissionColumns+`
		 FROM submissions WHERE contest_id = $1 AND user_id = $2
		 ORDER BY submitted_at DESC LIMIT $3`,
		contestID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: list user submissions: %w", err)
	}
	defer rows.Close()

	var out []Submission
	for rows.Next() {
		sub, err := scanSubmission(rows)
		if err != nil {
			return nil, fmt.Errorf("db: scan submission: %w", err)
		}
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: user submissions rows: %w", err)
	}
	return out, nil
}
