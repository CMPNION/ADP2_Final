package observability

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_grpc_requests_total",
			Help: "Total gRPC requests handled by the inventory service.",
		},
		[]string{"method", "code"},
	)
	grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "inventory_grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
	registered bool
)

func EnsureRegistered() {
	if registered {
		return
	}
	prometheus.MustRegister(grpcRequestsTotal, grpcRequestDuration)
	registered = true
}

func UnaryMetricsInterceptor() grpc.UnaryServerInterceptor {
	EnsureRegistered()
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)
		grpcRequestsTotal.WithLabelValues(info.FullMethod, code.String()).Inc()
		grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func UnaryLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Printf(`{"component":"inventory","method":%q,"code":%q,"duration_ms":%d}`,
			info.FullMethod, status.Code(err).String(), time.Since(start).Milliseconds())
		return resp, err
	}
}
