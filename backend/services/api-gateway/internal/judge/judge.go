// Package judge runs the gateway side of submission judging: workers that
// consume a durable Redis Streams queue, drive the executor over gRPC across
// every test case, persist verdicts, apply scoring, and publish live events.
//
// Durability/recovery model (ADR-0011): the submission row in PostgreSQL is
// the source of truth; the stream is at-least-once dispatch. A crashed
// worker's un-acked messages are reclaimed (ClaimStaleJudge); a flushed
// Redis is recovered by reconciling unfinished rows from PG on startup.
// Because delivery is at-least-once, judging is idempotent: an already-done
// submission is a no-op, and scoring is ON CONFLICT DO NOTHING.
package judge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/events"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

// Store is the persistence surface the judge needs; db.Store satisfies it.
type Store interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (db.Submission, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetProblemByOrd(ctx context.Context, contestID uuid.UUID, ord int) (db.Problem, error)
	ListProblems(ctx context.Context, contestID uuid.UUID) ([]db.Problem, error)
	ListTestCases(ctx context.Context, problemID uuid.UUID) ([]db.TestCase, error)
	MarkSubmissionRunning(ctx context.Context, id uuid.UUID) error
	FinishSubmission(ctx context.Context, id uuid.UUID, verdict string, timeUsedMs int) error
	ListPendingSubmissionIDs(ctx context.Context) ([]uuid.UUID, error)
	ApplyAccepted(ctx context.Context, sub db.Submission, acceptedAt time.Time) (db.ScoreResult, error)
	GetLeaderboard(ctx context.Context, contestID uuid.UUID, limit int) ([]db.LeaderboardEntry, error)
}

// Queue is the durable dispatch surface; redisx.Client satisfies it.
type Queue interface {
	EnsureJudgeGroup(ctx context.Context) error
	EnqueueJudge(ctx context.Context, submissionID uuid.UUID, traceParent string) error
	ReadJudge(ctx context.Context, consumer string, count int64, block time.Duration) ([]redisx.QueueItem, error)
	ClaimStaleJudge(ctx context.Context, consumer string, minIdle time.Duration, count, maxDeliveries int64) (claimed, poison []redisx.QueueItem, err error)
	AckJudge(ctx context.Context, messageID string) error
	JudgeQueueDepth(ctx context.Context) (int64, error)
}

// LeaderboardCache is the write-through read-cache; redisx.Client satisfies it.
type LeaderboardCache interface {
	SetLeaderboardEntry(ctx context.Context, contestID uuid.UUID, m redisx.LeaderboardMember) error
}

// Broadcaster publishes contest events; redisx.Client satisfies it.
type Broadcaster interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}

// leaderboardBroadcastSize bounds how many rows ride a leaderboard.update event.
const leaderboardBroadcastSize = 50

// perCaseOverhead covers compile time and sandbox lifecycle on top of the
// problem's own run limit when budgeting the executor call deadline.
const perCaseOverhead = 45 * time.Second

// Config tunes the queue consumer behavior.
type Config struct {
	// Workers is how many consumer goroutines this process runs.
	Workers int
	// ConsumerName identifies this process in the consumer group (a pod name
	// or host+pid). Per-goroutine suffixes are appended.
	ConsumerName string
	// QueueDepthLimit rejects new submissions (HTTP 503) once the stream holds
	// this many items; 0 disables the check.
	QueueDepthLimit int64
	// BlockTimeout bounds a single blocking read before the loop re-checks ctx.
	BlockTimeout time.Duration
	// ClaimMinIdle is how long a message must sit un-acked before another
	// worker may reclaim it (i.e. assume the original consumer died).
	ClaimMinIdle time.Duration
	// MaxDeliveries dead-letters a message (verdict internal_error) once it has
	// been delivered this many times — a poison-message backstop.
	MaxDeliveries int64
}

func (c Config) withDefaults() Config {
	if c.BlockTimeout <= 0 {
		c.BlockTimeout = 5 * time.Second
	}
	if c.ClaimMinIdle <= 0 {
		c.ClaimMinIdle = 60 * time.Second
	}
	if c.MaxDeliveries <= 0 {
		c.MaxDeliveries = 5
	}
	if c.ConsumerName == "" {
		c.ConsumerName = "judge"
	}
	return c
}

// job is one judging task. link carries the enqueuing request's span context
// (decoded from the stream's traceparent) so the judge trace links back
// across the async hop; a zero link (reconciled/untraced item) becomes a
// plain trace root.
type job struct {
	id   uuid.UUID
	link trace.SpanContext
}

