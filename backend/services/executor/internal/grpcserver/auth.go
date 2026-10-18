package grpcserver

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// healthServicePrefix matches every method of the standard gRPC health
// service. Health probes stay unauthenticated so load balancers, kubelet
// probes and grpc_health_probe keep working without the shared secret; they
// reveal nothing beyond "serving / not serving".
var healthServicePrefix = "/" + healthpb.Health_ServiceDesc.ServiceName + "/"

// errUnauthenticated is deliberately generic: it never says whether the
// header was missing, malformed, or carried the wrong token.
var errUnauthenticated = status.Error(codes.Unauthenticated, "missing or invalid credentials")

// tokenAuthorizer checks the `authorization: Bearer <token>` metadata the
// gateway attaches to every RPC. An empty token disables the check (local
// development, CI, kind) — the caller is expected to log that loudly.
type tokenAuthorizer struct {
	token []byte
}

func newTokenAuthorizer(token string) tokenAuthorizer {
	return tokenAuthorizer{token: []byte(token)}
}

// enabled reports whether a token is configured.
func (a tokenAuthorizer) enabled() bool { return len(a.token) > 0 }

// authorize returns nil when the call may proceed.
func (a tokenAuthorizer) authorize(ctx context.Context, fullMethod string) error {
	if !a.enabled() || strings.HasPrefix(fullMethod, healthServicePrefix) {
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errUnauthenticated
	}
	// gRPC lower-cases metadata keys; exactly one value is expected.
	values := md.Get("authorization")
	if len(values) != 1 {
		return errUnauthenticated
	}
	scheme, presented, found := strings.Cut(values[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return errUnauthenticated
	}
	// Constant-time comparison so response timing leaks nothing about how
	// much of the token matched. (Length still differs observably, which is
	// harmless for a high-entropy random secret.)
	if subtle.ConstantTimeCompare([]byte(presented), a.token) != 1 {
		return errUnauthenticated
	}
	return nil
}

// authUnaryInterceptor rejects unauthenticated unary RPCs.
func authUnaryInterceptor(a tokenAuthorizer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := a.authorize(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// authStreamInterceptor rejects unauthenticated streaming RPCs. The executor
// API is unary today; this closes the gap for any future stream (and for the
// health service's Watch, which stays exempt like Check).
func authStreamInterceptor(a tokenAuthorizer) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := a.authorize(ss.Context(), info.FullMethod); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}
