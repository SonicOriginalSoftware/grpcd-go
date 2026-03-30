//revive:disable:package-comments
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"git.sonicoriginal.software/grpcd-go/example/internal/service"
	"git.sonicoriginal.software/grpcd-go/example/internal/station"
	service_lib "git.sonicoriginal.software/grpcd-go/service"

	foundationotel "git.sonicoriginal.software/grpc-foundation/otel"
	"git.sonicoriginal.software/grpc-protos/diagnostics"
	"git.sonicoriginal.software/grpc-protos/info"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
)

const serviceName = "example"

func main() {
	ctx := context.Background()

	log, shutdown, err := foundationotel.Init(ctx, serviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize otel: %v\n", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	meter := otel.Meter(serviceName)
	exampleServer := service.NewServer(log, meter)

	err = service_lib.Run(
		context.Background(), serviceName, log,
		func(s grpc.ServiceRegistrar) {
			info.RegisterInfoServiceServer(s, exampleServer)
			diagnostics.RegisterDiagnosticsServiceServer(s, exampleServer)

			station.RegisterExampleServiceServer(s, exampleServer)
		},
		nil,
	)
	if err != nil {
		log.Error("Server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