// Service judges submissions consumed from the durable queue.
type Service struct {
	store    Store
	executor executorv1.ExecutorServiceClient
	queue    Queue
	lb       LeaderboardCache
	bc       Broadcaster
	cfg      Config
	log      *slog.Logger
	tracer   trace.Tracer

	wg sync.WaitGroup
}

// Dial connects the executor gRPC client with client-side round-robin load
// balancing across all backends a (headless) DNS name resolves to, so the
// gateway fans Execute calls across executor replicas (ADR-0011). The
// connection is lazy; failures surface per-call.
func Dial(addr string) (executorv1.ExecutorServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
		// Client spans + W3C propagation toward the executor; no-op when
		// tracing is disabled.
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("judge: dial executor at %s: %w", addr, err)
	}
	return executorv1.NewExecutorServiceClient(conn), conn, nil
}

// New builds a judge service.
func New(store Store, executor executorv1.ExecutorServiceClient, queue Queue, lb LeaderboardCache, bc Broadcaster, cfg Config, log *slog.Logger) *Service {
	return &Service{
		store:    store,
		executor: executor,
		queue:    queue,
		lb:       lb,
		bc:       bc,
		cfg:      cfg.withDefaults(),
		log:      log,
		tracer:   otel.Tracer("github.com/caezu/arena/backend/services/api-gateway/internal/judge"),
	}
}

// Enqueue publishes a submission to the durable queue. It returns false when
// the queue is saturated (the caller signals backpressure with HTTP 503).
// The caller's trace context rides along as a W3C traceparent.
func (s *Service) Enqueue(ctx context.Context, id uuid.UUID) bool {
	if limit := s.cfg.QueueDepthLimit; limit > 0 {
		if depth, err := s.queue.JudgeQueueDepth(ctx); err == nil && depth >= limit {
			s.log.WarnContext(ctx, "judge queue saturated", "depth", depth, "limit", limit)
			return false
		}
	}
	if err := s.queue.EnqueueJudge(ctx, id, traceParentOf(ctx)); err != nil {
		s.log.ErrorContext(ctx, "enqueue submission", "submission_id", id, "error", err)
		return false
	}
	return true
}

// StartConsumers ensures the consumer group, reconciles any work orphaned by
// a Redis flush, and launches the worker + reclaim goroutines. Workers == 0
// starts nothing (a producer-only web tier). Call Wait to drain.
func (s *Service) StartConsumers(ctx context.Context) error {
	if s.cfg.Workers <= 0 {
		return nil // producer-only: it XADDs; some worker process owns the group
	}
	if err := s.queue.EnsureJudgeGroup(ctx); err != nil {
		return fmt.Errorf("judge: ensure group: %w", err)
	}
	if err := s.reconcile(ctx); err != nil {
		// Reconciliation is best-effort recovery; log and continue rather than
		// refuse to start judging.
		s.log.ErrorContext(ctx, "reconcile pending submissions", "error", err)
	}

	for i := range s.cfg.Workers {
		consumer := fmt.Sprintf("%s-%d", s.cfg.ConsumerName, i)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.consumeLoop(ctx, consumer)
		}()
	}
	// One reclaim goroutine per process recovers messages abandoned by dead
	// consumers (here or on other replicas).
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.reclaimLoop(ctx, s.cfg.ConsumerName+"-reclaim")
	}()
	return nil
}

// Wait blocks until every consumer/reclaim goroutine has exited.
func (s *Service) Wait() { s.wg.Wait() }

// StartQueueDepthSampler periodically publishes the stream depth to the
// arena_judge_queue_depth gauge. Both the web and worker tiers run it so the
// metric is present wherever Prometheus scrapes.
func (s *Service) StartQueueDepthSampler(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			if !sleepCtx(ctx, 5*time.Second) {
				return
			}
			if depth, err := s.queue.JudgeQueueDepth(ctx); err == nil {
				queueDepth.Set(float64(depth))
			}
		}
	}()
}

// reconcile re-enqueues unfinished submissions when the stream is empty —
// the recovery path after Redis is lost (ADR-0004). When the stream is
// non-empty, in-flight work is either ready or recoverable via reclaim, so
// reconciliation would only create wasteful duplicates and is skipped.
func (s *Service) reconcile(ctx context.Context) error {
	depth, err := s.queue.JudgeQueueDepth(ctx)
	if err != nil {
		return fmt.Errorf("judge: queue depth: %w", err)
	}
	if depth > 0 {
		return nil
	}
	pending, err := s.store.ListPendingSubmissionIDs(ctx)
	if err != nil {
		return fmt.Errorf("judge: list pending: %w", err)
	}
	for _, id := range pending {
		if err := s.queue.EnqueueJudge(ctx, id, ""); err != nil {
			return fmt.Errorf("judge: re-enqueue %s: %w", id, err)
		}
	}
	if len(pending) > 0 {
		s.log.InfoContext(ctx, "reconciled pending submissions into empty queue", "count", len(pending))
	}
	return nil
}

