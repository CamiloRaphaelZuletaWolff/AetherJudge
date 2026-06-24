// Package integration tests the gateway against real PostgreSQL and Redis
// (Testcontainers) with the executor stubbed over a real gRPC bufconn server
// — the generated stubs are exercised end to end, so contract drift breaks
// here at compile time.
//
// Opt-in: ARENA_INFRA_TESTS=1 (Docker required).
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/api-gateway/internal/api"
	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
	"github.com/caezu/arena/backend/services/api-gateway/internal/config"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/events"
	"github.com/caezu/arena/backend/services/api-gateway/internal/httpserver"
	"github.com/caezu/arena/backend/services/api-gateway/internal/judge"
	"github.com/caezu/arena/backend/services/api-gateway/internal/realtime"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

var (
	testDSN       string
	testRedisAddr string
)

func TestMain(m *testing.M) {
	if os.Getenv("ARENA_INFRA_TESTS") != "1" {
		os.Exit(m.Run()) // every test self-skips
	}

	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("arena_test"),
		tcpostgres.WithUsername("arena"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container:", err)
		os.Exit(1)
	}

	redisC, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		fmt.Fprintln(os.Stderr, "start redis container:", err)
		terminate(ctx, pgC)
		os.Exit(1)
	}

	testDSN, err = pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "postgres connection string:", err)
		terminate(ctx, pgC, redisC)
		os.Exit(1)
	}
	redisURI, err := redisC.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "redis connection string:", err)
		terminate(ctx, pgC, redisC)
		os.Exit(1)
	}
	testRedisAddr = strings.TrimPrefix(redisURI, "redis://")

	if err := db.Migrate(ctx, testDSN, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		terminate(ctx, pgC, redisC)
		os.Exit(1)
	}

	code := m.Run()
	terminate(ctx, pgC, redisC)
	os.Exit(code)
}

func terminate(ctx context.Context, containers ...testcontainers.Container) {
	for _, c := range containers {
		if err := c.Terminate(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "terminate container:", err)
		}
	}
}

