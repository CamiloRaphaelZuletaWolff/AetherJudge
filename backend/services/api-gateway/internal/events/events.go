// Package events defines the JSON messages that flow over contest pub/sub
// channels and out to WebSocket clients. The judge publishes them; the
// realtime hub relays them verbatim — both sides share these types so the
// wire format has exactly one definition.
package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event type discriminators.
const (
	TypeSubmissionUpdate  = "submission.update"
	TypeLeaderboardUpdate = "leaderboard.update"
	TypeContestEvent      = "contest.event"
)

// Envelope is the outer wire format: a discriminator plus a typed payload.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// SubmissionUpdate reports a submission's lifecycle transitions to the room
// (public, like a contest status board).
type SubmissionUpdate struct {
	SubmissionID uuid.UUID `json:"submission_id"`
	Username     string    `json:"username"`
	ProblemOrd   int       `json:"problem_ord"`
	Language     string    `json:"language"`
	Status       string    `json:"status"`
	Verdict      string    `json:"verdict,omitempty"`
	TimeUsedMs   int       `json:"time_used_ms,omitempty"`
	SubmittedAt  time.Time `json:"submitted_at"`
}

// LeaderboardRow is one standing in a LeaderboardUpdate.
type LeaderboardRow struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Solved   int    `json:"solved"`
	PenaltyS int64  `json:"penalty_s"`
}

// LeaderboardUpdate carries the room's current top standings.
type LeaderboardUpdate struct {
	Entries []LeaderboardRow `json:"entries"`
}

// ContestEvent reports room-level happenings.
type ContestEvent struct {
	Kind     string `json:"kind"` // participant_joined
	Username string `json:"username,omitempty"`
}

// Marshal wraps a payload in an Envelope and encodes it.
func Marshal(eventType string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("events: marshal payload: %w", err)
	}
	data, err := json.Marshal(Envelope{Type: eventType, Payload: raw})
	if err != nil {
		return nil, fmt.Errorf("events: marshal envelope: %w", err)
	}
	return data, nil
}
