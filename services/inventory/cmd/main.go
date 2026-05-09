package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/go-redis/redis/v8"
	"github.com/yourorg/inventory/internal/infra/nats"
	redisinfra "github.com/yourorg/inventory/internal/infra/redis"
	"github.com/yourorg/inventory/internal/infra/postgres"
	"github.com/yourorg/inventory/internal/application/usecases"
	grpcsrv "github.com/yourorg/inventory/internal/delivery/grpc"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	natsURL := os.Getenv("NATS_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dbURL == "" { dbURL = "postgres://postgres:pass@localhost:5432/inventory?sslmode=disable" }
	if natsURL == "" { natsURL = "nats://localhost:4222" }
	if redisAddr == "" { redisAddr = "localhost:6379" }

	db, err := sql.Open("postgres", dbURL)
	if err != nil { log.Fatalf("db open: %v", err) }
	defer db.Close()
	// ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil { log.Fatalf("db ping: %v", err) }

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil { log.Printf("redis ping: %v", err) }

	natsPub, err := natsinfra.NewPublisher(natsURL)
	if err != nil { log.Printf("nats connect: %v", err) }

	repo := postgres.NewPostgresRepo(db)
	cache := redisinfra.NewCache(rdb)
	uc := usecases.NewReserveService(repo, cache, cache, natsPub)

	// start gRPC server
	srv := grpcsrv.NewServer(uc)
	if err := grpcsrv.StartGRPC(":50051", srv); err != nil { log.Fatalf("grpc: %v", err) }
}
