package client

import (
	"context"
	"log/slog"
	"os"

	"git.sonicoriginal.software/grpc-foundation/client"
	grpcd "git.sonicoriginal.software/grpcd-protos"
)

func (c *Client) deregister(ctx context.Context) {
	grpcdAddress := os.Getenv(GRPCDAddressKey)
	if grpcdAddress == "" {
		c.log.InfoContext(ctx, "grpcd address not set - not deregistering")
		return
	}

	ctx, span := c.tracer.Start(ctx, "deregister")
	defer span.End()

	conn, err := client.New(grpcdAddress, nil, nil)
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to create client",
			"error", err, slog.String("address", grpcdAddress))
		return
	}
	defer conn.Close()

	client := grpcd.NewGRPCDServiceClient(conn)
	_, err = client.Deregister(ctx, &grpcd.DeregisterRequest{})
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to deregister",
			"error", err, slog.String("address", grpcdAddress))
	}
}
