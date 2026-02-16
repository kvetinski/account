package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/kvetinski/account/config"
	"github.com/kvetinski/account/internal/adapters/grpcapi"
	"github.com/kvetinski/account/internal/adapters/grpcapi/accountv1"
	"github.com/kvetinski/account/internal/adapters/repository"
	accountsvc "github.com/kvetinski/account/internal/service/account"
	"github.com/kvetinski/account/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.New()
	logger.Info("starting account service", "grpc_addr", cfg.GRPCAddr, "metrics_addr", cfg.MetricsAddr)

	ctx := context.Background()
	shutdownTracing, err := telemetry.InitTracing(ctx, telemetry.TracingConfig{
		Enabled:      cfg.TracingEnabled,
		ServiceName:  cfg.TracingServiceName,
		OTLPEndpoint: cfg.TracingOTLPEndpoint,
		Insecure:     cfg.TracingOTLPInsecure,
		SampleRatio:  cfg.TracingSampleRatio,
	})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			logger.Error("shutdown tracing failed", "error", err)
		}
	}()

	db, err := sql.Open("postgres", cfg.PostgresURI)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	logger.Info("database connected")

	metrics := telemetry.NewMetrics(nil)
	if err = telemetry.RegisterDBPoolMetrics(db, nil); err != nil {
		return fmt.Errorf("register db pool metrics: %w", err)
	}

	repo := repository.NewWithMetrics(db, metrics)
	svc := accountsvc.New(repo)
	grpcServerImpl := grpcapi.NewServer(svc, logger)

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(grpcapi.UnaryMetricsInterceptor(metrics, logger)),
	)
	accountv1.RegisterAccountServiceServer(grpcSrv, grpcServerImpl)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}
	defer lis.Close()

	errCh := make(chan error, 2)

	go func() {
		logger.Info("metrics server listening", "addr", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	go func() {
		logger.Info("grpc server listening", "addr", cfg.GRPCAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metricsErrCh := make(chan error, 1)
	go func() {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			metricsErrCh <- fmt.Errorf("shutdown metrics server: %w", err)
			return
		}
		metricsErrCh <- nil
	}()

	grpcDone := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(grpcDone)
	}()

	select {
	case <-grpcDone:
		logger.Info("grpc server stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("grpc graceful shutdown timed out, forcing stop")
		grpcSrv.Stop()
	}

	if err := <-metricsErrCh; err != nil {
		return err
	}

	logger.Info("shutdown complete")
	return nil
}
