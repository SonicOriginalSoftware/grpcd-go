package service_test

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"

	"git.sonicoriginal.software/logger"

	foundation "git.sonicoriginal.software/grpc-foundation/server"

	grpcdclient "git.sonicoriginal.software/grpcd-go/client"
	"git.sonicoriginal.software/grpcd-go/diagnostics"
	"git.sonicoriginal.software/grpcd-go/service"
)

// registerExampleService stands in for the caller's own service registration.
func registerExampleService(s grpc.ServiceRegistrar) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "example.ExampleService",
		HandlerType: (*any)(nil),
		Methods:     []grpc.MethodDesc{{MethodName: "Create"}},
	}, struct{}{})
}

// Example shows how a service wires itself up. The listener and the server are
// the caller's, as are the goroutines and the blocking Serve; this package only
// assembles. It has no Output comment, so it is compiled but never run — its
// job is to keep this sequence type-checked.
func Example() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serviceName := "example-service"
	log := slog.Default()

	lis, err := foundation.Listen()
	if err != nil {
		log.Error("Failed to create listener", slog.Any("error", err))

		return
	}

	srv := foundation.New(log)

	log = log.With(slog.String("address", lis.Addr().String()))
	ctx = logger.ContextWithLogger(ctx, log)

	checks := diagnostics.Checks{}
	if err := service.Register(srv, serviceName, checks, registerExampleService); err != nil {
		log.Error("Failed to register services", slog.Any("error", err))

		return
	}

	grpcdClient := grpcdclient.New(log, service.Methods(srv))
	go grpcdClient.Run(ctx)

	go foundation.HandleGracefulShutdown(ctx, cancel, log, srv, 5*time.Second)

	log.Info("gRPC server listening")
	if err := srv.Serve(lis); err != nil {
		log.Error("Failed to serve", slog.Any("error", err))
	}
}
