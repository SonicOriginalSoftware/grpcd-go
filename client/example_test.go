package client_test

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
	"git.sonicoriginal.software/grpc-service/diagnostics"
	"git.sonicoriginal.software/grpc-service/service"

	grpcdclient "github.com/grpcd/client/client"
	"github.com/grpcd/client/discover"
	grpcd "github.com/grpcd/protos"
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

// Example shows how a service registers with grpcd and reaches an upstream
// through it. The listener and the server are the caller's, as are the
// goroutines. It has no Output comment, so it is compiled but never run — its
// job is to keep this sequence type-checked.
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

	// Checks for the services this one depends on, keyed by the name
	// diagnostics reports them under.
	checks := diagnostics.Checks{}

	// With no grpcd address there is nothing to register with and nothing to
	// discover through, and the server serves anyway.
	var grpcdClient grpcd.GRPCDServiceClient

	if grpcdAddress := os.Getenv(grpcdclient.GRPCDAddressKey); grpcdAddress != "" {
		conn, err := foundationclient.New(grpcdAddress, nil, nil)
		if err != nil {
			log.Error("Failed to connect to grpcd", slog.Any("error", err))
			return
		}
		defer conn.Close()

		grpcdClient = grpcd.NewGRPCDServiceClient(conn)

		// The registration and every discovery go through this connection, so
		// it is the one diagnostics report on.
		checks[grpcdclient.CheckName] = diagnostics.NewDependencyCheck(conn)

		// One Discovery per process, shared by every upstream. The example
		// service has none; the method below stands in for a generated
		// _FullMethodName constant of a real one.
		discovery := discover.New(serveCtx, log, grpcdClient, discover.NewProbe())

		upstream := discovery.Upstream("/example.UpstreamService/Get")

		// A plain connection. The resolver carried in by DialOptions pushes
		// each address it discovers into it, so the generated client built on
		// it never sees an address change.
		upstreamConn, err := foundationclient.New(upstream.Target(), nil, nil, upstream.DialOptions()...)
		if err != nil {
			log.Error("Failed to build the upstream connection", slog.Any("error", err))
			return
		}
		defer upstreamConn.Close()

		// grpc-go builds the resolver when the connection first leaves idle.
		// Discovering from startup rather than from the first RPC is worth an
		// explicit nudge.
		upstreamConn.Connect()

		checks["upstream"] = diagnostics.NewUpstreamCheck(upstreamConn, upstream)
	}

	healthSrv := health.NewServer()

	// The returned method list is what this server exposes beyond the
	// infrastructure endpoints, which is what it advertises to grpcd.
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

	if grpcdClient != nil {
		registration := grpcdclient.New(log, serverName, addr, methodList, grpcdClient)

		// Register holds the registration stream open. Its ending is what
		// removes the rows, so there is no deregistration to wait for here.
		go registration.Register(serveCtx)
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
