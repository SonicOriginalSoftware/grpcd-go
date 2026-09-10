package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"

	"git.sonicoriginal.software/grpc-testing/mocks/addr"
	grpcd "git.sonicoriginal.software/grpcd-protos"
)

// recvResult is one answer the fake stream hands back, in order.
type recvResult struct {
	response *grpcd.RegisterResponse
	err      error
}

// registerStream stands in for the stream grpcd holds open. The embedded
// interface supplies the ClientStream methods, none of which the client calls.
type registerStream struct {
	grpc.ClientStream

	results []recvResult
	calls   int
}

func (s *registerStream) Recv() (*grpcd.RegisterResponse, error) {
	if s.calls >= len(s.results) {
		return nil, io.EOF
	}

	result := s.results[s.calls]
	s.calls++

	return result.response, result.err
}

// serviceStub records the requests the client sent and answers with a stream
// the test prepared. The embedded interface supplies Discover, which the
// registration path never calls.
type serviceStub struct {
	grpcd.GRPCDServiceClient

	results  []recvResult
	err      error
	requests []*grpcd.RegisterRequest

	// stop ends Run's loop once the test has seen what it needs.
	stop context.CancelFunc
}

func (s *serviceStub) Register(
	_ context.Context, in *grpcd.RegisterRequest, _ ...grpc.CallOption,
) (grpc.ServerStreamingClient[grpcd.RegisterResponse], error) {
	s.requests = append(s.requests, in)

	if s.stop != nil {
		s.stop()
	}

	if s.err != nil {
		return nil, s.err
	}

	return &registerStream{results: s.results}, nil
}

func TestNew(t *testing.T) {
	t.Run("substitutes a logger when given none", func(t *testing.T) {
		client := New(nil, "example", addr.New("10.0.0.1:50051"), []string{"/pkg.S/M"}, nil)

		if client.log == nil {
			t.Fatal("expected a logger")
		}
	})
}

func TestRun(t *testing.T) {
	acknowledged := []recvResult{{response: &grpcd.RegisterResponse{}}, {err: io.EOF}}

	t.Run("holds the stream and sends the listening port", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{results: acknowledged, stop: cancel}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1:50051"), []string{"/pkg.S/M"}, service,
		).Register(ctx)

		if len(service.requests) != 1 {
			t.Fatalf("expected one registration, got %d", len(service.requests))
		}

		request := service.requests[0]

		if request.Port != 50051 {
			t.Errorf("expected port 50051, got %d", request.Port)
		}

		if request.ServerName != "example" {
			t.Errorf("expected server name %q, got %q", "example", request.ServerName)
		}
	})

	t.Run("reopens the stream after it ends", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{results: acknowledged}

		// Cancelling on the second call lets the first stream end and be
		// reopened, which is what the loop exists for.
		service.stop = func() {
			if len(service.requests) >= 2 {
				cancel()
			}
		}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1:50051"), []string{"/pkg.S/M"}, service,
		).Register(ctx)

		if len(service.requests) < 2 {
			t.Fatalf("expected the stream to be reopened, got %d attempts", len(service.requests))
		}
	})

	t.Run("registers nothing without methods", func(t *testing.T) {
		service := &serviceStub{}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1:50051"), nil, service,
		).Register(t.Context())

		if len(service.requests) != 0 {
			t.Fatalf("expected no registration, got %d", len(service.requests))
		}
	})

	t.Run("registers nothing when the address has no port", func(t *testing.T) {
		service := &serviceStub{}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1"), []string{"/pkg.S/M"}, service,
		).Register(t.Context())

		if len(service.requests) != 0 {
			t.Fatalf("expected no registration, got %d", len(service.requests))
		}
	})

	t.Run("registers nothing when the port is not a number", func(t *testing.T) {
		service := &serviceStub{}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1:http"), []string{"/pkg.S/M"}, service,
		).Register(t.Context())

		if len(service.requests) != 0 {
			t.Fatalf("expected no registration, got %d", len(service.requests))
		}
	})

	t.Run("gives up the attempt when the stream cannot be opened", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{err: errors.New("unavailable"), stop: cancel}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1:50051"), []string{"/pkg.S/M"}, service,
		).Register(ctx)

		if len(service.requests) != 1 {
			t.Fatalf("expected one attempt, got %d", len(service.requests))
		}
	})

	t.Run("gives up the attempt when the registration is refused", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{
			results: []recvResult{{err: errors.New("invalid method name")}},
			stop:    cancel,
		}

		New(
			slog.New(slog.DiscardHandler), "example",
			addr.New("10.0.0.1:50051"), []string{"/pkg.S/M"}, service,
		).Register(ctx)

		if len(service.requests) != 1 {
			t.Fatalf("expected one attempt, got %d", len(service.requests))
		}
	})
}
