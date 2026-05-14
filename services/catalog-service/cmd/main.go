package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	pb "github.com/cmpnion/adp-final/proto/catalog"
	"github.com/cmpnion/adp-final/services/catalog/internal/application/usecases"
	grpcdelivery "github.com/cmpnion/adp-final/services/catalog/internal/delivery/grpc"
	pg "github.com/cmpnion/adp-final/services/catalog/internal/infra/postgres"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func main() {
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8081"
	}
	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":50052"
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:pass@localhost:5433/catalog_db?sslmode=disable"
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Printf("catalog nats connect warn: %v", err)
	}
	if nc != nil {
		defer nc.Close()
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("catalog db open: %v", err)
	}
	defer db.Close()
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()
	if err := db.PingContext(dbCtx); err != nil {
		log.Fatalf("catalog db ping: %v", err)
	}
	svc := usecases.NewService(pg.NewRepo(db), nc)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("catalog grpc listen: %v", err)
	}
	defer lis.Close()
	grpcSrv := grpc.NewServer()
	pb.RegisterCatalogServiceServer(grpcSrv, grpcdelivery.NewServer(svc))

	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "catalog_http_requests_total",
			Help: "Total HTTP requests handled by catalog service.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "catalog_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds for catalog service.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	instrumented := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		mux.ServeHTTP(rec, r)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rec.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
	})

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           instrumented,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Printf("catalog-service gRPC listening on %s", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("catalog grpc serve: %v", err)
		}
	}()

	log.Printf("catalog-service health http listening on %s", httpAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("catalog http serve: %v", err)
	}
}
