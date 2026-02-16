package grpcapi

import (
	"context"
	"log/slog"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/kvetinski/account/internal/telemetry"
)

func UnaryMetricsInterceptor(metrics *telemetry.Metrics, logger *slog.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		method := path.Base(info.FullMethod)
		start := time.Now()

		metrics.IncRPCInFlight()
		defer metrics.DecRPCInFlight()

		resp, err = handler(ctx, req)
		code := status.Code(err).String()
		metrics.ObserveRPC(method, code, time.Since(start))

		logger.Info("grpc request",
			"method", method,
			"code", code,
			"duration_ms", time.Since(start).Milliseconds(),
		)

		return resp, err
	}
}