func (s *Service) consumeLoop(ctx context.Context, consumer string) {
	for {
		if ctx.Err() != nil {
			return
		}
		items, err := s.queue.ReadJudge(ctx, consumer, 1, s.cfg.BlockTimeout)
		if err != nil {
			s.log.ErrorContext(ctx, "read judge queue", "consumer", consumer, "error", err)
			if !sleepCtx(ctx, time.Second) {
				return
			}
			continue
		}
		for _, item := range items {
			s.handle(ctx, item)
		}
	}
}

func (s *Service) reclaimLoop(ctx context.Context, consumer string) {
	// Probe a little more often than the idle threshold so abandoned work is
	// picked up promptly without hammering Redis.
	interval := s.cfg.ClaimMinIdle / 2
	if interval < time.Second {
		interval = time.Second
	}
	for {
		if !sleepCtx(ctx, interval) {
			return
		}
		claimed, poison, err := s.queue.ClaimStaleJudge(ctx, consumer, s.cfg.ClaimMinIdle, 32, s.cfg.MaxDeliveries)
		if err != nil {
			s.log.ErrorContext(ctx, "claim stale judge messages", "error", err)
			continue
		}
		for _, item := range poison {
			s.deadLetter(ctx, item)
		}
		for _, item := range claimed {
			s.log.InfoContext(ctx, "reclaimed abandoned submission", "submission_id", item.SubmissionID)
			s.handle(ctx, item)
		}
	}
}

// handle judges one queue item and acks it on a terminal outcome. A retryable
// failure (infrastructure down) is left un-acked so it is reclaimed and
// retried later; the MaxDeliveries backstop dead-letters anything that never
// succeeds.
func (s *Service) handle(ctx context.Context, item redisx.QueueItem) {
	if item.SubmissionID == uuid.Nil {
		// Unparseable entry: drop it rather than block the group forever.
		s.ack(ctx, item.MessageID)
		return
	}
	retry, err := s.process(ctx, job{id: item.SubmissionID, link: spanContextFrom(item.TraceParent)})
	if retry {
		s.log.WarnContext(ctx, "judge deferred for retry", "submission_id", item.SubmissionID, "error", err)
		return // leave un-acked → reclaimed after ClaimMinIdle
	}
	s.ack(ctx, item.MessageID)
}

func (s *Service) ack(ctx context.Context, messageID string) {
	if err := s.queue.AckJudge(ctx, messageID); err != nil {
		s.log.ErrorContext(ctx, "ack judge message", "message_id", messageID, "error", err)
	}
}

// deadLetter terminates a poison submission: it has exceeded MaxDeliveries
// (it keeps crashing its consumer), so record internal_error — visible and
// never silently lost — and ack it out of the queue.
func (s *Service) deadLetter(ctx context.Context, item redisx.QueueItem) {
	s.log.ErrorContext(ctx, "dead-lettering poison submission", "submission_id", item.SubmissionID)
	if item.SubmissionID != uuid.Nil {
		if sub, err := s.store.GetSubmission(ctx, item.SubmissionID); err == nil && sub.Status != "done" {
			user, _ := s.store.GetUserByID(ctx, sub.UserID)
			s.finish(ctx, sub, user, 0, "internal_error", 0)
		}
	}
	s.ack(ctx, item.MessageID)
}