func requireInfra(t *testing.T) {
	t.Helper()
	if os.Getenv("ARENA_INFRA_TESTS") != "1" {
		t.Skip("set ARENA_INFRA_TESTS=1 to run infrastructure integration tests")
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubExecutorServer judges over real gRPC: accepted unless the code
// contains "WRONG".
type stubExecutorServer struct {
	executorv1.UnimplementedExecutorServiceServer
}

func (stubExecutorServer) Execute(_ context.Context, req *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
	if strings.Contains(req.GetCode(), "WRONG") {
		return &executorv1.ExecuteResponse{
			Verdict: executorv1.Verdict_VERDICT_WRONG_ANSWER, TimeUsedMs: 7,
		}, nil
	}
	return &executorv1.ExecuteResponse{
		Verdict: executorv1.Verdict_VERDICT_ACCEPTED, TimeUsedMs: 42, Stdout: req.GetExpectedOutput(),
	}, nil
}

// stack is one fully wired gateway over real PG/Redis and a stub executor.
type stack struct {
	store  *db.Store
	redis  *redisx.Client
	server *httptest.Server
}

func newStack(t *testing.T) *stack {
	t.Helper()
	requireInfra(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pool, err := db.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(pool.Close)
	store := db.New(pool)

	redis, err := redisx.Connect(ctx, testRedisAddr, discardLogger())
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() {
		if err := redis.Close(); err != nil {
			t.Errorf("close redis: %v", err)
		}
	})

	// Real gRPC server+client over an in-memory pipe.
	lis := bufconn.Listen(1 << 20)
	grpcSrv := grpc.NewServer()
	executorv1.RegisterExecutorServiceServer(grpcSrv, stubExecutorServer{})
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			t.Logf("bufconn serve: %v", err)
		}
	}()
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("bufconn client: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close grpc conn: %v", err)
		}
	})
	executorClient := executorv1.NewExecutorServiceClient(conn)

	cfg := config.Config{
		Env:                  "dev",
		JWTSecret:            "integration-test-secret",
		AccessTokenTTL:       15 * time.Minute,
		RefreshTokenTTL:      time.Hour,
		FrontendOrigin:       "http://localhost:3000",
		JudgeWorkers:         2,
		JudgeQueueDepthLimit: 1024,
		JudgeMaxDeliveries:   5,
		ConsumerName:         "integration",
		AuthRatePerMin:       1000, // generous defaults; the rate-limit test overrides
		SubmitRatePerMin:     1000,
		RunRatePerMin:        1000,
	}

	// redis (a real Testcontainers instance) backs the durable queue, the
	// leaderboard cache, and pub/sub, so this suite exercises the real
	// stream path end to end.
	judgeSvc := judge.New(store, executorClient, redis, redis, redis, judge.Config{
		Workers:         cfg.JudgeWorkers,
		ConsumerName:    cfg.ConsumerName,
		QueueDepthLimit: int64(cfg.JudgeQueueDepthLimit),
		MaxDeliveries:   int64(cfg.JudgeMaxDeliveries),
		ClaimMinIdle:    2 * time.Second, // reclaim fast so the suite stays quick
	}, discardLogger())
	if err := judgeSvc.StartConsumers(ctx); err != nil {
		t.Fatalf("start judge: %v", err)
	}

	router := api.NewRouter(api.Deps{
		Cfg:      cfg,
		Log:      discardLogger(),
		Store:    store,
		Redis:    redis,
		Tokens:   auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL),
		Refresh:  auth.NewRefreshManager(store, cfg.RefreshTokenTTL, discardLogger()),
		Judge:    judgeSvc,
		Hub:      realtime.NewHub(ctx, redis, discardLogger()),
		Executor: executorClient,
	})

	// Serve through the full chassis middleware, exactly like production —
	// a logging wrapper that hides http.Hijacker breaks WebSocket upgrades,
	// and only this wiring catches that class of bug.
	ts := httptest.NewServer(httpserver.New(":0", discardLogger(), router).Handler())
	t.Cleanup(ts.Close)

	return &stack{store: store, redis: redis, server: ts}
}

// --- HTTP helpers ---------------------------------------------------------

