package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/events"
	"github.com/caezu/arena/backend/services/api-gateway/internal/realtime"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

const leaderboardLimit = 50

type contestDTO struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
}

func toContestDTO(c db.Contest) contestDTO {
	return contestDTO{
		ID: c.ID.String(), Slug: c.Slug, Title: c.Title, Description: c.Description,
		StartsAt: c.StartsAt, EndsAt: c.EndsAt,
	}
}

type problemSummaryDTO struct {
	Ord   int    `json:"ord"`
	Title string `json:"title"`
}

type problemDTO struct {
	Ord           int    `json:"ord"`
	Title         string `json:"title"`
	StatementMD   string `json:"statement_md"`
	TimeLimitMs   int    `json:"time_limit_ms"`
	MemoryLimitMB int    `json:"memory_limit_mb"`
}

func (s *server) listContests(w http.ResponseWriter, r *http.Request) {
	filter := db.ContestFilter(r.URL.Query().Get("filter"))
	if filter == "" {
		filter = db.ContestsAll
	}
	switch filter {
	case db.ContestsActive, db.ContestsUpcoming, db.ContestsPast, db.ContestsAll:
	default:
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "filter must be active, upcoming, past, or all")
		return
	}

	contests, err := s.store.ListContests(r.Context(), filter, time.Now())
	if err != nil {
		s.log.Error("list contests", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list contests")
		return
	}

	out := make([]contestDTO, len(contests))
	for i, c := range contests {
		out[i] = toContestDTO(c)
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"contests": out})
}

func (s *server) getContest(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}

	problems, err := s.store.ListProblems(r.Context(), contest.ID)
	if err != nil {
		s.log.Error("list problems", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load contest")
		return
	}

	summaries := make([]problemSummaryDTO, len(problems))
	for i, p := range problems {
		summaries[i] = problemSummaryDTO{Ord: p.Ord, Title: p.Title}
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{
		"contest":  toContestDTO(contest),
		"problems": summaries,
	})
}

func (s *server) joinContest(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}
	contest, found := s.contestFromPath(w, r)
	if !found {
		return
	}

	if err := s.store.JoinContest(r.Context(), contest.ID, user.ID); err != nil {
		s.log.Error("join contest", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not join contest")
		return
	}

	s.publishEvent(r, contest.ID, events.TypeContestEvent, events.ContestEvent{
		Kind:     "participant_joined",
		Username: user.Username,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) leaderboard(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}

	rows, err := s.leaderboardRows(r.Context(), contest.ID)
	if err != nil {
		s.log.Error("load leaderboard", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load leaderboard")
		return
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"entries": rows})
}

// leaderboardRows serves standings from the Redis read cache, falling back to
// PostgreSQL (the source of truth) on a cold cache or any cache error and
// warming the cache from that authoritative read (ADR-0012 / ADR-0004).
func (s *server) leaderboardRows(ctx context.Context, contestID uuid.UUID) ([]events.LeaderboardRow, error) {
	if members, warm, err := s.redis.TopLeaderboard(ctx, contestID, leaderboardLimit); err != nil {
		s.log.WarnContext(ctx, "leaderboard cache read; falling back to db", "error", err)
	} else if warm {
		rows := make([]events.LeaderboardRow, len(members))
		for i, m := range members {
			rows[i] = events.LeaderboardRow{Rank: i + 1, Username: m.Username, Solved: m.Solved, PenaltyS: m.PenaltyS}
		}
		return rows, nil
	}

	standings, err := s.store.GetLeaderboard(ctx, contestID, leaderboardLimit)
	if err != nil {
		return nil, err
	}

	members := make([]redisx.LeaderboardMember, len(standings))
	rows := make([]events.LeaderboardRow, len(standings))
	for i, e := range standings {
		members[i] = redisx.LeaderboardMember{UserID: e.UserID, Username: e.Username, Solved: e.Solved, PenaltyS: e.PenaltyS}
		rows[i] = events.LeaderboardRow{Rank: i + 1, Username: e.Username, Solved: e.Solved, PenaltyS: e.PenaltyS}
	}
	if err := s.redis.RebuildLeaderboard(ctx, contestID, members); err != nil {
		s.log.WarnContext(ctx, "leaderboard cache warm", "error", err)
	}
	return rows, nil
}

func (s *server) getProblem(w http.ResponseWriter, r *http.Request) {
	_, problem, ok := s.problemFromPath(w, r)
	if !ok {
		return
	}

	respondJSON(w, s.log, http.StatusOK, problemDTO{
		Ord:           problem.Ord,
		Title:         problem.Title,
		StatementMD:   problem.StatementMD,
		TimeLimitMs:   problem.TimeLimitMs,
		MemoryLimitMB: problem.MemoryLimitMB,
	})
}

func (s *server) serveContestWS(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}
	realtime.ServeWS(w, r, s.hub, contest.ID, s.wsOriginPatterns, s.log)
}

// contestFromPath resolves {id}; on failure it writes the error response and
// returns ok=false.
func (s *server) contestFromPath(w http.ResponseWriter, r *http.Request) (db.Contest, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "contest id must be a UUID")
		return db.Contest{}, false
	}
	contest, err := s.store.GetContest(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, s.log, http.StatusNotFound, "not_found", "contest not found")
		return db.Contest{}, false
	}
	if err != nil {
		s.log.Error("load contest", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load contest")
		return db.Contest{}, false
	}
	return contest, true
}

// problemFromPath resolves {id}/{ord}.
func (s *server) problemFromPath(w http.ResponseWriter, r *http.Request) (db.Contest, db.Problem, bool) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return db.Contest{}, db.Problem{}, false
	}

	ord := 0
	for _, ch := range r.PathValue("ord") {
		if ch < '0' || ch > '9' {
			ord = -1
			break
		}
		ord = ord*10 + int(ch-'0')
		if ord > 1000 {
			break
		}
	}
	if ord < 1 {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "problem ord must be a positive integer")
		return db.Contest{}, db.Problem{}, false
	}

	problem, err := s.store.GetProblemByOrd(r.Context(), contest.ID, ord)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, s.log, http.StatusNotFound, "not_found", "problem not found")
		return db.Contest{}, db.Problem{}, false
	}
	if err != nil {
		s.log.Error("load problem", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load problem")
		return db.Contest{}, db.Problem{}, false
	}
	return contest, problem, true
}

func (s *server) publishEvent(r *http.Request, contestID uuid.UUID, eventType string, payload any) {
	data, err := events.Marshal(eventType, payload)
	if err != nil {
		s.log.Error("marshal event", "type", eventType, "error", err)
		return
	}
	if err := s.redis.Publish(r.Context(), redisx.ContestChannel(contestID), data); err != nil {
		s.log.Error("publish event", "type", eventType, "error", err)
	}
}
