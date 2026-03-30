//revive:disable:package-comments
package client

import (
	"log/slog"
	"sync"

	"git.sonicoriginal.software/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Client provides service grpcd functionality
type Client struct {
	mu sync.RWMutex

	tracer  trace.Tracer
	log     *slog.Logger
	methods []string
}

const (
	component = "grpcd-client"
	// GRPCDAddressKey is the env variable name
	// of what address to use for the grpcd connection
	GRPCDAddressKey = "GRPCD_ADDRESS"
)

// New returns a new grpcd client
func New(log *slog.Logger, methods []string) *Client {
	if log == nil {
		log = logger.NewNullLogger()
	}
	log = log.With(slog.String("component", component))
	return &Client{log: log, tracer: otel.Tracer(component), methods: methods}
}
