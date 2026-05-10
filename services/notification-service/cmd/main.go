package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/mailgun/mailgun-go/v4"
	"google.golang.org/grpc"
	pb "github.com/cmpnion/adp-final/proto/notification"
)

type notificationServer struct {
	pb.UnimplementedNotificationServiceServer
	mg mailgun.Mailgun
}

func (s *notificationServer) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendEmailResponse, error) {
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
	from := os.Getenv("MAILGUN_FROM_EMAIL")
	if from == "" {
		from = "norkindima57@gmail.com"
	}
	m := s.mg.NewMessage(
		from,
		"Order Confirmation",
		"Order confirmation email",
		"customer@example.com",
	)
	_, _, err := s.mg.Send(ctx, m)
	if err != nil {
		return &pb.SendOrderConfirmationResponse{Ok: false}, err
	}
	return &pb.SendOrderConfirmationResponse{Ok: true}, nil
}

func (s *notificationServer) SendStockAlert(ctx context.Context, req *pb.SendStockAlertRequest) (*pb.SendStockAlertResponse, error) {
	from := os.Getenv("MAILGUN_FROM_EMAIL")
	if from == "" {
		from = "norkindima57@gmail.com"
	}
	m := s.mg.NewMessage(
		from,
		"Stock Alert",
		"Stock alert email",
		"admin@inventory.local",
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

	apiKey := os.Getenv("MAILGUN_API_KEY")
	if apiKey == "" {
		log.Println("warning: MAILGUN_API_KEY not set, email notifications will fail")
	}
	domain := os.Getenv("MAILGUN_DOMAIN")
	if domain == "" {
		log.Println("warning: MAILGUN_DOMAIN not set, email notifications will fail")
	}

	mg := mailgun.NewMailgun(domain, apiKey)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	grpcSrv := grpc.NewServer()
	pb.RegisterNotificationServiceServer(grpcSrv, &notificationServer{mg: mg})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
	}()

	log.Printf("notification-service gRPC listening on %s", grpcAddr)
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
