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
- **`service/`** - Helper for running gRPC services with automatic grpcd
  registration
- **`example/`** - Reference implementation demonstrating usage

## Usage

### Service Helper (Recommended)

The `service` package provides a convenient wrapper that automatically handles:

- gRPC server setup with health checks and reflection
- Background grpcd registration of your service's methods
- Graceful shutdown with deregistration
- OpenTelemetry tracing and metrics

```go
package main

import (
    "context"
    "log/slog"

    "git.sonicoriginal.software/grpcd-go/service"
    "google.golang.org/grpc"
)

func main() {
    log := slog.Default()

    err := service.Run(
        context.Background(),
        "my-service",
        log,
        func(s grpc.ServiceRegistrar) {
            // Register your gRPC service implementations
            yourpb.RegisterYourServiceServer(s, &yourServer{})
        },
    )
    if err != nil {
        log.Error("Service failed", slog.Any("error", err))
    }
}
```

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

The client automatically extracts method names from your registered gRPC
services when using the `service` helper.

## Running the Example

The `example/` directory contains a working example service that demonstrates
the out-of-the-box functionality provided by grpcd-go. This example includes:

- A protobuf service definition (`example/lib/creation_station.proto`)
- Generated Go code for the service
- An implementation that uses the `service` helper package

### Generate Code from Proto

Use the Taskfile to generate the Go code from the example proto service:

```bash
task gen-example
```

This generates the protobuf and gRPC code that the example service
implementation uses.

### Run the Example

```bash
go run example/main.go
```

The example service will start and demonstrate:

- Automatic gRPC server setup with health checks and reflection
- Service registration with grpcd (if configured)
- Multiple service implementations (Info, Diagnostics, and Example services)

### Testing with grpcd

If you have a running `grpcd` instance, you can see the example service register
and sync with it:

```bash
GRPCD_ADDRESS=localhost:50051 go run example/main.go
```

The example service will:

1. Connect to grpcd at the specified address
2. Register all its implemented methods
3. Send periodic heartbeats to maintain registration
4. Automatically deregister on shutdown

Without `GRPCD_ADDRESS` set, the service runs in disconnected mode (no
registration attempts).
