package discover

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	grpcdmethods "git.sonicoriginal.software/grpcd-go/methods"
)

// scheme is what routes a target to this package's resolver.
const scheme = "grpcd"

// Upstream is one dependency reached through grpcd.
//
// It is the resolver builder for its own ClientConn: DialOptions carries it
// into grpc.NewClient, grpc-go calls Build when the connection first leaves
// idle, and the loop that starts there pushes each address it discovers into
// the connection. The caller holds a plain *grpc.ClientConn and never sees an
// address change.
type Upstream struct {
	discovery *Discovery
	method    string
	sensor    *sensor
	log       *slog.Logger
}

// Target is the string to build the ClientConn against. Its scheme selects
// this resolver; its endpoint is the service name, which grpc-go also uses as
// the default :authority, so it holds no slash.
func (u *Upstream) Target() string {
	return scheme + ":///" + grpcdmethods.ServiceName(u.method)
}

// DialOptions are what the ClientConn needs to be discovered: this resolver,
// the sensor that reports its transports opening and closing, and no idle
// timeout, since grpc-go would otherwise shut the resolver down after thirty
// quiet minutes and a held connection is meant to be held.
func (u *Upstream) DialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithResolvers(u),
		grpc.WithStatsHandler(u.sensor),
		grpc.WithIdleTimeout(0),
	}
}

// Address reports the replica the connection is on right now, or "" while
// none is held. Diagnostics report this; the ClientConn's own Target is the
// grpcd:/// URL.
func (u *Upstream) Address() string {
	return u.sensor.Address()
}

// Scheme names this resolver for grpc-go.
func (*Upstream) Scheme() string {
	return scheme
}

// Build starts the discovery loop for the ClientConn and answers with what
// stops it. grpc-go calls this when the connection leaves idle.
func (u *Upstream) Build(
	_ resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions,
) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(u.discovery.ctx)

	l := &loop{upstream: u, cc: cc, cancel: cancel}

	go l.run(ctx)

	return l, nil
}
