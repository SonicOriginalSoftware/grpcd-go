package diagnostics

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// refuseConnection stands in for the socket layer, so a target check can fail
// to connect without anything being dialled.
func refuseConnection(_ context.Context, _ string) (net.Conn, error) {
	return nil, errors.New("connection refused")
}

func TestNewDependencyCheck(t *testing.T) {
	t.Run("reports a serving dependency", func(t *testing.T) {
		address := "cache:443"
		check := NewDependencyCheck(newServingDependency(address))

		got, err := check(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Address != address {
			t.Errorf("address = %q, want %q", got.Address, address)
		}
		if want := grpc_health_v1.HealthCheckResponse_SERVING.String(); got.Serving != want {
			t.Errorf("serving = %q, want %q", got.Serving, want)
		}
		if want := connectivity.Ready.String(); got.State != want {
			t.Errorf("state = %q, want %q", got.State, want)
		}
		if got.LastChecked == 0 {
			t.Error("last checked was not recorded")
		}
		if got.Details == nil {
			t.Error("details map is nil")
		}
	})

	t.Run("reports the dependency when the health check fails", func(t *testing.T) {
		address := "queue:443"
		wantErr := status.Error(codes.Unavailable, "health service unavailable")
		check := NewDependencyCheck(newFailingDependency(address, wantErr))

		got, err := check(t.Context())
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if got.Address != address {
			t.Errorf("address = %q, want %q", got.Address, address)
		}
		if want := grpc_health_v1.HealthCheckResponse_UNKNOWN.String(); got.Serving != want {
			t.Errorf("serving = %q, want %q", got.Serving, want)
		}
		if want := connectivity.TransientFailure.String(); got.State != want {
			t.Errorf("state = %q, want %q", got.State, want)
		}
	})
}

// addressStub answers Address with a fixed replica.
type addressStub string

func (a addressStub) Address() string { return string(a) }

func TestNewUpstreamCheck(t *testing.T) {
	t.Run("reports the replica rather than the target", func(t *testing.T) {
		check := NewUpstreamCheck(newServingDependency("grpcd:///pkg.Service"), addressStub("10.0.0.1:50054"))

		got, err := check(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Address != "10.0.0.1:50054" {
			t.Errorf("address = %q, want 10.0.0.1:50054", got.Address)
		}
		if want := grpc_health_v1.HealthCheckResponse_SERVING.String(); got.Serving != want {
			t.Errorf("serving = %q, want %q", got.Serving, want)
		}
	})

	t.Run("reports no replica when the health check fails", func(t *testing.T) {
		wantErr := status.Error(codes.Unavailable, "health service unavailable")
		check := NewUpstreamCheck(newFailingDependency("grpcd:///pkg.Service", wantErr), addressStub(""))

		got, err := check(t.Context())
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if got.Address != "" {
			t.Errorf("address = %q, want empty", got.Address)
		}
	})
}

func TestNewTargetCheck(t *testing.T) {
	t.Run("returns no dependency when the target cannot be parsed", func(t *testing.T) {
		// An unknown scheme is not enough: grpc falls back to its default one
		// and builds a client anyway. A control character fails URL parsing on
		// both attempts, which is what makes the connection itself fail.
		check := NewTargetCheck("cache:443\n")

		got, err := check(t.Context())
		if err == nil {
			t.Fatal("expected error")
		}
		if got != nil {
			t.Fatalf("dependency = %v, want nil", got)
		}
	})

	t.Run("reports an unknown status when the target does not answer", func(t *testing.T) {
		// The passthrough scheme hands the target straight to the dialer. The
		// default scheme would resolve it first, and a failed DNS lookup would
		// end the RPC before refuseConnection is ever reached.
		address := "passthrough:///queue:443"
		check := NewTargetCheck(address, grpc.WithContextDialer(refuseConnection))

		got, err := check(t.Context())
		if err == nil {
			t.Fatal("expected error")
		}
		if got.Address != address {
			t.Errorf("address = %q, want %q", got.Address, address)
		}
		if want := grpc_health_v1.HealthCheckResponse_UNKNOWN.String(); got.Serving != want {
			t.Errorf("serving = %q, want %q", got.Serving, want)
		}
		if want := connectivity.TransientFailure.String(); got.State != want {
			t.Errorf("state = %q, want %q", got.State, want)
		}
	})
}
