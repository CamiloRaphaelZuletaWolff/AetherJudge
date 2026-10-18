package grpcserver

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

const testToken = "s3cret-token-for-tests-only"

func TestAuthorize(t *testing.T) {
	t.Parallel()

	const executeMethod = "/executor.v1.ExecutorService/Execute"

	withAuth := func(values ...string) context.Context {
		md := metadata.MD{}
		for _, v := range values {
			md.Append("authorization", v)
		}
		return metadata.NewIncomingContext(context.Background(), md)
	}

	tests := []struct {
		name    string
		token   string
		ctx     context.Context
		method  string
		wantErr bool
	}{
		{"disabled allows missing metadata", "", context.Background(), executeMethod, false},
		{"disabled allows any header", "", withAuth("Bearer whatever"), executeMethod, false},
		{"valid bearer token", testToken, withAuth("Bearer " + testToken), executeMethod, false},
		{"scheme is case-insensitive", testToken, withAuth("bearer " + testToken), executeMethod, false},
		{"no metadata at all", testToken, context.Background(), executeMethod, true},
		{"metadata without authorization", testToken, withAuth(), executeMethod, true},
		{"wrong token", testToken, withAuth("Bearer nope"), executeMethod, true},
		{"token prefix only", testToken, withAuth("Bearer " + testToken[:5]), executeMethod, true},
		{"token with suffix", testToken, withAuth("Bearer " + testToken + "x"), executeMethod, true},
		{"wrong scheme", testToken, withAuth("Basic " + testToken), executeMethod, true},
		{"bare token without scheme", testToken, withAuth(testToken), executeMethod, true},
		{"duplicate header", testToken, withAuth("Bearer "+testToken, "Bearer "+testToken), executeMethod, true},
		{"health check stays open", testToken, context.Background(), "/grpc.health.v1.Health/Check", false},
		{"health watch stays open", testToken, context.Background(), "/grpc.health.v1.Health/Watch", false},
		{"lookalike service is not exempt", testToken, context.Background(), "/grpc.health.v1.HealthX/Check", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := newTokenAuthorizer(tt.token).authorize(tt.ctx, tt.method)
			if tt.wantErr {
				if status.Code(err) != codes.Unauthenticated {
					t.Fatalf("authorize() = %v, want Unauthenticated", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("authorize() = %v, want nil", err)
			}
		})
	}
}

func TestExecuteRequiresTokenOverGRPC(t *testing.T) {
	t.Parallel()

	var called atomic.Bool
	stub := &stubExecutor{
		execute: func(context.Context, *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
			called.Store(true)
			return &executorv1.ExecuteResponse{Verdict: executorv1.Verdict_VERDICT_ACCEPTED}, nil
		},
	}
	conn, shutdown := startBufconnServerWithToken(t, stub, testToken)
	client := executorv1.NewExecutorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No credentials: rejected before reaching the handler.
	if _, err := client.Execute(ctx, &executorv1.ExecuteRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("Execute without token: code = %v, want Unauthenticated (err: %v)", status.Code(err), err)
	}
	if called.Load() {
		t.Error("handler ran for an unauthenticated call")
	}

	// Wrong token: rejected.
	badCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer wrong")
	if _, err := client.Execute(badCtx, &executorv1.ExecuteRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("Execute with wrong token: code = %v, want Unauthenticated", status.Code(err))
	}

	// Correct token: served.
	okCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+testToken)
	resp, err := client.Execute(okCtx, &executorv1.ExecuteRequest{})
	if err != nil {
		t.Fatalf("Execute with token: %v", err)
	}
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("verdict = %v, want ACCEPTED", resp.GetVerdict())
	}

	// Health stays reachable without credentials.
	hresp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check without token: %v", err)
	}
	if hresp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("health status = %v, want SERVING", hresp.GetStatus())
	}

	if err := shutdown(); err != nil {
		t.Errorf("Serve returned error after graceful stop: %v", err)
	}
}
