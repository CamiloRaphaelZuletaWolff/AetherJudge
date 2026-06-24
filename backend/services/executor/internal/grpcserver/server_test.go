package grpcserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubExecutor lets tests control the registered ExecutorService behavior.
type stubExecutor struct {
	executorv1.UnimplementedExecutorServiceServer
	execute func(context.Context, *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error)
}

func (s *stubExecutor) Execute(ctx context.Context, req *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
	return s.execute(ctx, req)
}

// startBufconnServer serves a Server over an in-memory listener and returns
// a connected client plus a shutdown function.
func startBufconnServer(t *testing.T, executor executorv1.ExecutorServiceServer) (*grpc.ClientConn, func() error) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := New("unused", discardLogger(), executor)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		t.Fatalf("grpc.NewClient: %v", err)
	}

	shutdown := func() error {
		if err := conn.Close(); err != nil {
			t.Errorf("close client conn: %v", err)
		}
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("Serve did not return within 5s of context cancellation")
			return nil
		}
	}

	return conn, shutdown
}

func okExecutor() *stubExecutor {
	return &stubExecutor{
		execute: func(context.Context, *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
			return &executorv1.ExecuteResponse{Verdict: executorv1.Verdict_VERDICT_ACCEPTED}, nil
		},
	}
}

func TestHealthCheckReportsServing(t *testing.T) {
	t.Parallel()

	conn, shutdown := startBufconnServer(t, okExecutor())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.GetStatus())
	}

	if err := shutdown(); err != nil {
		t.Errorf("Serve returned error after graceful stop: %v", err)
	}
}

func TestExecutorServiceIsRegistered(t *testing.T) {
	t.Parallel()

	conn, shutdown := startBufconnServer(t, okExecutor())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := executorv1.NewExecutorServiceClient(conn).Execute(ctx, &executorv1.ExecuteRequest{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("verdict = %v, want ACCEPTED", resp.GetVerdict())
	}

	if err := shutdown(); err != nil {
		t.Errorf("Serve returned error after graceful stop: %v", err)
	}
}

func TestPanicBecomesInternalError(t *testing.T) {
	t.Parallel()

	panicking := &stubExecutor{
		execute: func(context.Context, *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
			panic("boom")
		},
	}
	conn, shutdown := startBufconnServer(t, panicking)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := executorv1.NewExecutorServiceClient(conn).Execute(ctx, &executorv1.ExecuteRequest{})
	if status.Code(err) != codes.Internal {
		t.Errorf("error code = %v, want Internal (err: %v)", status.Code(err), err)
	}

	if err := shutdown(); err != nil {
		t.Errorf("Serve returned error after graceful stop: %v", err)
	}
}

func TestRunReturnsListenError(t *testing.T) {
	t.Parallel()

	srv := New("256.256.256.256:0", discardLogger(), okExecutor())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Run(ctx); err == nil {
		t.Fatal("Run with invalid address returned nil error, want error")
	}
}

func TestRunListensAndShutsDown(t *testing.T) {
	t.Parallel()

	srv := New("127.0.0.1:0", discardLogger(), okExecutor())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	// Give the listener a moment to start before requesting shutdown.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s of context cancellation")
	}
}
