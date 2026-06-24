package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/events"
)

const mySubmissionsLimit = 50

// runRequestTimeout bounds an ad-hoc /run call end to end.
const runRequestTimeout = 60 * time.Second

type submissionDTO struct {
	ID          string     `json:"id"`
	ProblemOrd  int        `json:"problem_ord,omitempty"`
	Language    string     `json:"language"`
	Status      string     `json:"status"`
	Verdict     string     `json:"verdict,omitempty"`
	TimeUsedMs  int        `json:"time_used_ms,omitempty"`
	SubmittedAt time.Time  `json:"submitted_at"`
	JudgedAt    *time.Time `json:"judged_at,omitempty"`
}

func toSubmissionDTO(sub db.Submission, problemOrd int) submissionDTO {
	dto := submissionDTO{
		ID:          sub.ID.String(),
		ProblemOrd:  problemOrd,
		Language:    sub.Language,
		Status:      sub.Status,
		SubmittedAt: sub.SubmittedAt,
		JudgedAt:    sub.JudgedAt,
	}
	if sub.Verdict != nil {
		dto.Verdict = *sub.Verdict
	}
	if sub.TimeUsedMs != nil {
		dto.TimeUsedMs = *sub.TimeUsedMs
	}
	return dto
}

func (s *server) createSubmission(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}
	contest, problem, found := s.problemFromPath(w, r)
	if !found {
		return
	}

	now := time.Now()
	if now.Before(contest.StartsAt) {
		respondError(w, s.log, http.StatusForbidden, "contest_not_started", "the contest has not started yet")
		return
	}
	if now.After(contest.EndsAt) {
		respondError(w, s.log, http.StatusForbidden, "contest_ended", "the contest has ended")
		return
	}

	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := decodeJSON(w, r, maxBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	for _, v := range []*requestError{validateLanguage(req.Language), validateCode(req.Code)} {
		if v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	// Submitting implies participating; joining is idempotent.
	if err := s.store.JoinContest(r.Context(), contest.ID, user.ID); err != nil {
		s.log.Error("auto-join on submit", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not submit")
		return
	}

	sub, err := s.store.CreateSubmission(r.Context(), user.ID, problem.ID, contest.ID, req.Language, req.Code)
	if err != nil {
		s.log.Error("create submission", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not submit")
		return
	}

	if !s.judge.Enqueue(r.Context(), sub.ID) {
		// Queue full: the row stays queued and a startup requeue (or a later
		// retry) will pick it up, but tell the client we're saturated.
		s.log.Warn("judge queue full", "submission_id", sub.ID)
		respondError(w, s.log, http.StatusServiceUnavailable, "judge_busy", "judging is saturated; your submission is queued")
		return
	}

	s.publishEvent(r, contest.ID, events.TypeSubmissionUpdate, events.SubmissionUpdate{
		SubmissionID: sub.ID,
		Username:     user.Username,
		ProblemOrd:   problem.Ord,
		Language:     sub.Language,
		Status:       "queued",
		SubmittedAt:  sub.SubmittedAt,
	})

	respondJSON(w, s.log, http.StatusAccepted, toSubmissionDTO(sub, problem.Ord))
}

func (s *server) getSubmission(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "submission id must be a UUID")
		return
	}

	sub, err := s.store.GetSubmission(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) || (err == nil && sub.UserID != user.ID) {
		// Non-owners get the same 404 as nonexistent ids: no resource probing.
		respondError(w, s.log, http.StatusNotFound, "not_found", "submission not found")
		return
	}
	if err != nil {
		s.log.Error("load submission", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load submission")
		return
	}

	respondJSON(w, s.log, http.StatusOK, toSubmissionDTO(sub, 0))
}

func (s *server) listMySubmissions(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}
	contest, found := s.contestFromPath(w, r)
	if !found {
		return
	}

	subs, err := s.store.ListUserContestSubmissions(r.Context(), contest.ID, user.ID, mySubmissionsLimit)
	if err != nil {
		s.log.Error("list submissions", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list submissions")
		return
	}

	// Map problem IDs to ords for display.
	problems, err := s.store.ListProblems(r.Context(), contest.ID)
	if err != nil {
		s.log.Error("list problems for submissions", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list submissions")
		return
	}
	ordByProblem := make(map[uuid.UUID]int, len(problems))
	for _, p := range problems {
		ordByProblem[p.ID] = p.Ord
	}

	out := make([]submissionDTO, len(subs))
	for i, sub := range subs {
		out[i] = toSubmissionDTO(sub, ordByProblem[sub.ProblemID])
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"submissions": out})
}

type runResponse struct {
	Status     string `json:"status"` // ok | compile_error | runtime_error | timeout | memory_exceeded | error
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	CompileOut string `json:"compile_output,omitempty"`
	ExitCode   int32  `json:"exit_code"`
	TimeUsedMs uint32 `json:"time_used_ms"`
}

// runCode executes code with custom stdin and returns raw output — the
// editor's "Run" button. Nothing is judged or persisted.
func (s *server) runCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
		Stdin    string `json:"stdin"`
	}
	if err := decodeJSON(w, r, maxBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	for _, v := range []*requestError{validateLanguage(req.Language), validateCode(req.Code), validateStdin(req.Stdin)} {
		if v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), runRequestTimeout)
	defer cancel()

	resp, err := s.executor.Execute(ctx, &executorv1.ExecuteRequest{
		Code:     req.Code,
		Language: protoLanguage(req.Language),
		Stdin:    req.Stdin,
		// No expected output: the verdict's AC/WA distinction is meaningless
		// here; only the execution outcome classes matter.
	})
	if err != nil {
		s.log.Error("ad-hoc run failed", "error", err)
		respondError(w, s.log, http.StatusBadGateway, "executor_unavailable", "could not run code; try again")
		return
	}

	respondJSON(w, s.log, http.StatusOK, runResponse{
		Status:     runStatus(resp.GetVerdict()),
		Stdout:     resp.GetStdout(),
		Stderr:     resp.GetStderr(),
		CompileOut: resp.GetCompileOutput(),
		ExitCode:   resp.GetExitCode(),
		TimeUsedMs: resp.GetTimeUsedMs(),
	})
}

// runStatus collapses judge verdicts into execution outcomes for /run,
// where there is no expected output to be "wrong" against.
func runStatus(v executorv1.Verdict) string {
	switch v {
	case executorv1.Verdict_VERDICT_ACCEPTED, executorv1.Verdict_VERDICT_WRONG_ANSWER:
		return "ok"
	case executorv1.Verdict_VERDICT_COMPILATION_ERROR:
		return "compile_error"
	case executorv1.Verdict_VERDICT_RUNTIME_ERROR:
		return "runtime_error"
	case executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED:
		return "timeout"
	case executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED:
		return "memory_exceeded"
	default:
		return "error"
	}
}

func protoLanguage(language string) executorv1.Language {
	switch language {
	case "cpp":
		return executorv1.Language_LANGUAGE_CPP
	case "python":
		return executorv1.Language_LANGUAGE_PYTHON
	case "go":
		return executorv1.Language_LANGUAGE_GO
	default:
		return executorv1.Language_LANGUAGE_UNSPECIFIED
	}
}
