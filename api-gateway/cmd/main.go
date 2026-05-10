package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

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

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("api-gateway listening on %s", cfg.HTTPAddr)
	if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
