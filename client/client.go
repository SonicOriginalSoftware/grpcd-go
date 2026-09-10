//revive:disable:package-comments
package client

import (
	"log/slog"
	"net"

	"git.sonicoriginal.software/logger"

	grpcd "git.sonicoriginal.software/grpcd-protos"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Client holds this server's registration with grpcd.
type Client struct {
	tracer     trace.Tracer
	log        *slog.Logger
	service    grpcd.GRPCDServiceClient
	serverName string
	methods    []string
	addr       net.Addr
}

const (
	component = "grpcd-client"
	// GRPCDAddressKey is the env variable name
	// of what address to use for the grpcd connection
	GRPCDAddressKey = "GRPCD_ADDRESS"
)

// New returns a new grpcd client.
//
// The server name is what grpcd reports the registration under, and is the same
// name the server identifies itself by everywhere else.
//
// addr is the server's own listener address. grpcd takes the IP from the
// connection and cannot see the port the server is listening on, so the port
// half comes from here — read from the listener rather than from configuration,
// so a bind to :0 reports what it actually received.
//
// service is the grpcd client the caller dialed. Taking it rather than an
// address is what lets a test supply a fake.
func New(
	log *slog.Logger,
	serverName string,
	addr net.Addr,
	methods []string,
	service grpcd.GRPCDServiceClient,
) *Client {
	if log == nil {
		log = logger.NewNullLogger()
	}

	log = log.With(slog.String("component", component))

	return &Client{
		log:        log,
		tracer:     otel.Tracer(component),
		service:    service,
		serverName: serverName,
		methods:    methods,
		addr:       addr,
	}
}
