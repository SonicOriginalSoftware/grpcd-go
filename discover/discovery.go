//revive:disable:package-comments
package discover

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"git.sonicoriginal.software/logger"

	grpcd "github.com/grpcd/protos"
)

const component = "grpcd-discover"

// Discovery is what every upstream in a process shares: the grpcd client the
// lookups go through, the probe that decides whether a candidate is reachable,
// and the context the lookups run under.
type Discovery struct {
	ctx     context.Context
	log     *slog.Logger
	tracer  trace.Tracer
	service grpcd.GRPCDServiceClient
	probe   Probe
}

// New returns a Discovery.
//
// ctx is the process context. grpc-go builds a resolver with no context of its
// own, so the loops run under a child of this one and stop with it. Each loop
// also stops when its ClientConn is closed.
//
// service is the grpcd client the caller dialed, the same one its registration
// uses. Taking it rather than an address is what lets a test supply a fake.
//
// probe is what a candidate has to pass before it is used; nil means the one
// NewProbe builds.
func New(
	ctx context.Context,
	log *slog.Logger,
	service grpcd.GRPCDServiceClient,
	probe Probe,
) *Discovery {
	if log == nil {
		log = logger.NewNullLogger()
	}

	if probe == nil {
		probe = NewProbe()
	}

	return &Discovery{
		ctx:     ctx,
		log:     log.With(slog.String("component", component)),
		tracer:  otel.Tracer(component),
		service: service,
		probe:   probe,
	}
}

// Upstream describes one dependency, named by one of its methods. A replica
// registers every method of its service, so the connection built from this
// serves the whole service.
func (d *Discovery) Upstream(method string) *Upstream {
	return &Upstream{
		discovery: d,
		method:    method,
		sensor:    newSensor(),
		log:       d.log.With(slog.String("method", method)),
	}
}
