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

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"net"
	"omnichannel/inventory/internal/application/usecases"
	grpcsrv "omnichannel/inventory/internal/delivery/grpc"
	natsinfra "omnichannel/inventory/internal/infra/nats"
	"omnichannel/inventory/internal/infra/postgres"
	redisinfra "omnichannel/inventory/internal/infra/redis"
	"omnichannel/inventory/internal/observability"
	evpb "omnichannel/proto/events"
	invpb "omnichannel/proto/inventory"
)

func main() {
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
		items := []usecases.ItemReq{}
		for _, it := range ev.Items {
			items = append(items, usecases.ItemReq{SKU: it.Sku, WarehouseID: "", Quantity: it.Qty})
		}
		go func(orderID string, items []usecases.ItemReq) {
			if _, err := reserveUC.ReserveStock(context.Background(), orderID, items); err != nil {
				log.Printf("reserve err: %v", err)
			}
		}(ev.OrderId, items)
	})

	_, _ = natsPub.Subscribe("order.cancelled", func(m *nats.Msg) {
		var ev evpb.OrderCancelled
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			log.Printf("order.cancelled unmarshal: %v", err)
			return
		}
		go func(orderID string) {
			if err := releaseUC.ReleaseStock(context.Background(), orderID); err != nil {
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
			if err := confirmUC.ConfirmStockDeduction(context.Background(), orderID); err != nil {
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
	ctxShutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-ctxShutdown.Done()
	log.Println("shutting down")
	s.GracefulStop()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = metricsSrv.Shutdown(shutdownCtx)
	natsPub.Close()
	stop()
}
