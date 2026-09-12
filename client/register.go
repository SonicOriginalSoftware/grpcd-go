package client

import (
	"context"
	"log/slog"
	"net"
	"strconv"

	"google.golang.org/grpc"

	grpcd "github.com/grpcd/protos"
)

// Register holds this server's registration open for as long as ctx lives.
//
// The registration is the stream: grpcd writes the rows when it opens and
// removes them when it ends, so there is nothing to refresh and nothing to
// deregister on the way out.
//
// A broken stream is reopened. The wait between attempts is the gRPC
// connection's own backoff, reached through WaitForReady, so this holds no
// timer of its own.
func (c *Client) Register(ctx context.Context) {
	// A registration is the set of methods it names. With none there is nothing
	// to register, and holding a stream open would claim otherwise.
	if len(c.methods) == 0 {
		c.log.InfoContext(ctx, "No methods to register, not registering")
		return
	}

	port, err := port(c.addr)
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to read listening port",
			slog.Any("error", err), slog.String("address", c.addr.String()))
		return
	}

	request := &grpcd.RegisterRequest{
		ServerName: c.serverName,
		Methods:    c.methods,
		Port:       port,
	}

	for ctx.Err() == nil {
		c.hold(ctx, request)
	}
}

// hold opens the registration stream and stays on it until it ends.
func (c *Client) hold(ctx context.Context, request *grpcd.RegisterRequest) {
	ctx, span := c.tracer.Start(ctx, "register")
	defer span.End()

	stream, err := c.service.Register(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to register", slog.Any("error", err))
		return
	}

	if _, err = stream.Recv(); err != nil {
		c.log.ErrorContext(ctx, "Registration was not accepted", slog.Any("error", err))
		return
	}

	c.log.InfoContext(ctx, "Registered",
		slog.Int("method_count", len(c.methods)), slog.Uint64("port", uint64(request.Port)))

	// The stream carries nothing after the acknowledgement. This blocks until it
	// ends, which is the point: grpcd removes the rows when it does.
	_, err = stream.Recv()

	c.log.InfoContext(ctx, "Registration ended", slog.Any("error", err))
}

// port pulls the port half out of a listener address. grpcd composes the whole
// address from this and the IP it reads off the connection.
func port(addr net.Addr) (uint32, error) {
	_, value, err := net.SplitHostPort(addr.String())
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(parsed), nil
}
