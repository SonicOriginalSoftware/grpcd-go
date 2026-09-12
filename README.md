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

- **`client/`** - Client SDK for method registration
- **`discover/`** - Resolver that keeps a connection pointed at a discovered upstream
- **`diagnostics/`** - Diagnostics service reporting on upstream dependencies
- **`methods/`** - Method-name helpers shared by the packages above
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

`client.New` takes a `grpcd.GRPCDServiceClient` rather than an address, so the
connection is yours to build and a test can supply a fake. It also takes your
listener's address: grpcd reads the IP off the connection and cannot see the
port you are serving on, so the port half comes from there.

`Run` opens the registration stream and holds it. The stream is the
registration — grpcd writes the rows when it opens and removes them when it
ends — so there is no interval to refresh and nothing to deregister on the way
out. A broken stream is reopened, paced by the gRPC connection's own backoff.

Given an empty method list there is nothing to register, so `Run` logs that and
returns rather than holding a stream that claims otherwise.

`service/example_test.go` holds the whole sequence, including building the
connection only when `GRPCD_ADDRESS` is set. Read it there rather than from a
copy here.

### Reaching an Upstream

A service that depends on another grpcd-registered service holds one
`*grpc.ClientConn` to it for the life of the process. The `discover` package
supplies that connection's resolver: it asks grpcd for the method, probes each
candidate from the service's own network position, reports the ones it cannot
reach, and pushes the one it can into the connection. When the transport to
that replica drops, it discovers again. The application holds a plain
connection and the generated client built on it never sees an address change.

`discover.New` is built once per process on the same grpcd client the
registration uses. Each upstream is one `Upstream`, named by one of its
methods (a replica registers every method of its service, so one stands for
the whole). Its `Target()` and `DialOptions()` go to `foundationclient.New`
like any other target and options. grpc-go builds the resolver when the
connection first leaves idle, so call `Connect()` on it to start discovering
at startup rather than on the first RPC.

`diagnostics.NewUpstreamCheck` reports such a connection under the replica
address it is currently on; the connection's own `Target()` is the `grpcd:///`
URL. While no replica is held, RPCs on the connection fail with `Unavailable`
rather than waiting.

`service/example_test.go` holds the wiring.

## Configuration

Configure via environment variables:

### Required

- `GRPCD_ADDRESS` - Address of the grpcd service (e.g., `grpcd.example.com:443`)

If `GRPCD_ADDRESS` is not set, the service runs in disconnected mode (no
registration).

### Method Names

Methods are gRPC wire format:

```
/package.Service/Method
```

Examples:

- `/auth.AuthService/Login`
- `/api.v1.UserService/GetUser`

`service.Register` derives these names from what is registered on your server,
so you do not maintain the list by hand.
