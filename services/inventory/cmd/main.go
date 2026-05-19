package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	evpb "github.com/cmpnion/adp-final/proto/events"
	invpb "github.com/cmpnion/adp-final/proto/inventory"
	"github.com/cmpnion/adp-final/services/inventory/internal/application/usecases"
	grpcsrv "github.com/cmpnion/adp-final/services/inventory/internal/delivery/grpc"
	natsinfra "github.com/cmpnion/adp-final/services/inventory/internal/infra/nats"
	"github.com/cmpnion/adp-final/services/inventory/internal/infra/postgres"
	redisinfra "github.com/cmpnion/adp-final/services/inventory/internal/infra/redis"
	"github.com/cmpnion/adp-final/services/inventory/internal/observability"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"net"
)

func main() {
	ctxShutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	natsURL := os.Getenv("NATS_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	metricsAddr := os.Getenv("METRICS_ADDR")
	grpcAddr := os.Getenv("GRPC_ADDR")
	if dbURL == "" {
		dbURL = "postgres://postgres:pass@localhost:5432/inventory_db?sslmode=disable"
	}
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	if metricsAddr == "" {
		metricsAddr = ":9090"
	}
	if grpcAddr == "" {
		grpcAddr = ":50051"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("redis ping: %v", err)
	}

	natsPub, err := natsinfra.NewPublisher(natsURL)
	if err != nil {
		log.Fatalf("nats connect: %v", err)
	}

	repo := postgres.NewPostgresRepo(db)
	cache := redisinfra.NewCache(rdb)
	locker := cache

	reserveUC := usecases.NewReserveService(repo, locker, cache, natsPub)
	releaseUC := usecases.NewReleaseService(repo, locker, cache, natsPub)
	confirmUC := usecases.NewConfirmService(repo, locker, cache, natsPub)

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctxShutdown.Done():
				return
			case <-ticker.C:
				removed, err := reserveUC.CleanupExpiredReservations(context.Background())
				if err != nil {
					log.Printf("expired reservations cleanup: %v", err)
					continue
				}
				if removed > 0 {
					log.Printf("expired reservations cleaned up: %d", removed)
				}
			}
		}
	}()

	observability.EnsureRegistered()

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
	metricsSrv := &http.Server{Addr: metricsAddr, Handler: metricsMux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Printf("inventory metrics listening %s", metricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("metrics serve: %v", err)
		}
	}()

	// subscribe to order events
	_, _ = natsPub.Subscribe("order.created", func(m *nats.Msg) {
		var ev evpb.OrderCreated
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			log.Printf("order.created unmarshal: %v", err)
			return
		}
		go func(orderID string, items []*evpb.OrderItem) {
			ctx := context.Background()
			// Reserve each item individually
			for _, it := range items {
				if _, err := reserveUC.ReserveStock(ctx, orderID, it.Sku, "", it.Qty); err != nil {
					log.Printf("reserve item %s err: %v", it.Sku, err)
				}
			}
		}(ev.OrderId, ev.Items)
	})

	_, _ = natsPub.Subscribe("order.cancelled", func(m *nats.Msg) {
		var ev evpb.OrderCancelled
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			log.Printf("order.cancelled unmarshal: %v", err)
			return
		}
		go func(orderID string) {
			if _, err := releaseUC.ReleaseStock(context.Background(), orderID); err != nil {
				log.Printf("release err: %v", err)
			}
		}(ev.OrderId)
	})

	_, _ = natsPub.Subscribe("order.completed", func(m *nats.Msg) {
		var ev evpb.OrderCompleted
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			log.Printf("order.completed unmarshal: %v", err)
			return
		}
		go func(orderID string) {
			if _, err := confirmUC.ConfirmStock(context.Background(), orderID); err != nil {
				log.Printf("confirm err: %v", err)
			}
		}(ev.OrderId)
	})

	// start gRPC
	l, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			observability.UnaryMetricsInterceptor(),
			observability.UnaryLoggingInterceptor(),
		),
	)
	server := grpcsrv.NewServer(repo, reserveUC, releaseUC, confirmUC)
	invpb.RegisterInventoryServiceServer(s, server)
	go func() {
		log.Printf("inventory gRPC listening %s", grpcAddr)
		if err := s.Serve(l); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	// graceful shutdown
	<-ctxShutdown.Done()
	log.Println("shutting down")
	s.GracefulStop()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = metricsSrv.Shutdown(shutdownCtx)
	natsPub.Close()
}
