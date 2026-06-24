package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMetrics returns a Server exposing the Prometheus registry on its own
// listener. Keeping /metrics off the public server means the NodePort (and
// any future ingress) never exposes operational internals; in Kubernetes
// only Prometheus scrapes this port.
func NewMetrics(addr string, log *slog.Logger) *Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	return &Server{
		httpSrv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		log: log,
	}
}
