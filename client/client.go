//revive:disable:package-comments
package client

import (
	"log/slog"

	"git.sonicoriginal.software/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Client provides service grpcd functionality
type Client struct {
	tracer     trace.Tracer
	log        *slog.Logger
	serverName string
	methods    []string
}

const (
	component = "grpcd-client"
	// GRPCDAddressKey is the env variable name
	// of what address to use for the grpcd connection
	GRPCDAddressKey = "GRPCD_ADDRESS"
)

// New returns a new grpcd client. The server name is what grpcd reports the
// registration under, and is the same name the server identifies itself by
// everywhere else.
func New(log *slog.Logger, serverName string, methods []string) *Client {
	if log == nil {
		log = logger.NewNullLogger()
	}
	log = log.With(slog.String("component", component))
	return &Client{
		log:        log,
		tracer:     otel.Tracer(component),
		serverName: serverName,
		methods:    methods,
	}
}