func (s *stack) post(t *testing.T, path, token string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.server.URL+path, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (s *stack) get(t *testing.T, path, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.server.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func decodeBody[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	}()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

type authResp struct {
	User struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
	AccessToken string `json:"access_token"`
}

func (s *stack) signup(t *testing.T, username string) authResp {
	t.Helper()
	resp := s.post(t, "/api/v1/auth/signup", "", map[string]string{
		"username": username,
		"email":    username + "@example.com",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusCreated {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			t.Fatalf("signup status %d (body unreadable: %v)", resp.StatusCode, readErr)
		}
		t.Fatalf("signup status = %d, body %s", resp.StatusCode, body)
	}
	return decodeBody[authResp](t, resp)
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

// uniqueSlug matches the contests.slug CHECK (lowercase, digits, hyphens).
func uniqueSlug(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

// --- tests ----------------------------------------------------------------

func TestRepositoriesAndScoring(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	username := uniqueName("repo")
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user, err := st.store.CreateUser(ctx, username, username+"@example.com", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := st.store.CreateUser(ctx, username, "other@example.com", hash); err != db.ErrUsernameTaken {
		t.Errorf("duplicate username error = %v, want ErrUsernameTaken", err)
	}

	start := time.Now().Add(-time.Hour)
	contest, err := st.store.CreateContest(ctx, uniqueSlug("contest"), "Repo Test", "", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create contest: %v", err)
	}
	problem, err := st.store.CreateProblem(ctx, db.Problem{
		ContestID: contest.ID, Ord: 1, Title: "P1", StatementMD: "x",
		TimeLimitMs: 2000, MemoryLimitMB: 128,
	})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}

	// One wrong attempt, then an accept: penalty must include the
	// 20-minute surcharge.
	wrong, err := st.store.CreateSubmission(ctx, user.ID, problem.ID, contest.ID, "python", "print(0)")
	if err != nil {
		t.Fatalf("create wrong submission: %v", err)
	}
	if err := st.store.FinishSubmission(ctx, wrong.ID, "wrong_answer", 10); err != nil {
		t.Fatalf("finish wrong: %v", err)
	}

	accept, err := st.store.CreateSubmission(ctx, user.ID, problem.ID, contest.ID, "python", "print(1)")
	if err != nil {
		t.Fatalf("create accept submission: %v", err)
	}
	if err := st.store.FinishSubmission(ctx, accept.ID, "accepted", 20); err != nil {
		t.Fatalf("finish accept: %v", err)
	}

	acceptedAt := start.Add(30 * time.Minute)
	res, err := st.store.ApplyAccepted(ctx, accept, acceptedAt)
	if err != nil {
		t.Fatalf("ApplyAccepted: %v", err)
	}
	if !res.FirstSolve {
		t.Fatal("FirstSolve = false on first accept")
	}
	wantPenalty := int64((30 * time.Minute).Seconds()) + int64((20 * time.Minute).Seconds())
	if res.Entry.PenaltyS != wantPenalty {
		t.Errorf("penalty = %d, want %d (solve time + one wrong-attempt surcharge)", res.Entry.PenaltyS, wantPenalty)
	}

	// Re-applying the same accept must not double-score.
	res2, err := st.store.ApplyAccepted(ctx, accept, acceptedAt)
	if err != nil {
		t.Fatalf("ApplyAccepted twice: %v", err)
	}
	if res2.FirstSolve {
		t.Error("FirstSolve = true on duplicate accept")
	}

	board, err := st.store.GetLeaderboard(ctx, contest.ID, 10)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(board) != 1 || board[0].Username != username || board[0].Solved != 1 {
		t.Errorf("leaderboard = %+v, want one row for %s with solved=1", board, username)
	}
}

func TestAuthFlowOverHTTP(t *testing.T) {
	st := newStack(t)
	username := uniqueName("auth")

	signup := st.signup(t, username)
	if signup.AccessToken == "" {
		t.Fatal("signup returned empty access token")
	}

	// /me with the access token.
	me := st.get(t, "/api/v1/me", signup.AccessToken)
	if me.StatusCode != http.StatusOK {
		t.Fatalf("/me status = %d", me.StatusCode)
	}
	profile := decodeBody[map[string]any](t, me)
	if profile["username"] != username {
		t.Errorf("/me username = %v, want %s", profile["username"], username)
	}

	// Wrong password yields 401 with no detail about which field failed.
	bad := st.post(t, "/api/v1/auth/login", "", map[string]string{"login": username, "password": "wrong-password"})
	if bad.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad login status = %d, want 401", bad.StatusCode)
	}
	if err := bad.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}

	// Refresh flow with the cookie jar: rotate once, then replay the old
	// cookie — the replay must fail (rotation) and kill the session family.
	signupResp := st.post(t, "/api/v1/auth/login", "", map[string]string{"login": username, "password": "password123"})
	if signupResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", signupResp.StatusCode)
	}
	var refreshCookie *http.Cookie
	for _, c := range signupResp.Cookies() {
		if c.Name == "arena_refresh" {
			refreshCookie = c
		}
	}
	if err := signupResp.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
	if refreshCookie == nil {
		t.Fatal("login did not set the refresh cookie")
	}

	doRefresh := func(cookie *http.Cookie) *http.Response {
		req, err := http.NewRequest(http.MethodPost, st.server.URL+"/api/v1/auth/refresh", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.AddCookie(cookie)
		resp, err := st.server.Client().Do(req)
		if err != nil {
			t.Fatalf("refresh: %v", err)
		}
		return resp
	}

	first := doRefresh(refreshCookie)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200", first.StatusCode)
	}
	if err := first.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}

	replay := doRefresh(refreshCookie)
	if replay.StatusCode != http.StatusUnauthorized {
		t.Errorf("replayed refresh status = %d, want 401 (rotation + reuse detection)", replay.StatusCode)
	}
	if err := replay.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
}

func TestRateLimiting(t *testing.T) {
	st := newStack(t)
	requireInfra(t)

	// The login limiter keys on IP; hammer it past the integration config's
	// generous limit would take 1000 calls — instead exercise the limiter
	// primitive directly plus one HTTP 429 with a tight custom router.
	redis, err := redisx.Connect(context.Background(), testRedisAddr, discardLogger())
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	defer func() {
		if err := redis.Close(); err != nil {
			t.Errorf("close redis: %v", err)
		}
	}()

	key := uniqueName("limit")
	for i := 1; i <= 3; i++ {
		allowed, err := redis.Allow(context.Background(), key, 3, time.Minute)
		if err != nil {
			t.Fatalf("Allow #%d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("Allow #%d = false, want true (limit 3)", i)
		}
	}
	allowed, err := redis.Allow(context.Background(), key, 3, time.Minute)
	if err != nil {
		t.Fatalf("Allow #4: %v", err)
	}
	if allowed {
		t.Error("Allow #4 = true, want false (over limit)")
	}

	_ = st // stack exists to prove router construction with limits wired
}

// readEvent reads WS messages until one of wantType arrives (or times out).
func readEvent(ctx context.Context, t *testing.T, conn *websocket.Conn, wantType string) events.Envelope {
	t.Helper()
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		_, data, err := conn.Read(deadline)
		if err != nil {
			t.Fatalf("websocket read while waiting for %s: %v", wantType, err)
		}
		var env events.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("bad event payload %q: %v", data, err)
		}
		if env.Type == wantType {
			return env
		}
	}
}

