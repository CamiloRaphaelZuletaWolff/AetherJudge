package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/executor/internal/config"
	"github.com/caezu/arena/backend/services/executor/internal/sandbox"
)

// fakeSandbox records the request it received and returns canned results.
type fakeSandbox struct {
	got    sandbox.Request
	result sandbox.Result
	err    error
}

func (f *fakeSandbox) Execute(_ context.Context, req sandbox.Request) (sandbox.Result, error) {
	f.got = req
	return f.result, f.err
}

func testConfig() config.Config {
	return config.Config{
		ImageTag:        "latest",
		MaxConcurrent:   2,
		RunTimeout:      2 * time.Second,
		RunTimeoutMax:   10 * time.Second,
		RunMemoryMB:     128,
		RunMemoryMaxMB:  512,
		CompileTimeout:  20 * time.Second,
		CompileMemoryMB: 512,
		OutputLimitKB:   1024,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validRequest() *executorv1.ExecuteRequest {
	return &executorv1.ExecuteRequest{
		Code:           "print(42)",
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		Stdin:          "",
		ExpectedOutput: "42",
	}
}

func TestExecuteAccepted(t *testing.T) {
	t.Parallel()

	fake := &fakeSandbox{result: sandbox.Result{ExitCode: 0, Stdout: "42\n", Duration: 30 * time.Millisecond}}
	svc := New(fake, testConfig(), discardLogger())

	resp, err := svc.Execute(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("verdict = %v, want ACCEPTED", resp.GetVerdict())
	}
	if resp.GetTimeUsedMs() != 30 {
		t.Errorf("time_used_ms = %d, want 30", resp.GetTimeUsedMs())
	}
}

func TestExecuteVerdictMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result sandbox.Result
		want   executorv1.Verdict
	}{
		{"wrong answer", sandbox.Result{ExitCode: 0, Stdout: "41"}, executorv1.Verdict_VERDICT_WRONG_ANSWER},
		{"runtime error", sandbox.Result{ExitCode: 1}, executorv1.Verdict_VERDICT_RUNTIME_ERROR},
		{"timeout", sandbox.Result{TimedOut: true, ExitCode: -1}, executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED},
		{"oom", sandbox.Result{OOMKilled: true, ExitCode: 137}, executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(&fakeSandbox{result: tt.result}, testConfig(), discardLogger())
			resp, err := svc.Execute(context.Background(), validRequest())
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if resp.GetVerdict() != tt.want {
				t.Errorf("verdict = %v, want %v", resp.GetVerdict(), tt.want)
			}
		})
	}
}

func TestExecuteCompilationError(t *testing.T) {
	t.Parallel()

	fake := &fakeSandbox{result: sandbox.Result{CompileFailed: true, CompileOutput: "main.cpp:1: error: expected ';'"}}
	svc := New(fake, testConfig(), discardLogger())

	req := validRequest()
	req.Language = executorv1.Language_LANGUAGE_CPP

	resp, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_COMPILATION_ERROR {
		t.Errorf("verdict = %v, want COMPILATION_ERROR", resp.GetVerdict())
	}
	if !strings.Contains(resp.GetCompileOutput(), "expected ';'") {
		t.Errorf("compile_output = %q, want compiler diagnostics", resp.GetCompileOutput())
	}
}

func TestExecuteSandboxFailureReturnsInternalErrorVerdict(t *testing.T) {
	t.Parallel()

	fake := &fakeSandbox{err: errors.New("daemon unreachable")}
	svc := New(fake, testConfig(), discardLogger())

	resp, err := svc.Execute(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Execute returned gRPC error %v, want in-band INTERNAL_ERROR verdict", err)
	}
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_INTERNAL_ERROR {
		t.Errorf("verdict = %v, want INTERNAL_ERROR", resp.GetVerdict())
	}
}

func TestExecuteValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*executorv1.ExecuteRequest)
	}{
		{"unspecified language", func(r *executorv1.ExecuteRequest) { r.Language = executorv1.Language_LANGUAGE_UNSPECIFIED }},
		{"empty code", func(r *executorv1.ExecuteRequest) { r.Code = "   \n " }},
		{"oversized code", func(r *executorv1.ExecuteRequest) { r.Code = strings.Repeat("a", maxCodeBytes+1) }},
		{"oversized stdin", func(r *executorv1.ExecuteRequest) { r.Stdin = strings.Repeat("a", maxStdinBytes+1) }},
		{"oversized expected", func(r *executorv1.ExecuteRequest) { r.ExpectedOutput = strings.Repeat("a", maxExpectedBytes+1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(&fakeSandbox{}, testConfig(), discardLogger())
			req := validRequest()
			tt.mutate(req)

			_, err := svc.Execute(context.Background(), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("Execute error code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
			}
		})
	}
}

func TestExecuteClampsLimits(t *testing.T) {
	t.Parallel()

	fake := &fakeSandbox{result: sandbox.Result{ExitCode: 0, Stdout: "42"}}
	svc := New(fake, testConfig(), discardLogger())

	req := validRequest()
	req.TimeLimitMs = 60_000  // above 10s hard cap
	req.MemoryLimitMb = 4_096 // above 512MB hard cap

	if _, err := svc.Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fake.got.RunTimeout != 10*time.Second {
		t.Errorf("RunTimeout = %v, want clamped to 10s", fake.got.RunTimeout)
	}
	if want := int64(512 << 20); fake.got.Run.MemoryBytes != want {
		t.Errorf("Run.MemoryBytes = %d, want clamped to %d", fake.got.Run.MemoryBytes, want)
	}
}

func TestExecuteDefaultsLimitsWhenZero(t *testing.T) {
	t.Parallel()

	fake := &fakeSandbox{result: sandbox.Result{ExitCode: 0, Stdout: "42"}}
	svc := New(fake, testConfig(), discardLogger())

	if _, err := svc.Execute(context.Background(), validRequest()); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fake.got.RunTimeout != 2*time.Second {
		t.Errorf("RunTimeout = %v, want server default 2s", fake.got.RunTimeout)
	}
	if want := int64(128 << 20); fake.got.Run.MemoryBytes != want {
		t.Errorf("Run.MemoryBytes = %d, want server default %d", fake.got.Run.MemoryBytes, want)
	}
	if fake.got.Compile.MemoryBytes != int64(512<<20) {
		t.Errorf("Compile.MemoryBytes = %d, want %d", fake.got.Compile.MemoryBytes, int64(512<<20))
	}
}

// blockingSandbox holds executions until released, to test admission control.
type blockingSandbox struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingSandbox) Execute(ctx context.Context, _ sandbox.Request) (sandbox.Result, error) {
	b.started <- struct{}{}
	select {
	case <-b.release:
		return sandbox.Result{ExitCode: 0}, nil
	case <-ctx.Done():
		return sandbox.Result{}, ctx.Err()
	}
}

func TestExecuteAdmissionRespectsContext(t *testing.T) {
	t.Parallel()

	blocking := &blockingSandbox{started: make(chan struct{}, 2), release: make(chan struct{})}
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	svc := New(blocking, cfg, discardLogger())

	// Occupy the single slot.
	go func() {
		_, _ = svc.Execute(context.Background(), validRequest())
	}()
	<-blocking.started

	// Second request queues; its context expires while waiting.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := svc.Execute(ctx, validRequest())
	if status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("queued Execute error code = %v, want DeadlineExceeded (err: %v)", status.Code(err), err)
	}

	close(blocking.release)
}
