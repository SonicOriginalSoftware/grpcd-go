//revive:disable:package-comments
package service

import (
	"log/slog"

	"git.sonicoriginal.software/grpcd-go/example/internal/station"

	"git.sonicoriginal.software/grpc-protos/diagnostics"
	"git.sonicoriginal.software/grpc-protos/info"

	"git.sonicoriginal.software/logger"
	"go.opentelemetry.io/otel/metric"
)

// Server is the concretion of the gRPC service implementation
type Server struct {
	station.UnimplementedExampleServiceServer
	info.UnimplementedInfoServiceServer
	diagnostics.UnimplementedDiagnosticsServiceServer
	log *slog.Logger

	// Metrics
	createCount            metric.Int64Counter
	validationFailureCount metric.Int64Counter
}

// NewServer returns a new server
func NewServer(log *slog.Logger, meter metric.Meter) *Server {
	if log == nil {
		log = logger.NewNullLogger()
	}

	// Initialize metrics
	createCount, err := meter.Int64Counter(
		"creates.total",
		metric.WithDescription("Total number created"),
		metric.WithUnit("{creates}"),
	)
	if err != nil {
		log.Error("Failed to create creates metric", "error", err)
	}

	validationFailureCount, err := meter.Int64Counter(
		"creates.validation_failures.total",
		metric.WithDescription("Total number of validation failures"),
		metric.WithUnit("{failure}"),
	)
	if err != nil {
		log.Error("Failed to create validation failures metric", "error", err)
	}

	return &Server{
		log:                    log,
		createCount:            createCount,
		validationFailureCount: validationFailureCount,
	}
}