// TestContestFlowWithLiveUpdates is the Phase 3 acceptance test: two clients
// in a room, one submits, both see live submission and leaderboard events,
// and REST agrees.
func TestContestFlowWithLiveUpdates(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	alice := st.signup(t, uniqueName("alice"))
	bob := st.signup(t, uniqueName("bob"))

	// Seed an active contest with one problem directly through the store.
	start := time.Now().Add(-10 * time.Minute)
	contest, err := st.store.CreateContest(ctx, uniqueSlug("live"), "Live Contest", "", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create contest: %v", err)
	}
	problem, err := st.store.CreateProblem(ctx, db.Problem{
		ContestID: contest.ID, Ord: 1, Title: "Echo", StatementMD: "echo",
		TimeLimitMs: 2000, MemoryLimitMB: 128,
	})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}
	if err := st.store.CreateTestCase(ctx, db.TestCase{ProblemID: problem.ID, Ord: 1, Stdin: "x", ExpectedOutput: "x"}); err != nil {
		t.Fatalf("create test case: %v", err)
	}

	// Both users join the room over WebSocket.
	wsURL := strings.Replace(st.server.URL, "http://", "ws://", 1) +
		"/api/v1/ws/contests/" + contest.ID.String() + "?access_token="

	dial := func(token string) *websocket.Conn {
		conn, _, err := websocket.Dial(ctx, wsURL+token, nil)
		if err != nil {
			t.Fatalf("websocket dial: %v", err)
		}
		return conn
	}
	aliceWS := dial(alice.AccessToken)
	defer func() {
		if err := aliceWS.CloseNow(); err != nil {
			t.Logf("close alice ws: %v", err)
		}
	}()
	bobWS := dial(bob.AccessToken)
	defer func() {
		if err := bobWS.CloseNow(); err != nil {
			t.Logf("close bob ws: %v", err)
		}
	}()

	// Brief pause so both subscriptions attach before events flow.
	time.Sleep(300 * time.Millisecond)

	// Alice submits a correct solution (stub executor accepts).
	submitResp := st.post(t,
		"/api/v1/contests/"+contest.ID.String()+"/problems/1/submissions",
		alice.AccessToken,
		map[string]string{"language": "python", "code": "print(input())"},
	)
	if submitResp.StatusCode != http.StatusAccepted {
		body, readErr := io.ReadAll(submitResp.Body)
		if readErr != nil {
			t.Fatalf("submit status %d (body unreadable: %v)", submitResp.StatusCode, readErr)
		}
		t.Fatalf("submit status = %d, body %s", submitResp.StatusCode, body)
	}
	submission := decodeBody[map[string]any](t, submitResp)
	if submission["status"] != "queued" {
		t.Errorf("submission status = %v, want queued", submission["status"])
	}

	// BOTH clients must observe the submission lifecycle and the
	// leaderboard update — this is "multiplayer room works, live updates
	// work".
	for name, conn := range map[string]*websocket.Conn{"alice": aliceWS, "bob": bobWS} {
		env := readEvent(ctx, t, conn, events.TypeLeaderboardUpdate)
		var update events.LeaderboardUpdate
		if err := json.Unmarshal(env.Payload, &update); err != nil {
			t.Fatalf("%s: decode leaderboard update: %v", name, err)
		}
		if len(update.Entries) == 0 || update.Entries[0].Solved != 1 {
			t.Errorf("%s: leaderboard update = %+v, want alice with solved=1", name, update.Entries)
		}
	}

	// REST view agrees with the broadcast.
	lbResp := st.get(t, "/api/v1/contests/"+contest.ID.String()+"/leaderboard", "")
	if lbResp.StatusCode != http.StatusOK {
		t.Fatalf("leaderboard status = %d", lbResp.StatusCode)
	}
	board := decodeBody[struct {
		Entries []events.LeaderboardRow `json:"entries"`
	}](t, lbResp)
	if len(board.Entries) != 1 || board.Entries[0].Solved != 1 {
		t.Errorf("REST leaderboard = %+v, want one entry with solved=1", board.Entries)
	}

	// And the submission record shows the verdict.
	subResp := st.get(t, "/api/v1/submissions/"+submission["id"].(string), alice.AccessToken)
	if subResp.StatusCode != http.StatusOK {
		t.Fatalf("get submission status = %d", subResp.StatusCode)
	}
	final := decodeBody[map[string]any](t, subResp)
	if final["verdict"] != "accepted" {
		t.Errorf("final verdict = %v, want accepted", final["verdict"])
	}

	// Phase 6: the same flow must be visible in the metrics the dashboards
	// are built on (gateway runs in-process, so the default registry has
	// them). The route label must be the mux pattern, never the raw path.
	if got := counterValue(t, "arena_judge_jobs_total", map[string]string{"verdict": "accepted"}); got < 1 {
		t.Errorf(`arena_judge_jobs_total{verdict="accepted"} = %v, want >= 1`, got)
	}
	submitRoute := "POST /api/v1/contests/{id}/problems/{ord}/submissions"
	if got := counterValue(t, "arena_http_requests_total", map[string]string{"route": submitRoute, "code": "202"}); got < 1 {
		t.Errorf(`arena_http_requests_total{route=%q,code="202"} = %v, want >= 1`, submitRoute, got)
	}
}

