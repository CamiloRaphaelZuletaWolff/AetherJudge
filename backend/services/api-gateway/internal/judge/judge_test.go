package judge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/events"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

// fakeQueue is an in-memory stand-in for the Redis Streams queue. process()
// never touches it (the consumer loop does), so most methods are no-ops; the
// fields back the Enqueue tests.
type fakeQueue struct {
	mu       sync.Mutex
	depth    int64
	enqueued []enqueuedItem
}

type enqueuedItem struct {
	id          uuid.UUID
	traceParent string
}

func (q *fakeQueue) EnsureJudgeGroup(context.Context) error { return nil }

func (q *fakeQueue) EnqueueJudge(_ context.Context, id uuid.UUID, traceParent string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueued = append(q.enqueued, enqueuedItem{id: id, traceParent: traceParent})
	return nil
}

func (q *fakeQueue) ReadJudge(context.Context, string, int64, time.Duration) ([]redisx.QueueItem, error) {
	return nil, nil
}

func (q *fakeQueue) ClaimStaleJudge(context.Context, string, time.Duration, int64, int64) ([]redisx.QueueItem, []redisx.QueueItem, error) {
	return nil, nil, nil
}

func (q *fakeQueue) AckJudge(context.Context, string) error { return nil }

func (q *fakeQueue) JudgeQueueDepth(context.Context) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.depth, nil
}

func (q *fakeQueue) items() []enqueuedItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]enqueuedItem(nil), q.enqueued...)
}

// fakeLeaderboard records write-through cache updates.
type fakeLeaderboard struct {
	mu   sync.Mutex
	sets int
}

func (f *fakeLeaderboard) SetLeaderboardEntry(context.Context, uuid.UUID, redisx.LeaderboardMember) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets++
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeExecutor scripts per-call responses.
type fakeExecutor struct {
	mu        sync.Mutex
	responses []*executorv1.ExecuteResponse
	err       error
	calls     int
}

func (f *fakeExecutor) Execute(_ context.Context, _ *executorv1.ExecuteRequest, _ ...grpc.CallOption) (*executorv1.ExecuteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	resp := f.responses[min(f.calls, len(f.responses)-1)]
	f.calls++
	return resp, nil
}

// fakeStore is an in-memory Store covering exactly what the judge touches.
type fakeStore struct {
	mu          sync.Mutex
	sub         db.Submission
	user        db.User
	problem     db.Problem
	cases       []db.TestCase
	verdict     string
	timeUsed    int
	scored      bool
	firstSolve  bool
	standings   []db.LeaderboardEntry
	pendingErrs bool
}

func (f *fakeStore) GetSubmission(context.Context, uuid.UUID) (db.Submission, error) {
	return f.sub, nil
}

func (f *fakeStore) GetUserByID(context.Context, uuid.UUID) (db.User, error) {
	return f.user, nil
}

func (f *fakeStore) GetProblemByOrd(context.Context, uuid.UUID, int) (db.Problem, error) {
	return f.problem, nil
}

func (f *fakeStore) ListProblems(context.Context, uuid.UUID) ([]db.Problem, error) {
	return []db.Problem{f.problem}, nil
}

func (f *fakeStore) ListTestCases(context.Context, uuid.UUID) ([]db.TestCase, error) {
	return f.cases, nil
}

func (f *fakeStore) MarkSubmissionRunning(context.Context, uuid.UUID) error { return nil }

func (f *fakeStore) FinishSubmission(_ context.Context, _ uuid.UUID, verdict string, timeUsedMs int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verdict, f.timeUsed = verdict, timeUsedMs
	return nil
}

func (f *fakeStore) ListPendingSubmissionIDs(context.Context) ([]uuid.UUID, error) {
	if f.pendingErrs {
		return nil, errors.New("boom")
	}
	return nil, nil
}

func (f *fakeStore) ApplyAccepted(context.Context, db.Submission, time.Time) (db.ScoreResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scored = true
	return db.ScoreResult{FirstSolve: f.firstSolve, Entry: db.LeaderboardEntry{Solved: 1}}, nil
}

func (f *fakeStore) GetLeaderboard(context.Context, uuid.UUID, int) ([]db.LeaderboardEntry, error) {
	return f.standings, nil
}

