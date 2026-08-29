package client

import (
	"context"
	"log/slog"
	"os"
	"time"

	"git.sonicoriginal.software/grpc-foundation/client"
	grpcd "git.sonicoriginal.software/grpcd-protos"
)

// deregisterTimeout bounds the call so an unreachable grpcd cannot hold up
// shutdown
const deregisterTimeout = 5 * time.Second

func (c *Client) deregister(ctx context.Context) {
	grpcdAddress := os.Getenv(GRPCDAddressKey)
	if grpcdAddress == "" {
		c.log.InfoContext(ctx, "grpcd address not set - not deregistering")
		return
	}

	// Run returns because its context was cancelled, and this runs as its
	// deferred call, so the RPC would fail before leaving the process.
	// WithoutCancel keeps the logger and the trace span while dropping the
	// cancellation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deregisterTimeout)
	defer cancel()

	ctx, span := c.tracer.Start(ctx, "deregister")
	defer span.End()

	conn, err := client.New(grpcdAddress, nil, nil)
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to create client",
			"error", err, slog.String("grpcd_address", grpcdAddress))
		return
	}
	defer conn.Close()

	client := grpcd.NewGRPCDServiceClient(conn)
	_, err = client.Deregister(ctx, &grpcd.DeregisterRequest{})
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to deregister",
			"error", err, slog.String("grpcd_address", grpcdAddress))
	}
}