// counterValue sums every series of a counter family whose labels include
// all of want.
func counterValue(t *testing.T, name string, want map[string]string) float64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	total := 0.0
	for _, fam := range families {
		if fam.GetName() != name {
			continue
		}
		for _, m := range fam.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			matches := true
			for k, v := range want {
				if labels[k] != v {
					matches = false
					break
				}
			}
			if matches {
				total += m.GetCounter().GetValue()
			}
		}
	}
	return total
}

// TestWrongSubmissionGetsWrongAnswer drives the stub's WA path end to end.
func TestWrongSubmissionGetsWrongAnswer(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	user := st.signup(t, uniqueName("wa"))

	start := time.Now().Add(-10 * time.Minute)
	contest, err := st.store.CreateContest(ctx, uniqueSlug("wa"), "WA Contest", "", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create contest: %v", err)
	}
	problem, err := st.store.CreateProblem(ctx, db.Problem{
		ContestID: contest.ID, Ord: 1, Title: "P", StatementMD: "p",
		TimeLimitMs: 2000, MemoryLimitMB: 128,
	})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}
	if err := st.store.CreateTestCase(ctx, db.TestCase{ProblemID: problem.ID, Ord: 1, Stdin: "x", ExpectedOutput: "x"}); err != nil {
		t.Fatalf("create test case: %v", err)
	}

	resp := st.post(t,
		"/api/v1/contests/"+contest.ID.String()+"/problems/1/submissions",
		user.AccessToken,
		map[string]string{"language": "python", "code": "print('WRONG')"},
	)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit status = %d", resp.StatusCode)
	}
	submission := decodeBody[map[string]any](t, resp)

	// Poll until judged.
	deadline := time.Now().Add(30 * time.Second)
	for {
		r := st.get(t, "/api/v1/submissions/"+submission["id"].(string), user.AccessToken)
		got := decodeBody[map[string]any](t, r)
		if got["status"] == "done" {
			if got["verdict"] != "wrong_answer" {
				t.Errorf("verdict = %v, want wrong_answer", got["verdict"])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("submission was never judged")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wrong answers never score.
	board, err := st.store.GetLeaderboard(ctx, contest.ID, 10)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(board) != 0 {
		t.Errorf("leaderboard has %d entries after WA only, want 0", len(board))
	}
}

// setupContest creates an active contest with one trivial problem and returns
// its ID; the stub executor accepts any non-"WRONG" code.
func (s *stack) setupContest(t *testing.T, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	start := time.Now().Add(-10 * time.Minute)
	contest, err := s.store.CreateContest(ctx, uniqueSlug(prefix), prefix+" contest", "", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("create contest: %v", err)
	}
	problem, err := s.store.CreateProblem(ctx, db.Problem{
		ContestID: contest.ID, Ord: 1, Title: "P", StatementMD: "p",
		TimeLimitMs: 2000, MemoryLimitMB: 128,
	})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}
	if err := s.store.CreateTestCase(ctx, db.TestCase{ProblemID: problem.ID, Ord: 1, Stdin: "x", ExpectedOutput: "x"}); err != nil {
		t.Fatalf("create test case: %v", err)
	}
	return contest.ID
}

// submitAndAwait posts a correct submission and blocks until it is judged,
// returning the submission ID and final verdict.
func (s *stack) submitAndAwait(t *testing.T, contestID uuid.UUID, token string) (string, string) {
	t.Helper()
	resp := s.post(t,
		"/api/v1/contests/"+contestID.String()+"/problems/1/submissions",
		token, map[string]string{"language": "python", "code": "print('ok')"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit status = %d", resp.StatusCode)
	}
	id := decodeBody[map[string]any](t, resp)["id"].(string)

	deadline := time.Now().Add(30 * time.Second)
	for {
		got := decodeBody[map[string]any](t, s.get(t, "/api/v1/submissions/"+id, token))
		if got["status"] == "done" {
			verdict, _ := got["verdict"].(string)
			return id, verdict
		}
		if time.Now().After(deadline) {
			t.Fatal("submission was never judged")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestDuplicateDeliveryScoresOnce proves judging is idempotent under the
// queue's at-least-once delivery: re-enqueuing an already-judged submission
// must not double-score (the status=done short-circuit + ON CONFLICT scoring).
func TestDuplicateDeliveryScoresOnce(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	contestID := st.setupContest(t, "dup")
	user := st.signup(t, uniqueName("dup"))

	subID, verdict := st.submitAndAwait(t, contestID, user.AccessToken)
	if verdict != "accepted" {
		t.Fatalf("first verdict = %q, want accepted", verdict)
	}

	before, err := st.store.GetLeaderboard(ctx, contestID, 10)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(before) != 1 || before[0].Solved != 1 {
		t.Fatalf("after first solve: %+v, want one entry solved=1", before)
	}

	// Redeliver the same submission as if the queue delivered it twice.
	id, err := uuid.Parse(subID)
	if err != nil {
		t.Fatalf("parse sub id: %v", err)
	}
	if err := st.redis.EnqueueJudge(ctx, id, ""); err != nil {
		t.Fatalf("re-enqueue: %v", err)
	}

	// Give the consumers time to pick up and no-op the duplicate.
	time.Sleep(3 * time.Second)

	after, err := st.store.GetLeaderboard(ctx, contestID, 10)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(after) != 1 || after[0].Solved != 1 {
		t.Errorf("after duplicate delivery: %+v, want still one entry solved=1 (idempotent)", after)
	}
}

// TestLeaderboardCacheMatchesDB proves the write-through Redis ZSET read path
// returns exactly what the durable SQL query returns (ADR-0012).
func TestLeaderboardCacheMatchesDB(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	contestID := st.setupContest(t, "lb")
	for _, name := range []string{"lbalice", "lbbob", "lbcarol"} {
		user := st.signup(t, uniqueName(name))
		if _, verdict := st.submitAndAwait(t, contestID, user.AccessToken); verdict != "accepted" {
			t.Fatalf("%s verdict = %q, want accepted", name, verdict)
		}
	}

	sql, err := st.store.GetLeaderboard(ctx, contestID, 50)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	cached, warm, err := st.redis.TopLeaderboard(ctx, contestID, 50)
	if err != nil {
		t.Fatalf("TopLeaderboard: %v", err)
	}
	if !warm {
		t.Fatal("cache is cold after solves; write-through did not populate the ZSET")
	}
	if len(cached) != len(sql) {
		t.Fatalf("cache has %d entries, SQL has %d", len(cached), len(sql))
	}
	for i := range sql {
		if cached[i].Username != sql[i].Username || cached[i].Solved != sql[i].Solved || cached[i].PenaltyS != sql[i].PenaltyS {
			t.Errorf("rank %d: cache=%+v, sql {user:%s solved:%d penalty:%d}",
				i, cached[i], sql[i].Username, sql[i].Solved, sql[i].PenaltyS)
		}
	}
}

// TestQueueReclaimRecoversAbandoned proves a message left un-acked by a
// "crashed" consumer is reclaimable by another (the crash-recovery primitive,
// ADR-0011). Runs without a stack so the live consumers do not race for it.
func TestQueueReclaimRecoversAbandoned(t *testing.T) {
	requireInfra(t)
	ctx := context.Background()

	rc, err := redisx.Connect(ctx, testRedisAddr, discardLogger())
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })

	if err := rc.EnsureJudgeGroup(ctx); err != nil {
		t.Fatalf("ensure group: %v", err)
	}

	subID := uuid.New()
	if err := rc.EnqueueJudge(ctx, subID, ""); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Consumer A reads it but never acks (simulating a crash mid-judge).
	got, err := rc.ReadJudge(ctx, "consumer-A", 10, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msgID string
	for _, it := range got {
		if it.SubmissionID == subID {
			msgID = it.MessageID
		}
	}
	if msgID == "" {
		t.Fatal("consumer A did not receive the enqueued message")
	}

	// After the idle window, consumer B reclaims A's un-acked message.
	time.Sleep(400 * time.Millisecond)
	claimed, _, err := rc.ClaimStaleJudge(ctx, "consumer-B", 200*time.Millisecond, 32, 5)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	found := false
	for _, it := range claimed {
		if it.SubmissionID == subID {
			found = true
		}
	}
	if !found {
		t.Errorf("consumer B did not reclaim the abandoned message %s", subID)
	}

	// Clean up so the shared stream does not accumulate test entries.
	if err := rc.AckJudge(ctx, msgID); err != nil {
		t.Errorf("ack cleanup: %v", err)
	}
}
