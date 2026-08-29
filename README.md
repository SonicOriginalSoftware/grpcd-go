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
reflection, diagnostics, and info — alongside your own, and reports which
methods should be advertised to grpcd.

The info service answers with the server's version, read from
`GRPC_SERVER_VERSION`, so services no longer implement it themselves.

It assembles only. The listener, the server, the background goroutines, and the
blocking `Serve` call are yours, because those are the pieces that differ
between production and a test.

`service/example_test.go` holds the whole sequence as a Go `Example`. It has no
`// Output:` comment, so `go test` compiles it and never runs it, which keeps it
type-checked against the real API. Read it there rather than from a copy here.

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

`service.Register` derives these names from what is registered on your server,
so you do not maintain the list by hand.
