package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NIROOZbx/billing-service/internal/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func Run(a *app.App, addr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s := grpc.NewServer()

	if a.Config.App.Environment == "development" {
		reflection.Register(s)
	}

	go func() {
		log.Printf("Starting %s gRPC server on %s", a.Config.App.Name, addr)
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Graceful shutdown timed out, force stopping...")
		s.Stop()
	}

	return nil
}
