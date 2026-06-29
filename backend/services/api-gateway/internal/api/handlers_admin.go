package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
)

// adminSubmissionsLimit caps the moderator "all submissions" read.
const adminSubmissionsLimit = 200

// listUsers returns every account with its role — the admin user-management
// read (RBAC user.manage; ADR-0014).
func (s *server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.log.Error("list users", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list users")
		return
	}
	out := make([]userDTO, len(users))
	for i, u := range users {
		out[i] = toUserDTO(u)
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"users": out})
}

// changeUserRole sets a user's RBAC role (user.manage). Guardrail: a user
// cannot change their OWN role — this prevents accidental self-lockout and
// stops a sole admin from demoting themselves out of the role.
func (s *server) changeUserRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := UserFrom(r.Context())
	if !ok {
		respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "user id must be a UUID")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, maxAuthBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	if !auth.IsValidRole(req.Role) {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "role must be user, moderator, or admin")
		return
	}
	if id == actor.ID {
		respondError(w, s.log, http.StatusConflict, "cannot_change_own_role", "you cannot change your own role")
		return
	}

	user, err := s.store.UpdateUserRole(r.Context(), id, req.Role)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, s.log, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		s.log.Error("update user role", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not update role")
		return
	}
	respondJSON(w, s.log, http.StatusOK, toUserDTO(user))
}

// adminSubmissionDTO is a submission plus its author's username — the moderator
// "all submissions" view (RBAC submission.viewAll). The embedded submissionDTO
// fields are flattened into the JSON object alongside username.
type adminSubmissionDTO struct {
	submissionDTO
	Username string `json:"username"`
}

// listContestSubmissions returns every submission in a contest, newest first,
// for moderators and admins (submission.viewAll).
func (s *server) listContestSubmissions(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}

	subs, err := s.store.ListContestSubmissions(r.Context(), contest.ID, adminSubmissionsLimit)
	if err != nil {
		s.log.Error("list contest submissions", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list submissions")
		return
	}

	out := make([]adminSubmissionDTO, len(subs))
	for i, sub := range subs {
		out[i] = adminSubmissionDTO{
			submissionDTO: toSubmissionDTO(sub.Submission, sub.ProblemOrd),
			Username:      sub.Username,
		}
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"submissions": out})
}

// --- Content authoring (ADR-0015) -----------------------------------------
// Create/edit contests, add problems, attach hidden test cases. Authored
// content is judgeable immediately through the existing submit→judge path.

const maxTestCasesPerRequest = 100

type adminProblemDTO struct {
	ID            string `json:"id"`
	Ord           int    `json:"ord"`
	Title         string `json:"title"`
	StatementMD   string `json:"statement_md,omitempty"`
	TimeLimitMs   int    `json:"time_limit_ms"`
	MemoryLimitMB int    `json:"memory_limit_mb"`
	TestCaseCount int    `json:"test_case_count"`
}

type testCaseDTO struct {
	ID             string `json:"id,omitempty"`
	Ord            int    `json:"ord"`
	Stdin          string `json:"stdin"`
	ExpectedOutput string `json:"expected_output"`
}

// slugify derives a URL-safe contest slug from a title: lowercase, with runs of
// non-alphanumerics collapsed to single hyphens and the ends trimmed. The
// result is still run through validateSlug, which rejects anything too short —
// so a title with too few alphanumerics surfaces a clear error rather than a
// silently bad slug.
func slugify(title string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen && b.Len() > 0:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 64 {
		s = strings.Trim(s[:64], "-")
	}
	return s
}

func (s *server) createContest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string    `json:"title"`
		Slug        string    `json:"slug"`
		Description string    `json:"description"`
		StartsAt    time.Time `json:"starts_at"`
		EndsAt      time.Time `json:"ends_at"`
	}
	if err := decodeJSON(w, r, maxAdminBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = slugify(req.Title)
	}
	for _, v := range []*requestError{
		validateContestTitle(req.Title),
		validateSlug(slug),
		validateContestTimes(req.StartsAt, req.EndsAt),
	} {
		if v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	contest, err := s.store.InsertContest(r.Context(), slug, strings.TrimSpace(req.Title), req.Description, req.StartsAt, req.EndsAt)
	if errors.Is(err, db.ErrSlugTaken) {
		respondError(w, s.log, http.StatusConflict, "slug_taken", "a contest with that slug already exists")
		return
	}
	if err != nil {
		s.log.Error("create contest", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not create contest")
		return
	}
	respondJSON(w, s.log, http.StatusCreated, toContestDTO(contest))
}

func (s *server) updateContest(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}

	// Slug is intentionally immutable here — it keys joins/URLs; title,
	// description, and schedule are editable.
	var req struct {
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartsAt    time.Time `json:"starts_at"`
		EndsAt      time.Time `json:"ends_at"`
	}
	if err := decodeJSON(w, r, maxAdminBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	for _, v := range []*requestError{
		validateContestTitle(req.Title),
		validateContestTimes(req.StartsAt, req.EndsAt),
	} {
		if v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	updated, err := s.store.UpdateContest(r.Context(), contest.ID, strings.TrimSpace(req.Title), req.Description, req.StartsAt, req.EndsAt)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, s.log, http.StatusNotFound, "not_found", "contest not found")
		return
	}
	if err != nil {
		s.log.Error("update contest", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not update contest")
		return
	}
	respondJSON(w, s.log, http.StatusOK, toContestDTO(updated))
}

func (s *server) listAdminProblems(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}

	problems, err := s.store.ListProblemsWithCounts(r.Context(), contest.ID)
	if err != nil {
		s.log.Error("list admin problems", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list problems")
		return
	}
	out := make([]adminProblemDTO, len(problems))
	for i, p := range problems {
		out[i] = adminProblemDTO{
			ID:            p.ID.String(),
			Ord:           p.Ord,
			Title:         p.Title,
			StatementMD:   p.StatementMD,
			TimeLimitMs:   p.TimeLimitMs,
			MemoryLimitMB: p.MemoryLimitMB,
			TestCaseCount: p.TestCaseCount,
		}
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"problems": out})
}

