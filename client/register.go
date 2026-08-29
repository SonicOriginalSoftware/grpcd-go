package client

import (
	"context"
	"log/slog"
	"os"

	"git.sonicoriginal.software/grpc-foundation/client"
	grpcd "git.sonicoriginal.software/grpcd-protos"
)

func (c *Client) register(ctx context.Context) {
	grpcdAddress := os.Getenv(GRPCDAddressKey)
	if grpcdAddress == "" {
		c.log.InfoContext(ctx, "grpcd address not set - not registering")
		return
	}

	ctx, span := c.tracer.Start(ctx, "register")
	defer span.End()

	conn, err := client.New(grpcdAddress, nil, nil)
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to create client",
			"error", err, slog.String("grpcd_address", grpcdAddress))
		return
	}
	defer conn.Close()

	client := grpcd.NewGRPCDServiceClient(conn)
	_, err = client.Register(ctx, &grpcd.RegisterRequest{
		ServerName: c.serverName,
		Methods:    c.methods,
	})
	if err != nil {
		c.log.ErrorContext(ctx, "Failed to register",
			"error", err, slog.String("grpcd_address", grpcdAddress))
	}
}
