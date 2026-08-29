# grpcd-go

Go client library for the
[grpcd](https://github.com/sonic-original-software/grpcd-protos) gRPC method
discovery system.

## About

This library provides the Go implementation for interfacing with grpcd, a system
that maps gRPC method names to network addresses. Services register the methods
they implement; clients query grpcd to discover where those methods are
available.

For system architecture, protocol definitions, and design documentation, see
[grpcd-protos](https://github.com/sonic-original-software/grpcd-protos).

## Installation

```bash
go get git.sonicoriginal.software/grpcd-go
```

## What's Included

- **`client/`** - Client SDK for method registration and discovery
- **`diagnostics/`** - Diagnostics service reporting on upstream dependencies
- **`service/`** - Assembly helpers for registering a service's endpoints

## Usage

### Service Assembly

The `service` package attaches the endpoints every service exposes — health,
reflection, and diagnostics — alongside your own, and reports which methods
should be advertised to grpcd.

It assembles only. The listener, the server, the background goroutines, and the
blocking `Serve` call are yours, because those are the pieces that differ
between production and a test.

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"

    "git.sonicoriginal.software/logger"

    foundation "git.sonicoriginal.software/grpc-foundation/server"

    grpcdclient "git.sonicoriginal.software/grpcd-go/client"
    "git.sonicoriginal.software/grpcd-go/diagnostics"
    "git.sonicoriginal.software/grpcd-go/service"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    log := slog.Default()

    lis, err := foundation.Listen()
    if err != nil {
        log.Error("Failed to create listener", slog.Any("error", err))

        return
    }

    srv := foundation.New(log)

    log = log.With(slog.String("address", lis.Addr().String()))
    ctx = logger.ContextWithLogger(ctx, log)

    // Checks for the upstream services this one depends on. The grpcd check is
    // added for you, so "grpcd" is reserved.
    checks := diagnostics.Checks{}

    // You keep this handle. Register marks your services SERVING; flipping one
    // to NOT_SERVING later, or draining with Shutdown, is yours to do.
    healthSrv := health.NewServer()

    methodList, err := service.Register(srv, healthSrv, checks, func(s grpc.ServiceRegistrar) {
        yourpb.RegisterYourServiceServer(s, &yourServer{})
    })
    if err != nil {
        log.Error("Failed to register services", slog.Any("error", err))

        return
    }

    grpcdClient := grpcdclient.New(log, methodList)
    go grpcdClient.Run(ctx)

    go foundation.HandleGracefulShutdown(ctx, cancel, log, srv, 5*time.Second)

    if err := srv.Serve(lis); err != nil {
        log.Error("Failed to serve", slog.Any("error", err))
    }
}
```

The returned method list excludes the `grpc.`, `info.`, and `diagnostics.`
services, which are infrastructure rather than something callers discover.

## Health Status

`health.NewServer()` marks the `""` entry SERVING, which answers "is this
process alive". `Register` adds an entry per service you registered, under its
fully qualified gRPC name, so a probe asks about `yourpackage.YourService`
rather than a logical name of your choosing.

Those entries start SERVING and stay there until you change them. Only your
application knows whether a given upstream being down means it can still do its
job, so deciding that is yours:

```go
healthSrv.SetServingStatus(
    yourpb.YourService_ServiceDesc.ServiceName,
    grpc_health_v1.HealthCheckResponse_NOT_SERVING,
)
```

The generated `_ServiceDesc.ServiceName` constant is the same name `Register`
used, so the two cannot drift.

### Direct Client Usage

For more control, use the client directly:

```go
package main

import (
    "context"
    "log/slog"

    "git.sonicoriginal.software/grpcd-go/client"
)

func main() {
    log := slog.Default()

    // Create client with the methods your service implements
    methods := []string{
        "yourpackage.YourService.YourMethod",
        "yourpackage.YourService.AnotherMethod",
    }

    grpcdClient := client.New(log, methods)

    // Start background registration loop
    ctx := context.Background()
    go grpcdClient.Run(ctx)

    // Your service runs...

    // Graceful shutdown deregisters automatically via context cancellation
}
```

A registration is a lease that `Run` renews on an interval. Given an empty
method list there is no lease worth holding, so `Run` logs that and returns
immediately rather than registering nothing every interval.

## Configuration

Configure via environment variables:

### Required

- `GRPCD_ADDRESS` - Address of the grpcd service (e.g., `grpcd.example.com:443`)

If `GRPCD_ADDRESS` is not set, the service runs in disconnected mode (no
registration).

### Method Names

Methods must be fully qualified in the format:

```
package.service.Method
```

Examples:

- `auth.AuthService.Login`
- `api.v1.UserService.GetUser`

`service.Methods` derives these names from what is registered on your server, so
you do not maintain the list by hand.

## Reference Implementation

`service/example_test.go` holds the wiring above as a Go `Example`. It has no
`// Output:` comment, so `go test` compiles it and never runs it — its job is to
keep the documented sequence type-checked against the real API.

Read it with:

```bash
go doc git.sonicoriginal.software/grpcd-go/service
```
