package service_test

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"

	"git.sonicoriginal.software/logger"

	foundationclient "git.sonicoriginal.software/grpc-foundation/client"
	foundationotel "git.sonicoriginal.software/grpc-foundation/otel"
	foundation "git.sonicoriginal.software/grpc-foundation/server"

	grpcdclient "git.sonicoriginal.software/grpcd-go/client"
	"git.sonicoriginal.software/grpcd-go/diagnostics"
	"git.sonicoriginal.software/grpcd-go/service"
	grpcd "git.sonicoriginal.software/grpcd-protos"
)

const cleanupTimeout = 5 * time.Second

// registerExampleService stands in for the caller's own service registration.
func registerExampleService(s grpc.ServiceRegistrar) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "example.ExampleService",
		HandlerType: (*any)(nil),
		Methods:     []grpc.MethodDesc{{MethodName: "Create"}},
	}, struct{}{})
}

// Example shows how a service wires itself up. The listener and the server are
// the caller's, as are the goroutines; this package only assembles. It has no
// Output comment, so it is compiled but never run — its job is to keep this
// sequence type-checked.
func Example() {
	// Registered first so it runs last, after the teardown below has flushed.
	// Returning rather than calling os.Exit directly is what lets the defers run
	// at all.
	exitCode := 1
	defer func() { os.Exit(exitCode) }()

	// The process context. The shutdown builds its deadline on this one, which
	// is why the signal cancels a child of it rather than this.
	ctx := context.Background()

	serveCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The same name the otel resource and the grpcd registration are keyed by.
	serverName := foundation.Name("example")

	log, flush, err := foundationotel.Init(ctx, serverName, foundation.Version())
	if err != nil {
		slog.Default().Error("Failed to initialize telemetry", slog.Any("error", err))
		return
	}

	ctx = logger.ContextWithLogger(ctx, log)

	// New installs an interceptor that puts this logger into every request
	// context, so it has to be built after the logger is complete.
	srv := foundation.New(log)

	// Deferred before anything else can fail, so every path out of here stops
	// the server and exports what it logged on the way.
	defer foundation.HandleGracefulShutdown(ctx, log, srv, flush, cleanupTimeout)

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

	lis, err := foundation.Listen()
	if err != nil {
		log.Error("Failed to create listener", slog.Any("error", err))
		return
	}
	addr := lis.Addr()

	log = log.With(slog.String("address", addr.String()))

	// With no grpcd address there is nothing to register with, and the server
	// serves anyway.
	if grpcdAddress := os.Getenv(grpcdclient.GRPCDAddressKey); grpcdAddress != "" {
		conn, err := foundationclient.New(grpcdAddress, nil, nil)
		if err != nil {
			log.Error("Failed to connect to grpcd", slog.Any("error", err))
			return
		}
		defer conn.Close()

		sc := grpcd.NewGRPCDServiceClient(conn)
		grpcdClient := grpcdclient.New(log, serverName, addr, methodList, sc)

		// Register holds the registration stream open. Its ending is what removes the
		// rows, so there is no deregistration to wait for here.
		go grpcdClient.Register(serveCtx)
	}

	// Serve blocks, and a deferred teardown cannot run while it does, so it goes
	// to a goroutine and the select below decides when this returns.
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	log.Info("gRPC server listening")

	select {
	case err := <-serveErr:
		if err != nil {
			log.Error("Failed to serve", slog.Any("error", err))
		}
	case <-serveCtx.Done():
		exitCode = 0
	}
}
