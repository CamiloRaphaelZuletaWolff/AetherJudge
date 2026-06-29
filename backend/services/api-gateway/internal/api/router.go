package api

import (
	"log/slog"
	"net/http"
	"net/url"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
	"github.com/caezu/arena/backend/services/api-gateway/internal/config"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/judge"
	"github.com/caezu/arena/backend/services/api-gateway/internal/realtime"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

// Deps carries everything the router needs, wired once in main.
type Deps struct {
	Cfg      config.Config
	Log      *slog.Logger
	Store    *db.Store
	Redis    *redisx.Client
	Tokens   *auth.TokenIssuer
	Refresh  *auth.RefreshManager
	Judge    *judge.Service
	Hub      *realtime.Hub
	Executor executorv1.ExecutorServiceClient
}

// server holds handler state; one instance serves all requests.
type server struct {
	cfg      config.Config
	log      *slog.Logger
	store    *db.Store
	redis    *redisx.Client
	tokens   *auth.TokenIssuer
	refresh  *auth.RefreshManager
	judge    *judge.Service
	hub      *realtime.Hub
	executor executorv1.ExecutorServiceClient

	// wsOriginPatterns is derived from FrontendOrigin for websocket.Accept.
	wsOriginPatterns []string
}

// NewRouter assembles the full public HTTP surface.
func NewRouter(d Deps) http.Handler {
	s := &server{
		cfg:      d.Cfg,
		log:      d.Log,
		store:    d.Store,
		redis:    d.Redis,
		tokens:   d.Tokens,
		refresh:  d.Refresh,
		judge:    d.Judge,
		hub:      d.Hub,
		executor: d.Executor,
	}
	if u, err := url.Parse(d.Cfg.FrontendOrigin); err == nil && u.Host != "" {
		s.wsOriginPatterns = []string{u.Host}
	}

	mux := http.NewServeMux()

	// Probes. Liveness is unconditional; readiness checks the dependencies
	// this service cannot work without.
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)

	// Auth (IP rate-limited: these are the credential-guessing surface).
	mux.HandleFunc("POST /api/v1/auth/signup", s.rateLimited("auth", s.cfg.AuthRatePerMin, byIP, s.signup))
	mux.HandleFunc("POST /api/v1/auth/login", s.rateLimited("auth", s.cfg.AuthRatePerMin, byIP, s.login))
	mux.HandleFunc("POST /api/v1/auth/refresh", s.rateLimited("auth", s.cfg.AuthRatePerMin, byIP, s.refreshToken))
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/me", s.requireAuth(s.me))

	// Contests.
	mux.HandleFunc("GET /api/v1/contests", s.listContests)
	mux.HandleFunc("GET /api/v1/contests/{id}", s.getContest)
	mux.HandleFunc("POST /api/v1/contests/{id}/join", s.requireAuth(s.joinContest))
	mux.HandleFunc("GET /api/v1/contests/{id}/leaderboard", s.leaderboard)
	mux.HandleFunc("GET /api/v1/contests/{id}/problems/{ord}", s.requireAuth(s.getProblem))

	// Submissions and ad-hoc runs (user rate-limited: each one costs a
	// sandbox).
	mux.HandleFunc("POST /api/v1/contests/{id}/problems/{ord}/submissions",
		s.requireAuth(s.rateLimited("submit", s.cfg.SubmitRatePerMin, byUser, s.createSubmission)))
	mux.HandleFunc("GET /api/v1/submissions/{id}", s.requireAuth(s.getSubmission))
	mux.HandleFunc("GET /api/v1/contests/{id}/submissions", s.requireAuth(s.listMySubmissions))
	mux.HandleFunc("POST /api/v1/run",
		s.requireAuth(s.rateLimited("run", s.cfg.RunRatePerMin, byUser, s.runCode)))

	// Admin / moderator surface. Every route is requireAuth + requirePermission
	// — the real authorization boundary (the frontend's gating is UX only;
	// ADR-0014).
	mux.HandleFunc("GET /api/v1/admin/users",
		s.requireAuth(s.requirePermission(auth.PermUserManage, s.listUsers)))
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}/role",
		s.requireAuth(s.requirePermission(auth.PermUserManage, s.changeUserRole)))
	mux.HandleFunc("GET /api/v1/admin/contests/{id}/submissions",
		s.requireAuth(s.requirePermission(auth.PermSubmissionViewAll, s.listContestSubmissions)))

	// Content authoring (ADR-0015): create/edit contests, add problems and
	// hidden test cases. contest.create/contest.edit/problem.manage.
	mux.HandleFunc("POST /api/v1/admin/contests",
		s.requireAuth(s.requirePermission(auth.PermContestCreate, s.createContest)))
	mux.HandleFunc("PATCH /api/v1/admin/contests/{id}",
		s.requireAuth(s.requirePermission(auth.PermContestEdit, s.updateContest)))
	mux.HandleFunc("GET /api/v1/admin/contests/{id}/problems",
		s.requireAuth(s.requirePermission(auth.PermProblemManage, s.listAdminProblems)))
	mux.HandleFunc("POST /api/v1/admin/contests/{id}/problems",
		s.requireAuth(s.requirePermission(auth.PermProblemManage, s.createProblem)))
	mux.HandleFunc("GET /api/v1/admin/problems/{problemId}/test-cases",
		s.requireAuth(s.requirePermission(auth.PermProblemManage, s.listTestCases)))
	mux.HandleFunc("POST /api/v1/admin/problems/{problemId}/test-cases",
		s.requireAuth(s.requirePermission(auth.PermProblemManage, s.createTestCases)))
	// Parse an uploaded file (.txt/.md/.csv/.json/.xlsx) into test cases without
	// writing — the client previews then commits via the batch endpoint above
	// (ADR-0016).
	mux.HandleFunc("POST /api/v1/admin/test-cases/parse",
		s.requireAuth(s.requirePermission(auth.PermProblemManage, s.parseTestCasesUpload)))

	// Live room channel.
	mux.HandleFunc("GET /api/v1/ws/contests/{id}", s.requireAuth(s.serveContestWS))

	// Security headers wrap the CORS layer so they ride every response,
	// including preflight 204s and error envelopes.
	return s.withSecurityHeaders(s.withCORS(mux))
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, s.log, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		respondError(w, s.log, http.StatusServiceUnavailable, "not_ready", "database unavailable")
		return
	}
	if err := s.redis.Ping(r.Context()); err != nil {
		respondError(w, s.log, http.StatusServiceUnavailable, "not_ready", "redis unavailable")
		return
	}
	respondJSON(w, s.log, http.StatusOK, map[string]string{"status": "ready"})
}
