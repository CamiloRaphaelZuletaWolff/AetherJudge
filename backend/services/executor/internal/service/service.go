// Package service implements the ExecutorService gRPC contract: request
// validation, limit clamping, admission control, and mapping sandbox
// observations to verdicts.
package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/executor/internal/config"
	"github.com/caezu/arena/backend/services/executor/internal/judge"
	"github.com/caezu/arena/backend/services/executor/internal/lang"
	"github.com/caezu/arena/backend/services/executor/internal/sandbox"
)

// Request payload caps. Protocol policy, deliberately distinct from the
// sandbox resource limits: these reject abuse before a container exists.
const (
	maxCodeBytes     = 256 * 1024
	maxStdinBytes    = 1024 * 1024
	maxExpectedBytes = 1024 * 1024
)

// Phase CPU and pids policy. Compile gets headroom (g++ with the standard
// competitive-programming headers needs real memory and parallelism); run is
// the strict contest envelope from the requirements.
const (
	compileNanoCPUs = 2_000_000_000 // 2.0 CPU
	runNanoCPUs     = 500_000_000   // 0.5 CPU
	compilePids     = 256
	runPids         = 64
)

// Sandbox abstracts the Docker engine so the service is unit-testable
// without a daemon.
type Sandbox interface {
	Execute(ctx context.Context, req sandbox.Request) (sandbox.Result, error)
}

// Service implements executorv1.ExecutorServiceServer.
type Service struct {
	executorv1.UnimplementedExecutorServiceServer

	sandbox Sandbox
	cfg     config.Config
	sem     chan struct{}
	log     *slog.Logger
}

// New builds the service with a semaphore sized to cfg.MaxConcurrent.
func New(sb Sandbox, cfg config.Config, log *slog.Logger) *Service {
	return &Service{
		sandbox: sb,
		cfg:     cfg,
		sem:     make(chan struct{}, cfg.MaxConcurrent),
		log:     log,
	}
}

// Execute judges one submission against one test case.
//
// Error contract: malformed requests fail with InvalidArgument; executor
// infrastructure failures return VERDICT_INTERNAL_ERROR in-band (the caller
// should retry, never present it as a judgment).
func (s *Service) Execute(ctx context.Context, req *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
	spec, err := lang.ForLanguage(req.GetLanguage(), s.cfg.ImageTag)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported language %q", req.GetLanguage().String())
	}
	if strings.TrimSpace(req.GetCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "code must not be empty")
	}
	if len(req.GetCode()) > maxCodeBytes {
		return nil, status.Errorf(codes.InvalidArgument, "code exceeds %d bytes", maxCodeBytes)
	}
	if len(req.GetStdin()) > maxStdinBytes {
		return nil, status.Errorf(codes.InvalidArgument, "stdin exceeds %d bytes", maxStdinBytes)
	}
	if len(req.GetExpectedOutput()) > maxExpectedBytes {
		return nil, status.Errorf(codes.InvalidArgument, "expected_output exceeds %d bytes", maxExpectedBytes)
	}

	runTimeout := clampTimeout(req.GetTimeLimitMs(), s.cfg.RunTimeout, s.cfg.RunTimeoutMax)
	runMemoryMB := clampMemoryMB(req.GetMemoryLimitMb(), s.cfg.RunMemoryMB, s.cfg.RunMemoryMaxMB)

	// Admission control: bound concurrent sandboxes; give up if the caller's
	// deadline expires while queued.
	admissionStart := time.Now()
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	admissionWait.Observe(time.Since(admissionStart).Seconds())
	activeExecutions.Inc()
	defer activeExecutions.Dec()

	res, err := s.sandbox.Execute(ctx, sandbox.Request{
		Spec:  spec,
		Code:  req.GetCode(),
		Stdin: req.GetStdin(),
		Compile: sandbox.Resources{
			MemoryBytes: s.cfg.CompileMemoryMB << 20,
			NanoCPUs:    compileNanoCPUs,
			PidsLimit:   compilePids,
		},
		CompileTimeout: s.cfg.CompileTimeout,
		Run: sandbox.Resources{
			MemoryBytes: runMemoryMB << 20,
			NanoCPUs:    runNanoCPUs,
			PidsLimit:   runPids,
		},
		RunTimeout:       runTimeout,
		OutputLimitBytes: s.cfg.OutputLimitKB << 10,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "sandbox execution failed",
			"language", spec.Name,
			"error", err,
		)
		executionsTotal.WithLabelValues(spec.Name, verdictLabel(executorv1.Verdict_VERDICT_INTERNAL_ERROR)).Inc()
		return &executorv1.ExecuteResponse{
			Verdict: executorv1.Verdict_VERDICT_INTERNAL_ERROR,
		}, nil
	}

	if res.CompileFailed {
		executionsTotal.WithLabelValues(spec.Name, verdictLabel(executorv1.Verdict_VERDICT_COMPILATION_ERROR)).Inc()
		return &executorv1.ExecuteResponse{
			Verdict:       executorv1.Verdict_VERDICT_COMPILATION_ERROR,
			CompileOutput: res.CompileOutput,
		}, nil
	}

	verdict := judge.Verdict(judge.Result{
		ExitCode:  res.ExitCode,
		TimedOut:  res.TimedOut,
		OOMKilled: res.OOMKilled,
		Stdout:    res.Stdout,
	}, req.GetExpectedOutput())

	executionsTotal.WithLabelValues(spec.Name, verdictLabel(verdict)).Inc()
	s.log.InfoContext(ctx, "submission judged",
		"language", spec.Name,
		"verdict", verdict.String(),
		"duration_ms", res.Duration.Milliseconds(),
		"exit_code", res.ExitCode,
	)

	return &executorv1.ExecuteResponse{
		Verdict:    verdict,
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		ExitCode:   clampToInt32(res.ExitCode),
		TimeUsedMs: clampToUint32(res.Duration.Milliseconds()),
	}, nil
}

// clampTimeout resolves the client-requested limit (0 = server default)
// against the server's hard ceiling.
func clampTimeout(requestedMs uint32, def, ceiling time.Duration) time.Duration {
	if requestedMs == 0 {
		return def
	}
	requested := time.Duration(requestedMs) * time.Millisecond
	return min(requested, ceiling)
}

// clampMemoryMB resolves the client-requested limit (0 = server default)
// against the server's hard ceiling.
func clampMemoryMB(requestedMB uint32, def, ceiling int64) int64 {
	if requestedMB == 0 {
		return def
	}
	return min(int64(requestedMB), ceiling)
}

func clampToInt32(v int) int32 {
	if v > 2147483647 {
		return 2147483647
	}
	if v < -2147483648 {
		return -2147483648
	}
	return int32(v)
}

func clampToUint32(v int64) uint32 {
	if v < 0 {
		return 0
	}
	if v > int64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(v)
}
