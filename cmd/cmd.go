package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NIROOZbx/billing-service/internal/app"
	billingv1 "github.com/NIROOZbx/billing-service/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func Run(a *app.App, addr string) error {


	signals := []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP}
	ctx, cancel := signal.NotifyContext(context.Background(), signals...)
		defer cancel()
		

	grpcServer, lis, err := setupGRPCServer(a, addr)
	if err != nil {
		return err
	}
	httpServer := setupHTTPServer(a)

	serverErrors := make(chan error, 2)
	go func() {
		a.Logger.Info().
			Str("service", a.Config.App.Name).
			Str("addr", addr).
			Msg("Starting gRPC server")
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			serverErrors <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	go func() {
		a.Logger.Info().
			Str("port", a.Config.App.HttpPort).
			Msg("Starting HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	go func() {
		a.Logger.Info().Msg("Starting subscription expiry scheduler")
		a.Scheduler.Start(ctx)
	}()

	a.Logger.Info().Msg("🟢 Service is fully started and running. Waiting for signals...")

	select {
	case err := <-serverErrors:
		a.Logger.Error().Err(err).Msg("💥 Server crashed during runtime")
		return err
	case <-ctx.Done():
		a.Logger.Info().Msg("🛑 Received shutdown signal. Starting graceful shutdown...")
	}

	return gracefulShutdown(a, grpcServer, httpServer)
}

func setupGRPCServer(a *app.App, addr string) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	s := grpc.NewServer()
	billingv1.RegisterBillingServiceServer(s, a.GRPCHandler)

	if a.Config.App.Environment == "development" {
		reflection.Register(s)
	}
	return s, lis, nil
}

func setupHTTPServer(a *app.App) *http.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/webhooks/stripe", a.WebHookHandler.Handle)
    return &http.Server{
        Addr:    ":" + a.Config.App.HttpPort,
        Handler: mux,
    }
}

func gracefulShutdown(a *app.App, grpcServer *grpc.Server, httpServer *http.Server) error {
	a.Logger.Info().Msg("❌ Service is gracefully stopping")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		a.Logger.Error().Err(err).Msg("HTTP shutdown error")
	}
	a.Logger.Info().Msg("HTTP server stopped gracefully")


	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
		a.Logger.Info().Msg("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		a.Logger.Warn().Msg("Graceful shutdown timed out, force stopping...")
		grpcServer.Stop()
	}

	return nil
}
