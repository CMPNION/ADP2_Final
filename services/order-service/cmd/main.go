package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"google.golang.org/grpc/credentials/insecure"

	catpb "github.com/cmpnion/adp-final/proto/catalog"
	pb "github.com/cmpnion/adp-final/proto/order"
	"github.com/cmpnion/adp-final/services/order/internal/application/usecases"
	grpcdelivery "github.com/cmpnion/adp-final/services/order/internal/delivery/grpc"
	pg "github.com/cmpnion/adp-final/services/order/internal/infra/postgres"
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
		httpAddr = ":8082"
	}
	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":50053"
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:pass@localhost:5434/order_db?sslmode=disable"
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Printf("order nats connect warn: %v", err)
	}
	if nc != nil {
		defer nc.Close()
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("order db open: %v", err)
	}
	defer db.Close()
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()
	if err := db.PingContext(dbCtx); err != nil {
		log.Fatalf("order db ping: %v", err)
	}

	catalogAddr := os.Getenv("CATALOG_SERVICE_ADDR")
	if catalogAddr == "" {
		catalogAddr = "catalog-service:50052"
	}
	catConn, err := grpc.Dial(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("order dial catalog service: %v", err)
	}
	defer catConn.Close()
	catClient := catpb.NewCatalogServiceClient(catConn)

	repo := pg.NewRepo(db)
	svc := usecases.NewService(repo, nc, catClient)

	// Subscribe to inventory events to keep order items in sync with reservations
	if nc != nil {
		// reserved -> add quantities to order items and mark PENDING
		_, _ = nc.Subscribe("inventory.stock.reserved", func(m *nats.Msg) {
			var evt struct {
				OrderID string `json:"order_id"`
				Items   []struct {
					SKU      string `json:"sku"`
					Quantity int64  `json:"quantity"`
				} `json:"items"`
			}
			if err := json.Unmarshal(m.Data, &evt); err != nil {
				log.Printf("order-service: failed parse reserved event: %v", err)
				return
			}
			ctx := context.Background()
			for _, it := range evt.Items {
				if it.Quantity == 0 {
					continue
				}
				if err := repo.AdjustOrderItemQuantity(ctx, evt.OrderID, it.SKU, it.Quantity); err != nil {
					log.Printf("order-service: failed adjust order item on reserve: %v", err)
				}
			}
			// set order status to PENDING
			if ok := svc.UpdateStatus(context.Background(), evt.OrderID, "PENDING"); !ok {
				log.Printf("order-service: failed set order PENDING %s", evt.OrderID)
			}
		})

		// released -> subtract quantities from order items; if no items left, set CREATED
		_, _ = nc.Subscribe("inventory.stock.released", func(m *nats.Msg) {
			var evt struct {
				OrderID     string `json:"order_id"`
				SKU         string `json:"sku"`
				Quantity    int64  `json:"quantity"`
				WarehouseID string `json:"warehouse_id"`
			}
			if err := json.Unmarshal(m.Data, &evt); err != nil {
				log.Printf("order-service: failed parse released event: %v", err)
				return
			}
			ctx := context.Background()
			if evt.Quantity != 0 {
				if err := repo.AdjustOrderItemQuantity(ctx, evt.OrderID, evt.SKU, -evt.Quantity); err != nil {
					log.Printf("order-service: failed adjust order item on release: %v", err)
				}
			}
			// if order now has no items, set status back to CREATED
			o, _ := repo.GetOrder(ctx, evt.OrderID)
			if o := o; o != nil {
				if len(o.Items) == 0 {
					if ok := svc.UpdateStatus(context.Background(), evt.OrderID, "CREATED"); !ok {
						log.Printf("order-service: failed set order CREATED %s", evt.OrderID)
					}
				}
			}
		})

		// confirmed -> mark order as CONFIRMED
		_, _ = nc.Subscribe("inventory.stock.confirmed", func(m *nats.Msg) {
			var evt struct {
				OrderID string `json:"order_id"`
				SKU     string `json:"sku"`
			}
			if err := json.Unmarshal(m.Data, &evt); err != nil {
				log.Printf("order-service: failed parse confirmed event: %v", err)
				return
			}
			if ok := svc.UpdateStatus(context.Background(), evt.OrderID, "CONFIRMED"); !ok {
				log.Printf("order-service: failed set order CONFIRMED %s", evt.OrderID)
			}
		})
	}

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("order grpc listen: %v", err)
	}
	defer lis.Close()

	grpcSrv := grpc.NewServer()
	pb.RegisterOrderServiceServer(grpcSrv, grpcdelivery.NewServer(svc))

	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "order_http_requests_total",
			Help: "Total HTTP requests handled by order service.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "order_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds for order service.",
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
		log.Printf("order-service gRPC listening on %s", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("order grpc serve: %v", err)
		}
	}()

	log.Printf("order-service health http listening on %s", httpAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("order http serve: %v", err)
	}
}
