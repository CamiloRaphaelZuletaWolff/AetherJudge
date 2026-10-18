package judge

import (
	"context"
	"crypto/tls"
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

// DialConfig selects how the gateway secures its connection to the executor.
// The zero value is plaintext and unauthenticated — right for local dev, CI,
// and in-cluster traffic, never for an executor reachable from the internet.
type DialConfig struct {
	// AuthToken, when non-empty, is sent on every RPC as
	// `authorization: Bearer <token>` metadata (EXECUTOR_AUTH_TOKEN).
	AuthToken string
	// TLS dials with TLS verified against the system root CAs
	// (EXECUTOR_TLS=true), e.g. through a Caddy/Let's Encrypt front proxy.
	TLS bool
}

// Dial connects the executor gRPC client with client-side round-robin load
// balancing across all backends a (headless) DNS name resolves to, so the
// gateway fans Execute calls across executor replicas (ADR-0011). The
// connection is lazy; failures surface per-call.
func Dial(addr string, dc DialConfig) (executorv1.ExecutorServiceClient, *grpc.ClientConn, error) {
	return dialWith(addr, dc)
}

// dialWith is Dial plus extra options (tests inject an in-memory dialer).
func dialWith(addr string, dc DialConfig, extra ...grpc.DialOption) (executorv1.ExecutorServiceClient, *grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials(dc.TLS)),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
		// Client spans + W3C propagation toward the executor; no-op when
		// tracing is disabled.
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
	if dc.AuthToken != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearerToken{
			token:      dc.AuthToken,
			requireTLS: dc.TLS,
		}))
	}

	opts = append(opts, extra...)

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("judge: dial executor at %s: %w", addr, err)
	}
	return executorv1.NewExecutorServiceClient(conn), conn, nil
}

// transportCredentials returns TLS (system roots, server name taken from the
// dial target) or plaintext.
func transportCredentials(useTLS bool) credentials.TransportCredentials {
	if useTLS {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}

// bearerToken attaches the shared executor secret to every RPC.
type bearerToken struct {
	token string
	// requireTLS mirrors whether the transport is TLS. When true, gRPC
	// refuses to send the token over a plaintext connection; when false the
	// token travels in clear text, which is acceptable only on a private
	// network (local dev, docker-compose, in-cluster).
	requireTLS bool
}

var _ credentials.PerRPCCredentials = bearerToken{}

// GetRequestMetadata implements credentials.PerRPCCredentials.
func (b bearerToken) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

// RequireTransportSecurity implements credentials.PerRPCCredentials.
func (b bearerToken) RequireTransportSecurity() bool { return b.requireTLS }
