//revive:disable:package-comments
package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"git.sonicoriginal.software/grpcd-go/client"

	"git.sonicoriginal.software/grpc-foundation/methods"
	"git.sonicoriginal.software/grpc-foundation/server"

	"git.sonicoriginal.software/logger"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // Experimental gzip initialization
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Run starts a gRPC server with the provided configuration.
// Blocking call but with early error failure if there is misconfiguration
// and also gracefully stops/shuts down.
// Handles reading gRPC service methods on startup and registering those methods
// if backend grpcd service is configured. On shutdown, will also gracefully
// deregister those methods with the grpcd service if configured.
// If lis is nil, a listener is created from the GRPC_SERVER_ADDRESS environment variable.
// Additional grpc.ServerOptions can be provided to customize server behavior (e.g., UnknownServiceHandler).
func Run(
	ctx context.Context,
	serviceName string,
	log *slog.Logger,
	registerFn func(grpc.ServiceRegistrar),
	lis net.Listener,
	opts ...grpc.ServerOption,
) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName cannot be empty")
	}

	if log == nil {
		log = logger.NewNullLogger()
	}

	// Create cancellable child context for background goroutines and attach logger
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Attach logger to context so all goroutines can access it
	ctx = logger.ContextWithLogger(ctx, log)

	// Create listener if not provided
	if lis == nil {
		address := server.Address()
		var err error
		lis, err = net.Listen("tcp", address)
		if err != nil {
			log.Error("Failed to create listener",
				slog.String("address", address),
				slog.Any("error", err))
			return fmt.Errorf("failed to create listener: %w", err)
		}
	}

	// Add listener address as persistent logger attribute so it appears on all subsequent logs
	log = log.With(slog.String("address", lis.Addr().String()))
	ctx = logger.ContextWithLogger(ctx, log)

	// Create gRPC server with interceptors and connection settings
	grpcServer := server.New(log, opts...)

	// Register service implementations
	if registerFn != nil {
		registerFn(grpcServer)
	}

	// Extract methods for service discovery, excluding internal/infrastructure services
	filter := methods.NewPatternFilter(nil, []string{
		"grpc.",        // gRPC infrastructure (health, reflection)
		"info.",        // Cumulus internal info endpoint
		"diagnostics.", // Cumulus internal diagnostics endpoint
	})
	methodList := methods.Extract(grpcServer, filter)
	log.DebugContext(ctx, "Extracted methods", slog.Any("methods", methodList))

	grpcdClient := client.New(log, methodList)

	if len(methodList) > 0 {
		go grpcdClient.Run(ctx)
	}

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service
	reflection.Register(grpcServer)

	// Handle graceful shutdown
	go server.HandleGracefulShutdown(ctx, cancel, log, grpcServer, 5*time.Second)

	// Run the server (blocking)
	log.Info("gRPC server listening")
	if err := grpcServer.Serve(lis); err != nil {
		log.Error("Failed to serve", slog.Any("error", err))
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