// fakeBroadcaster records published events.
type fakeBroadcaster struct {
	mu       sync.Mutex
	messages [][]byte
}

func (f *fakeBroadcaster) Publish(_ context.Context, _ string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, payload)
	return nil
}

func (f *fakeBroadcaster) eventTypes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, m := range f.messages {
		var env events.Envelope
		if err := json.Unmarshal(m, &env); err == nil {
			out = append(out, env.Type)
		}
	}
	return out
}

func newTestStore() *fakeStore {
	problemID := uuid.New()
	return &fakeStore{
		sub: db.Submission{
			ID: uuid.New(), UserID: uuid.New(), ProblemID: problemID,
			ContestID: uuid.New(), Language: "python", Status: "queued",
			SubmittedAt: time.Now(),
		},
		user:    db.User{Username: "alice"},
		problem: db.Problem{ID: problemID, Ord: 1, TimeLimitMs: 2000, MemoryLimitMB: 128},
		cases: []db.TestCase{
			{Ord: 1, Stdin: "1", ExpectedOutput: "2"},
			{Ord: 2, Stdin: "2", ExpectedOutput: "4"},
			{Ord: 3, Stdin: "3", ExpectedOutput: "6"},
		},
		firstSolve: true,
		standings:  []db.LeaderboardEntry{{Username: "alice", Solved: 1, PenaltyS: 60}},
	}
}

func newServiceWith(store *fakeStore, exec *fakeExecutor, bc *fakeBroadcaster, q Queue, lb LeaderboardCache, cfg Config) *Service {
	return New(store, exec, q, lb, bc, cfg, discardLogger())
}

func newService(store *fakeStore, exec *fakeExecutor, bc *fakeBroadcaster) *Service {
	return newServiceWith(store, exec, bc, &fakeQueue{}, &fakeLeaderboard{}, Config{Workers: 1, ConsumerName: "test"})
}

func accepted(ms uint32) *executorv1.ExecuteResponse {
	return &executorv1.ExecuteResponse{Verdict: executorv1.Verdict_VERDICT_ACCEPTED, TimeUsedMs: ms}
}

func TestProcessAllCasesAccepted(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	exec := &fakeExecutor{responses: []*executorv1.ExecuteResponse{accepted(50), accepted(120), accepted(80)}}
	bc := &fakeBroadcaster{}

	_, _ = newService(store, exec, bc).process(context.Background(), job{id: store.sub.ID})

	if store.verdict != "accepted" {
		t.Errorf("verdict = %q, want accepted", store.verdict)
	}
	if store.timeUsed != 120 {
		t.Errorf("timeUsed = %d, want 120 (slowest case)", store.timeUsed)
	}
	if exec.calls != 3 {
		t.Errorf("executor calls = %d, want 3", exec.calls)
	}
	if !store.scored {
		t.Error("accepted submission was not scored")
	}

	types := bc.eventTypes()
	wantSeq := []string{events.TypeSubmissionUpdate, events.TypeLeaderboardUpdate, events.TypeSubmissionUpdate}
	if len(types) != len(wantSeq) {
		t.Fatalf("published %d events (%v), want %d", len(types), types, len(wantSeq))
	}
	for i, want := range wantSeq {
		if types[i] != want {
			t.Errorf("event[%d] = %s, want %s", i, types[i], want)
		}
	}
}

func TestProcessStopsAtFirstFailure(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	exec := &fakeExecutor{responses: []*executorv1.ExecuteResponse{
		accepted(50),
		{Verdict: executorv1.Verdict_VERDICT_WRONG_ANSWER, TimeUsedMs: 70},
		accepted(90),
	}}
	bc := &fakeBroadcaster{}

	_, _ = newService(store, exec, bc).process(context.Background(), job{id: store.sub.ID})

	if store.verdict != "wrong_answer" {
		t.Errorf("verdict = %q, want wrong_answer", store.verdict)
	}
	if exec.calls != 2 {
		t.Errorf("executor calls = %d, want 2 (stop at first failure)", exec.calls)
	}
	if store.scored {
		t.Error("non-accepted submission was scored")
	}
}