// process judges one submission. It returns retry=true when judging could not
// reach a verdict because of an infrastructure failure (DB or executor
// unreachable): the message is left un-acked for a later retry rather than
// burning the submission. Permanent problems (missing data) finish as
// internal_error and return retry=false.
func (s *Service) process(ctx context.Context, j job) (retry bool, err error) {
	var opts []trace.SpanStartOption
	if j.link.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: j.link}))
	}
	ctx, span := s.tracer.Start(ctx, "judge.process", opts...)
	defer span.End()
	span.SetAttributes(attribute.String("arena.submission_id", j.id.String()))

	start := time.Now()
	log := s.log.With("submission_id", j.id)

	sub, err := s.store.GetSubmission(ctx, j.id)
	if errors.Is(err, db.ErrNotFound) {
		log.ErrorContext(ctx, "submission not found; dropping", "error", err)
		span.SetStatus(codes.Error, "submission not found")
		return false, nil // terminal: nothing to judge
	}
	if err != nil {
		span.SetStatus(codes.Error, "load submission failed")
		return true, fmt.Errorf("load submission: %w", err) // transient: retry
	}
	if sub.Status == "done" {
		return false, nil // already judged (idempotent re-delivery)
	}
	span.SetAttributes(attribute.String("arena.language", sub.Language))

	user, err := s.store.GetUserByID(ctx, sub.UserID)
	if err != nil {
		log.ErrorContext(ctx, "load submitter", "error", err)
		s.finish(ctx, sub, db.User{}, 0, "internal_error", 0)
		return false, nil
	}

	problem, testCases, err := s.loadProblem(ctx, sub)
	if err != nil {
		log.ErrorContext(ctx, "load problem", "error", err)
		s.finish(ctx, sub, user, 0, "internal_error", 0)
		return false, nil
	}

	if err := s.store.MarkSubmissionRunning(ctx, sub.ID); err != nil {
		log.ErrorContext(ctx, "mark running", "error", err)
	}
	s.publishSubmission(ctx, sub, user, problem.Ord, "running", "", 0)

	verdict, timeUsedMs, infraErr := s.runCases(ctx, sub, problem, testCases, log)
	if infraErr != nil {
		// The executor was unreachable/failed — retry when it recovers rather
		// than judging the submission internal_error. The row stays 'running'
		// and is idempotently re-judged on redelivery.
		span.SetStatus(codes.Error, "executor unavailable")
		return true, infraErr
	}

	s.finish(ctx, sub, user, problem.Ord, verdict, timeUsedMs)

	judgeDuration.WithLabelValues(sub.Language).Observe(time.Since(start).Seconds())
	span.SetAttributes(attribute.String("arena.verdict", verdict))
	log.InfoContext(ctx, "submission judged", "verdict", verdict, "time_used_ms", timeUsedMs)
	return false, nil
}

// runCases executes every test case, stopping at the first non-accepted
// verdict. A non-nil error means the executor itself failed (retryable);
// callers must not treat that as a verdict.
func (s *Service) runCases(ctx context.Context, sub db.Submission, problem db.Problem, testCases []db.TestCase, log *slog.Logger) (verdict string, timeUsedMs int, err error) {
	maxTime := 0
	for _, tc := range testCases {
		caseCtx, cancel := context.WithTimeout(ctx, time.Duration(problem.TimeLimitMs)*time.Millisecond+perCaseOverhead)
		resp, execErr := s.executor.Execute(caseCtx, &executorv1.ExecuteRequest{
			Code:           sub.Code,
			Language:       protoLanguage(sub.Language),
			Stdin:          tc.Stdin,
			ExpectedOutput: tc.ExpectedOutput,
			TimeLimitMs:    uint32(problem.TimeLimitMs),   //nolint:gosec // bounded 100..10000 by schema CHECK
			MemoryLimitMb:  uint32(problem.MemoryLimitMB), //nolint:gosec // bounded 16..512 by schema CHECK
		})
		cancel()
		if execErr != nil {
			log.ErrorContext(ctx, "executor call failed", "case", tc.Ord, "error", execErr)
			return "", maxTime, fmt.Errorf("executor execute: %w", execErr)
		}

		if used := int(resp.GetTimeUsedMs()); used > maxTime {
			maxTime = used
		}
		if v := verdictString(resp.GetVerdict()); v != "accepted" {
			log.InfoContext(ctx, "test case failed", "case", tc.Ord, "verdict", v)
			return v, maxTime, nil
		}
	}
	return "accepted", maxTime, nil
}

// finish persists the verdict, applies scoring on accepts, and broadcasts.
func (s *Service) finish(ctx context.Context, sub db.Submission, user db.User, problemOrd int, verdict string, timeUsedMs int) {
	jobsTotal.WithLabelValues(verdict).Inc()

	if err := s.store.FinishSubmission(ctx, sub.ID, verdict, timeUsedMs); err != nil {
		s.log.ErrorContext(ctx, "persist verdict", "submission_id", sub.ID, "error", err)
		return
	}

	if verdict == "accepted" {
		s.score(ctx, sub, user.Username)
	}

	s.publishSubmission(ctx, sub, user, problemOrd, "done", verdict, timeUsedMs)
}

