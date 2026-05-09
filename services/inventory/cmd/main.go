package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"net"
	"omnichannel/inventory/internal/application/usecases"
	grpcsrv "omnichannel/inventory/internal/delivery/grpc"
	natsinfra "omnichannel/inventory/internal/infra/nats"
	"omnichannel/inventory/internal/infra/postgres"
	redisinfra "omnichannel/inventory/internal/infra/redis"
	evpb "omnichannel/proto/events"
	invpb "omnichannel/proto/inventory"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	natsURL := os.Getenv("NATS_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dbURL == "" {
		dbURL = "postgres://postgres:pass@localhost:5432/inventory_db?sslmode=disable"
	}
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	if redisAddr == "" {
		redisAddr = "localhost:6379"
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
	l, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	s := grpc.NewServer()
	server := grpcsrv.NewServer(reserveUC, releaseUC, confirmUC)
	invpb.RegisterInventoryServer(s, server)
	go func() {
		log.Println("inventory gRPC listening :50051")
		if err := s.Serve(l); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	// graceful shutdown
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc
	log.Println("shutting down")
	s.GracefulStop()
	natsPub.Close()
}
