package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"testing"

	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"
)

type checkStub struct {
	result    *diagpb.ServiceDependency
	err       error
	callCount int
}

func (s *checkStub) run(_ context.Context) (*diagpb.ServiceDependency, error) {
	s.callCount++

	return s.result, s.err
}

// newStubbedServer registers one stub per name so a test can hold on to the
// stubs and assert against them afterwards.
func newStubbedServer(stubs map[string]*checkStub) *Server {
	checks := Checks{}
	for name, stub := range stubs {
		checks[name] = stub.run
	}

	return NewServer(checks)
}

// assertStubResults reports that every stub ran exactly once and that its
// result came back under its own name.
func assertStubResults(
	t *testing.T,
	stubs map[string]*checkStub,
	services map[string]*diagpb.ServiceDependency,
) {
	t.Helper()

	if len(services) != len(stubs) {
		t.Fatalf("service count = %d, want %d", len(services), len(stubs))
	}
	for name, stub := range stubs {
		if stub.callCount != 1 {
			t.Errorf("%s check call count = %d, want 1", name, stub.callCount)
		}
		if got := services[name]; got != stub.result {
			t.Errorf("%s result = %v, want %v", name, got, stub.result)
		}
	}
}

func TestGetDiagnostics(t *testing.T) {
	t.Run("returns empty services", func(t *testing.T) {
		server := NewServer(nil)

		response, err := server.GetDiagnostics(t.Context(), &diagpb.GetDiagnosticsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Services == nil {
			t.Fatal("services map is nil")
		}
		if len(response.Services) != 0 {
			t.Fatalf("service count = %d, want 0", len(response.Services))
		}
	})

	t.Run("returns check results by name", func(t *testing.T) {
		want := &diagpb.ServiceDependency{Address: "cached-service:443"}
		stub := &checkStub{result: want}
		checks := Checks{"cached-service": stub.run}
		server := NewServer(checks)

		response, err := server.GetDiagnostics(t.Context(), &diagpb.GetDiagnosticsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stub.callCount != 1 {
			t.Fatalf("check call count = %d, want 1", stub.callCount)
		}
		if got := response.Services["cached-service"]; got != want {
			t.Fatalf("service result = %v, want %v", got, want)
		}
	})

	t.Run("retains result when check fails", func(t *testing.T) {
		serviceName := "unavailable-service"
		want := &diagpb.ServiceDependency{
			Address: fmt.Sprintf("%s:443", serviceName),
			Serving: "UNKNOWN",
		}
		stub := &checkStub{
			result: want,
			err:    errors.New("health check failed"),
		}
		checks := Checks{serviceName: stub.run}
		server := NewServer(checks)

		response, err := server.GetDiagnostics(t.Context(), &diagpb.GetDiagnosticsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := response.Services[serviceName]; got != want {
			t.Fatalf("service result = %v, want %v", got, want)
		}
	})

	t.Run("returns a result for every check", func(t *testing.T) {
		stubs := map[string]*checkStub{
			"cache":    {result: &diagpb.ServiceDependency{Address: "cache:443"}},
			"database": {result: &diagpb.ServiceDependency{Address: "database:443"}},
			"queue":    {result: &diagpb.ServiceDependency{Address: "queue:443"}},
		}
		server := newStubbedServer(stubs)

		response, err := server.GetDiagnostics(t.Context(), &diagpb.GetDiagnosticsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertStubResults(t, stubs, response.Services)
	})

	t.Run("retains results of the other checks when one fails", func(t *testing.T) {
		stubs := map[string]*checkStub{
			"cache":    {result: &diagpb.ServiceDependency{Address: "cache:443"}},
			"database": {result: &diagpb.ServiceDependency{Address: "database:443"}},
			"queue": {
				result: &diagpb.ServiceDependency{Address: "queue:443", Serving: "UNKNOWN"},
				err:    errors.New("health check failed"),
			},
		}
		server := newStubbedServer(stubs)

		response, err := server.GetDiagnostics(t.Context(), &diagpb.GetDiagnosticsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertStubResults(t, stubs, response.Services)
	})
}
