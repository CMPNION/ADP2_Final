package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/cmpnion/adp-final/proto/notification"
	"github.com/mailgun/mailgun-go/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type notificationServer struct {
	pb.UnimplementedNotificationServiceServer
	mg   mailgun.Mailgun
	mode string
}

var (
	grpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_grpc_requests_total",
			Help: "Total gRPC requests handled by notification service.",
		},
		[]string{"method", "code"},
	)
	grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notification_grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds for notification service.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
	metricsOnce sync.Once
)

func ensureMetricsRegistered() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(grpcRequestsTotal, grpcRequestDuration)
	})
}

func metricsUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	start := time.Now()
	resp, err = handler(ctx, req)
	code := status.Code(err).String()
	grpcRequestsTotal.WithLabelValues(info.FullMethod, code).Inc()
	grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
	return resp, err
}

func (s *notificationServer) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendEmailResponse, error) {
	if strings.EqualFold(s.mode, "log") {
		log.Printf("notification(log) SendEmail to=%s subject=%s", req.GetTo(), req.GetSubject())
		return &pb.SendEmailResponse{Ok: true}, nil
	}
	from := os.Getenv("MAILGUN_FROM_EMAIL")
	if from == "" {
		from = "norkindima57@gmail.com"
	}
	m := s.mg.NewMessage(
		from,
		req.Subject,
		req.Body,
		req.To,
	)
	_, _, err := s.mg.Send(ctx, m)
	if err != nil {
		return &pb.SendEmailResponse{Ok: false}, err
	}
	return &pb.SendEmailResponse{Ok: true}, nil
}

func (s *notificationServer) SendOrderConfirmation(ctx context.Context, req *pb.SendOrderConfirmationRequest) (*pb.SendOrderConfirmationResponse, error) {
	if strings.EqualFold(s.mode, "log") {
		log.Printf("notification(log) SendOrderConfirmation to=%s order_id=%s", req.GetTo(), req.GetOrderId())
		return &pb.SendOrderConfirmationResponse{Ok: true}, nil
	}
	from := os.Getenv("MAILGUN_FROM_EMAIL")
	if from == "" {
		from = "norkindima57@gmail.com"
	}
	body := "Order confirmation email"
	if req.GetOrderId() != "" {
		body = "Order confirmation for order " + req.GetOrderId()
	}
	to := req.GetTo()
	if to == "" {
		to = "customer@example.com"
	}
	m := s.mg.NewMessage(
		from,
		"Order Confirmation",
		body,
		to,
	)
	_, _, err := s.mg.Send(ctx, m)
	if err != nil {
		return &pb.SendOrderConfirmationResponse{Ok: false}, err
	}
	return &pb.SendOrderConfirmationResponse{Ok: true}, nil
}

func (s *notificationServer) SendStockAlert(ctx context.Context, req *pb.SendStockAlertRequest) (*pb.SendStockAlertResponse, error) {
	if strings.EqualFold(s.mode, "log") {
		log.Printf("notification(log) SendStockAlert to=%s sku=%s body=%s", req.GetTo(), req.GetSku(), req.GetBody())
		return &pb.SendStockAlertResponse{Ok: true}, nil
	}
	from := os.Getenv("MAILGUN_FROM_EMAIL")
	if from == "" {
		from = "norkindima57@gmail.com"
	}
	body := req.GetBody()
	if body == "" {
		body = "Stock alert for sku " + req.GetSku()
	}
	to := req.GetTo()
	if to == "" {
		to = "admin@inventory.local"
	}
	m := s.mg.NewMessage(
		from,
		"Stock Alert",
		body,
		to,
	)
	_, _, err := s.mg.Send(ctx, m)
	if err != nil {
		return &pb.SendStockAlertResponse{Ok: false}, err
	}
	return &pb.SendStockAlertResponse{Ok: true}, nil
}

func main() {
	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":50054"
	}
	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":8083"
	}
	mode := os.Getenv("NOTIFICATION_MODE")
	if mode == "" {
		mode = "log"
	}

	apiKey := os.Getenv("MAILGUN_API_KEY")
	if apiKey == "" {
		log.Println("warning: MAILGUN_API_KEY not set, email notifications will fail")
	}
	domain := os.Getenv("MAILGUN_DOMAIN")
	if domain == "" {
		log.Println("warning: MAILGUN_DOMAIN not set, email notifications will fail")
	}
	if strings.EqualFold(mode, "mailgun") {
		if apiKey == "" || domain == "" {
			log.Fatal(errors.New("NOTIFICATION_MODE=mailgun requires MAILGUN_API_KEY and MAILGUN_DOMAIN"))
		}
	}

	mg := mailgun.NewMailgun(domain, apiKey)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	ensureMetricsRegistered()
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(metricsUnaryInterceptor))
	pb.RegisterNotificationServiceServer(grpcSrv, &notificationServer{mg: mg, mode: mode})

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Printf("notification-service metrics listening on %s", metricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("notification metrics serve: %v", err)
		}
	}()

	log.Printf("notification-service mode=%s gRPC listening on %s", mode, grpcAddr)
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