// score updates durable standings, write-through-updates the Redis read
// cache, and broadcasts the new leaderboard.
func (s *Service) score(ctx context.Context, sub db.Submission, username string) {
	res, err := s.store.ApplyAccepted(ctx, sub, time.Now())
	if err != nil {
		s.log.ErrorContext(ctx, "apply scoring", "submission_id", sub.ID, "error", err)
		return
	}
	if !res.FirstSolve {
		return
	}

	// Write-through the cache so the REST leaderboard reflects the solve
	// without a PG round-trip. Best-effort: a cache error is logged, never
	// fatal — the next read rebuilds from PG (ADR-0004).
	if err := s.lb.SetLeaderboardEntry(ctx, sub.ContestID, redisx.LeaderboardMember{
		UserID: sub.UserID, Username: username, Solved: res.Entry.Solved, PenaltyS: res.Entry.PenaltyS,
	}); err != nil {
		s.log.WarnContext(ctx, "leaderboard cache write", "error", err)
	}

	standings, err := s.store.GetLeaderboard(ctx, sub.ContestID, leaderboardBroadcastSize)
	if err != nil {
		s.log.ErrorContext(ctx, "load standings for broadcast", "error", err)
		return
	}
	rows := make([]events.LeaderboardRow, len(standings))
	for i, e := range standings {
		rows[i] = events.LeaderboardRow{Rank: i + 1, Username: e.Username, Solved: e.Solved, PenaltyS: e.PenaltyS}
	}
	s.publish(ctx, sub.ContestID, events.TypeLeaderboardUpdate, events.LeaderboardUpdate{Entries: rows})
}

func (s *Service) publishSubmission(ctx context.Context, sub db.Submission, user db.User, problemOrd int, status, verdict string, timeUsedMs int) {
	s.publish(ctx, sub.ContestID, events.TypeSubmissionUpdate, events.SubmissionUpdate{
		SubmissionID: sub.ID,
		Username:     user.Username,
		ProblemOrd:   problemOrd,
		Language:     sub.Language,
		Status:       status,
		Verdict:      verdict,
		TimeUsedMs:   timeUsedMs,
		SubmittedAt:  sub.SubmittedAt,
	})
}

func (s *Service) publish(ctx context.Context, contestID uuid.UUID, eventType string, payload any) {
	data, err := events.Marshal(eventType, payload)
	if err != nil {
		s.log.ErrorContext(ctx, "marshal event", "type", eventType, "error", err)
		return
	}
	if err := s.bc.Publish(ctx, redisx.ContestChannel(contestID), data); err != nil {
		s.log.ErrorContext(ctx, "publish event", "type", eventType, "error", err)
	}
}

func (s *Service) loadProblem(ctx context.Context, sub db.Submission) (db.Problem, []db.TestCase, error) {
	problems, err := s.store.ListProblems(ctx, sub.ContestID)
	if err != nil {
		return db.Problem{}, nil, err
	}
	for _, p := range problems {
		if p.ID == sub.ProblemID {
			testCases, err := s.store.ListTestCases(ctx, p.ID)
			if err != nil {
				return db.Problem{}, nil, err
			}
			if len(testCases) == 0 {
				return db.Problem{}, nil, fmt.Errorf("judge: problem %s has no test cases", p.ID)
			}
			return p, testCases, nil
		}
	}
	return db.Problem{}, nil, fmt.Errorf("judge: problem %s not found in contest %s", sub.ProblemID, sub.ContestID)
}

// traceParentOf serializes the active span context to a W3C traceparent so it
// can ride the stream to a (possibly different) worker process.
func traceParentOf(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"]
}

// spanContextFrom rebuilds the span context from a stored traceparent.
func spanContextFrom(traceParent string) trace.SpanContext {
	if traceParent == "" {
		return trace.SpanContext{}
	}
	carrier := propagation.MapCarrier{"traceparent": traceParent}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
	return trace.SpanContextFromContext(ctx)
}

// sleepCtx sleeps for d or until ctx is cancelled; it returns false when ctx
// was cancelled (the caller should stop).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
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

func verdictString(v executorv1.Verdict) string {
	switch v {
	case executorv1.Verdict_VERDICT_ACCEPTED:
		return "accepted"
	case executorv1.Verdict_VERDICT_WRONG_ANSWER:
		return "wrong_answer"
	case executorv1.Verdict_VERDICT_RUNTIME_ERROR:
		return "runtime_error"
	case executorv1.Verdict_VERDICT_COMPILATION_ERROR:
		return "compilation_error"
	case executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED:
		return "time_limit_exceeded"
	case executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED:
		return "memory_limit_exceeded"
	default:
		return "internal_error"
	}
}
