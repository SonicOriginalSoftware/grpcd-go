# `grpcd` Client

Go client library for the [grpcd](https://github.com/grpcd) gRPC method
discovery system.

## About

This library provides the Go implementation for interfacing with grpcd, a system
that maps gRPC method names to network addresses. Services register the methods
they implement; clients query grpcd to discover where those methods are
available.

For system architecture, protocol definitions, and design documentation, see
[grpcd-protos](https://github.com/grpcd/protos).

## Installation

```bash
go get github.com/grpcd/client
```

## What's Included

- **`client/`** - Client SDK for method registration
- **`discover/`** - Resolver that keeps a connection pointed at a discovered
  upstream

The endpoints a service exposes and the method list it advertises come from
[grpc-service](https://github.com/sonic-original-software/grpc-service); this
library takes that list and registers it.

## Usage

`client/example_test.go` holds the whole sequence as a Go `Example`: assembling
the server, dialing grpcd only when `GRPCD_ADDRESS` is set, reaching an
upstream, reporting both as dependencies, and holding the registration. It has
no `// Output:` comment, so `go test` compiles it and never runs it, which keeps
it type-checked against the real API. Read it there rather than from a copy
here.

### Registering

`client.New` takes a `grpcd.GRPCDServiceClient` rather than an address, so the
connection is yours to build and a test can supply a fake. It also takes your
listener's address: grpcd reads the IP off the connection and cannot see the
port you are serving on, so the port half comes from there.

`Register` opens the registration stream and holds it. The stream is the
registration — grpcd writes the rows when it opens and removes them when it
ends — so there is no interval to refresh and nothing to deregister on the way
out. A broken stream is reopened, paced by the gRPC connection's own backoff.

Given an empty method list there is nothing to register, so `Register` logs
that and returns rather than holding a stream that claims otherwise.

### Reaching an Upstream

A service that depends on another grpcd-registered service holds one
`*grpc.ClientConn` to it for the life of the process. The `discover` package
supplies that connection's resolver: it asks grpcd for the method, probes each
candidate from the service's own network position, reports the ones it cannot
reach, and pushes the one it can into the connection. When the transport to that
replica drops, it discovers again. The application holds a plain connection and
the generated client built on it never sees an address change.

`discover.New` is built once per process on the same grpcd client the
registration uses. Each upstream is one `Upstream`, named by one of its methods
(a replica registers every method of its service, so one stands for the whole).
Its `Target()` and `DialOptions()` go to `foundationclient.New` like any other
target and options. grpc-go builds the resolver when the connection first leaves
idle, so call `Connect()` on it to start discovering at startup rather than on
the first RPC.

The resolver also holds a `Watch` naming the address it took. When a replica of
the upstream registers later, grpcd tells a share of the holders to move to it;
the resolver probes the new address, pushes it into the connection, and opens a
`Watch` naming it. A new replica takes its share of existing connections that
way, and a move that cannot be reached is a no-op.

While no replica is held, RPCs on the connection fail with `Unavailable`
rather than waiting.

### Reporting Dependencies

Both connections are dependencies the service's diagnostics should report. The
grpcd connection goes in under `client.CheckName`, so every service reports it
under the same name; the upstream goes in through
`diagnostics.NewUpstreamCheck`, which reports the replica address the
connection is on rather than its `grpcd:///` target.

## Configuration

- `GRPCD_ADDRESS` - Address of the grpcd service (e.g., `grpcd.example.com:443`).
  When unset, the service neither registers nor discovers, and serves anyway.

### Method Names

Methods are gRPC wire format:

```
/package.Service/Method
```

Examples:

- `/auth.AuthService/Login`
- `/api.v1.UserService/GetUser`

`service.Register` in grpc-service derives these names from what is registered
on your server, so you do not maintain the list by hand.
