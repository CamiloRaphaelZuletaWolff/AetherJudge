package auth

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// refreshReuseTotal is a security signal, not a traffic stat: any non-zero
// rate means rotated refresh tokens are being replayed (theft, or a client
// bug double-spending tokens) and a whole token family was revoked.
var refreshReuseTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "arena_auth_refresh_reuse_total",
	Help: "Refresh-token reuse detections (each one revoked a user's token family).",
})
