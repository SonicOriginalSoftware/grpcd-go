package service_test

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"

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

	log := slog.Default()

	lis, err := foundation.Listen()
	if err != nil {
		log.Error("Failed to create listener", slog.Any("error", err))

		return
	}

	log = log.With(slog.String("address", lis.Addr().String()))
	ctx = logger.ContextWithLogger(ctx, log)

	// New installs an interceptor that puts this logger into every request
	// context, so it has to be built after the logger is complete.
	srv := foundation.New(log)

	// Checks for the upstream services this one depends on. A grpcd check is
	// added for you when GRPCD_ADDRESS is set, and "grpcd" is reserved either
	// way.
	checks := diagnostics.Checks{}

	healthSrv := health.NewServer()

	methodList, err := service.Register(srv, healthSrv, checks, registerExampleService)
	if err != nil {
		log.Error("Failed to register services", slog.Any("error", err))

		return
	}

	grpcdClient := grpcdclient.New(log, methodList)

	var wg sync.WaitGroup

	wg.Go(func() { grpcdClient.Run(ctx) })
	wg.Go(func() { foundation.HandleGracefulShutdown(ctx, cancel, log, srv, 5*time.Second) })

	log.Info("gRPC server listening")
	if err := srv.Serve(lis); err != nil {
		log.Error("Failed to serve", slog.Any("error", err))
	}

	// Serve returns once the shutdown handler has stopped the server. The grpcd
	// client deregisters on its way out, so the process has to stay up until it
	// has finished.
	wg.Wait()
}