func TestProcessExecutorFailureRetries(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	exec := &fakeExecutor{err: errors.New("executor down")}
	bc := &fakeBroadcaster{}

	// Executor-down is infrastructure, not a verdict: process must signal
	// retry and leave the submission unfinished (re-judged when the executor
	// recovers; the MaxDeliveries backstop dead-letters a permanent failure).
	retry, err := newService(store, exec, bc).process(context.Background(), job{id: store.sub.ID})

	if !retry {
		t.Error("executor failure should be retryable, not terminal")
	}
	if err == nil {
		t.Error("expected an error describing the executor failure")
	}
	if store.verdict != "" {
		t.Errorf("verdict = %q, want empty (submission left for retry, not burned)", store.verdict)
	}
}

func TestProcessNoTestCasesIsInternalError(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	store.cases = nil
	bc := &fakeBroadcaster{}

	_, _ = newService(store, &fakeExecutor{responses: []*executorv1.ExecuteResponse{accepted(1)}}, bc).
		process(context.Background(), job{id: store.sub.ID})

	if store.verdict != "internal_error" {
		t.Errorf("verdict = %q, want internal_error", store.verdict)
	}
}

func TestProcessSkipsAlreadyJudged(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	store.sub.Status = "done"
	exec := &fakeExecutor{responses: []*executorv1.ExecuteResponse{accepted(1)}}

	_, _ = newService(store, exec, &fakeBroadcaster{}).process(context.Background(), job{id: store.sub.ID})

	if exec.calls != 0 {
		t.Errorf("executor calls = %d, want 0 for already-judged submission", exec.calls)
	}
}

func TestDuplicateAcceptDoesNotBroadcastLeaderboard(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	store.firstSolve = false
	bc := &fakeBroadcaster{}

	_, _ = newService(store, &fakeExecutor{responses: []*executorv1.ExecuteResponse{accepted(10)}}, bc).
		process(context.Background(), job{id: store.sub.ID})

	for _, typ := range bc.eventTypes() {
		if typ == events.TypeLeaderboardUpdate {
			t.Error("duplicate accept broadcast a leaderboard update")
		}
	}
}

func TestEnqueueBackpressure(t *testing.T) {
	t.Parallel()

	q := &fakeQueue{}
	svc := newServiceWith(newTestStore(), &fakeExecutor{}, &fakeBroadcaster{}, q, &fakeLeaderboard{},
		Config{QueueDepthLimit: 2, ConsumerName: "test"})

	q.depth = 0
	if !svc.Enqueue(context.Background(), uuid.New()) {
		t.Fatal("rejected an enqueue below the depth limit")
	}
	q.depth = 1
	if !svc.Enqueue(context.Background(), uuid.New()) {
		t.Fatal("rejected an enqueue below the depth limit")
	}
	q.depth = 2 // at the limit
	if svc.Enqueue(context.Background(), uuid.New()) {
		t.Error("accepted an enqueue at/over the depth limit")
	}
}

func TestVerdictStringCoversAll(t *testing.T) {
	t.Parallel()

	want := map[executorv1.Verdict]string{
		executorv1.Verdict_VERDICT_ACCEPTED:              "accepted",
		executorv1.Verdict_VERDICT_WRONG_ANSWER:          "wrong_answer",
		executorv1.Verdict_VERDICT_RUNTIME_ERROR:         "runtime_error",
		executorv1.Verdict_VERDICT_COMPILATION_ERROR:     "compilation_error",
		executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED:   "time_limit_exceeded",
		executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED: "memory_limit_exceeded",
		executorv1.Verdict_VERDICT_INTERNAL_ERROR:        "internal_error",
		executorv1.Verdict_VERDICT_UNSPECIFIED:           "internal_error",
	}
	for v, s := range want {
		if got := verdictString(v); got != s {
			t.Errorf("verdictString(%v) = %q, want %q", v, got, s)
		}
	}

	for _, lang := range []string{"cpp", "python", "go"} {
		if protoLanguage(lang) == executorv1.Language_LANGUAGE_UNSPECIFIED {
			t.Errorf("protoLanguage(%q) = UNSPECIFIED", lang)
		}
	}
	if protoLanguage("rust") != executorv1.Language_LANGUAGE_UNSPECIFIED {
		t.Error("protoLanguage(rust) should be UNSPECIFIED")
	}

	if !strings.Contains(verdictString(executorv1.Verdict_VERDICT_ACCEPTED), "accepted") {
		t.Error("sanity")
	}
}