func (s *server) createProblem(w http.ResponseWriter, r *http.Request) {
	contest, ok := s.contestFromPath(w, r)
	if !ok {
		return
	}

	var req struct {
		Title         string `json:"title"`
		StatementMD   string `json:"statement_md"`
		TimeLimitMs   int    `json:"time_limit_ms"`
		MemoryLimitMB int    `json:"memory_limit_mb"`
	}
	if err := decodeJSON(w, r, maxAdminBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	for _, v := range []*requestError{
		validateContestTitle(req.Title),
		validateStatement(req.StatementMD),
		validateTimeLimit(req.TimeLimitMs),
		validateMemoryLimit(req.MemoryLimitMB),
	} {
		if v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	ord, err := s.store.NextProblemOrd(r.Context(), contest.ID)
	if err != nil {
		s.log.Error("next problem ord", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not add problem")
		return
	}
	problem, err := s.store.CreateProblem(r.Context(), db.Problem{
		ContestID:     contest.ID,
		Ord:           ord,
		Title:         strings.TrimSpace(req.Title),
		StatementMD:   req.StatementMD,
		TimeLimitMs:   req.TimeLimitMs,
		MemoryLimitMB: req.MemoryLimitMB,
	})
	if err != nil {
		s.log.Error("create problem", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not add problem")
		return
	}
	respondJSON(w, s.log, http.StatusCreated, adminProblemDTO{
		ID:            problem.ID.String(),
		Ord:           problem.Ord,
		Title:         problem.Title,
		StatementMD:   problem.StatementMD,
		TimeLimitMs:   problem.TimeLimitMs,
		MemoryLimitMB: problem.MemoryLimitMB,
		TestCaseCount: 0,
	})
}

func (s *server) listTestCases(w http.ResponseWriter, r *http.Request) {
	problem, ok := s.problemFromIDPath(w, r)
	if !ok {
		return
	}

	cases, err := s.store.ListTestCases(r.Context(), problem.ID)
	if err != nil {
		s.log.Error("list test cases", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not list test cases")
		return
	}
	out := make([]testCaseDTO, len(cases))
	for i, tc := range cases {
		out[i] = testCaseDTO{ID: tc.ID.String(), Ord: tc.Ord, Stdin: tc.Stdin, ExpectedOutput: tc.ExpectedOutput}
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"test_cases": out})
}

func (s *server) createTestCases(w http.ResponseWriter, r *http.Request) {
	problem, ok := s.problemFromIDPath(w, r)
	if !ok {
		return
	}

	var req struct {
		Cases []struct {
			Stdin          string `json:"stdin"`
			ExpectedOutput string `json:"expected_output"`
		} `json:"cases"`
	}
	if err := decodeJSON(w, r, maxAdminBodyBytes, &req); err != nil {
		var reqErr *requestError
		if errors.As(err, &reqErr) {
			respondRequestError(w, s.log, reqErr)
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "invalid request")
		return
	}
	if len(req.Cases) == 0 {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "provide at least one test case")
		return
	}
	if len(req.Cases) > maxTestCasesPerRequest {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "too many test cases in one request")
		return
	}
	for _, c := range req.Cases {
		if v := validateTestCaseIO(c.Stdin, c.ExpectedOutput); v != nil {
			respondRequestError(w, s.log, v)
			return
		}
	}

	ord, err := s.store.NextTestCaseOrd(r.Context(), problem.ID)
	if err != nil {
		s.log.Error("next test case ord", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not add test cases")
		return
	}
	created := make([]testCaseDTO, 0, len(req.Cases))
	for i, c := range req.Cases {
		tcOrd := ord + i
		if err := s.store.CreateTestCase(r.Context(), db.TestCase{
			ProblemID:      problem.ID,
			Ord:            tcOrd,
			Stdin:          c.Stdin,
			ExpectedOutput: c.ExpectedOutput,
		}); err != nil {
			s.log.Error("create test case", "error", err)
			respondError(w, s.log, http.StatusInternalServerError, "internal", "could not add test cases")
			return
		}
		created = append(created, testCaseDTO{Ord: tcOrd, Stdin: c.Stdin, ExpectedOutput: c.ExpectedOutput})
	}
	respondJSON(w, s.log, http.StatusCreated, map[string]any{"test_cases": created})
}

// problemFromIDPath resolves {problemId} to a problem, writing the error
// response and returning ok=false on failure.
func (s *server) problemFromIDPath(w http.ResponseWriter, r *http.Request) (db.Problem, bool) {
	id, err := uuid.Parse(r.PathValue("problemId"))
	if err != nil {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "problem id must be a UUID")
		return db.Problem{}, false
	}
	problem, err := s.store.GetProblemByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, s.log, http.StatusNotFound, "not_found", "problem not found")
		return db.Problem{}, false
	}
	if err != nil {
		s.log.Error("load problem", "error", err)
		respondError(w, s.log, http.StatusInternalServerError, "internal", "could not load problem")
		return db.Problem{}, false
	}
	return problem, true
}
