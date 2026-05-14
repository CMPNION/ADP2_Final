package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cmpnion/adp-final/apigateway/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := app.LoadConfig()
	if cfg.JWTSecret == "" && cfg.AuthToken == "" {
		log.Fatal("missing JWT_SECRET or AUTH_TOKEN")
	}
	
	invConn, err := grpc.DialContext(ctx, cfg.InventoryGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("inventory dial: %v", err)
	}
	defer invConn.Close()

	catConn, err := grpc.DialContext(ctx, "catalog-service:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("catalog dial warn (not critical): %v", err)
	}
	
	ordConn, err := grpc.DialContext(ctx, "order-service:50053", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("order dial warn (not critical): %v", err)
	}
	
	notConn, err := grpc.DialContext(ctx, "notification-service:50054", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("notification dial warn (not critical): %v", err)
	}

	srv := app.NewServer(cfg, invConn, catConn, ordConn, notConn)
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	metricsServer := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
		_ = metricsServer.Shutdown(shutdownCtx)
	}()

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	metricsLn, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		log.Fatalf("metrics listen: %v", err)
	}
	go func() {
		log.Printf("api-gateway metrics listening on %s", cfg.MetricsAddr)
		if err := metricsServer.Serve(metricsLn); err != nil && err != http.ErrServerClosed {
			log.Fatalf("metrics serve: %v", err)
		}
	}()
	log.Printf("api-gateway listening on %s", cfg.HTTPAddr)
	if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
