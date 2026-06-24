package db

import (
	"time"

	"github.com/google/uuid"
)

// User is an account row. PasswordHash never leaves the backend.
type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// RefreshToken is one member of a rotation chain (see internal/auth).
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  []byte
	ExpiresAt  time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
}

// Contest is a scheduled competition window.
type Contest struct {
	ID          uuid.UUID
	Slug        string
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	CreatedAt   time.Time
}

// Problem belongs to a contest; ord is its 1-based position.
type Problem struct {
	ID            uuid.UUID
	ContestID     uuid.UUID
	Ord           int
	Title         string
	StatementMD   string
	TimeLimitMs   int
	MemoryLimitMB int
}

// TestCase is one hidden judge case for a problem.
type TestCase struct {
	ID             uuid.UUID
	ProblemID      uuid.UUID
	Ord            int
	Stdin          string
	ExpectedOutput string
}

// Submission lifecycle: queued → running → done (verdict set).
type Submission struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ProblemID   uuid.UUID
	ContestID   uuid.UUID
	Language    string
	Code        string
	Status      string
	Verdict     *string
	TimeUsedMs  *int
	SubmittedAt time.Time
	JudgedAt    *time.Time
}

// LeaderboardEntry is a user's standing within a contest. Username is
// populated by reads that join users; zero elsewhere.
type LeaderboardEntry struct {
	ContestID uuid.UUID
	UserID    uuid.UUID
	Username  string
	Solved    int
	PenaltyS  int64
	UpdatedAt time.Time
}
