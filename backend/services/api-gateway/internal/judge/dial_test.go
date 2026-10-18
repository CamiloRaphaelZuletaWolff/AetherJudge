package judge

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

func TestBearerTokenMetadata(t *testing.T) {
	t.Parallel()

	md, err := bearerToken{token: "abc"}.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetRequestMetadata: %v", err)
	}
	if got := md["authorization"]; got != "Bearer abc" {
		t.Errorf("authorization = %q, want %q", got, "Bearer abc")
	}
	if (bearerToken{requireTLS: false}).RequireTransportSecurity() {
		t.Error("plaintext mode must allow the token over insecure transport (local dev)")
	}
	if !(bearerToken{requireTLS: true}).RequireTransportSecurity() {
		t.Error("TLS mode must refuse to send the token without transport security")
	}
}

func TestTransportCredentials(t *testing.T) {
	t.Parallel()

	if got := transportCredentials(false).Info().SecurityProtocol; got != "insecure" {
		t.Errorf("plaintext SecurityProtocol = %q, want insecure", got)
	}
	if got := transportCredentials(true).Info().SecurityProtocol; got != "tls" {
		t.Errorf("TLS SecurityProtocol = %q, want tls", got)
	}
}

// captureExecutor records the incoming authorization metadata.
type captureExecutor struct {
	executorv1.UnimplementedExecutorServiceServer
	got chan []string
}

func (c *captureExecutor) Execute(ctx context.Context, _ *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	c.got <- md.Get("authorization")
	return &executorv1.ExecuteResponse{}, nil
}

// TestDialSendsTokenOverPlaintext exercises the real Dial path end-to-end
// against an in-memory server: with TLS off (local dev), the token must
// still be attached.
func TestDialSendsTokenOverPlaintext(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		token string
		want  []string
	}{
		{"with token", "tok-123", []string{"Bearer tok-123"}},
		{"without token", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lis := bufconn.Listen(1 << 20)
			srv := grpc.NewServer()
			exec := &captureExecutor{got: make(chan []string, 1)}
			executorv1.RegisterExecutorServiceServer(srv, exec)
			go func() { _ = srv.Serve(lis) }()
			t.Cleanup(srv.Stop)

			// passthrough + a bufconn dialer: Dial's own options (insecure
			// transport, per-RPC token) stay exactly as in production.
			client, conn, err := dialWith("passthrough:///bufnet", DialConfig{AuthToken: tc.token},
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return lis.DialContext(ctx)
				}))
			if err != nil {
				t.Fatalf("Dial: %v", err)
			}
			t.Cleanup(func() { _ = conn.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := client.Execute(ctx, &executorv1.ExecuteRequest{}); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			got := <-exec.got
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Errorf("authorization metadata = %q, want %q", got, tc.want)
			}
		})
	}
}
